package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
)

// maxReceiptFailures caps the failing tests listed in a receipt; the full output stays in the shard log.
const maxReceiptFailures = 50

// shardRuntime is what a sharded `canopy test` resolved before running: the plan, the digest inputs,
// and everything the receipt and the shard UI need.
type shardRuntime struct {
	Index int
	Total int
	// From is where the shard came from: "flag", the CI variable pair for --shard auto
	// (e.g. CI_NODE_INDEX/CI_NODE_TOTAL), or empty when auto found no parallelism and fell back to 1/1.
	From string
	// Auto is set when the shard came from --shard auto.
	Auto bool
	// Dir is the shard dir (receipts and coverprofiles go under its out/ dir).
	Dir string

	Weights     shard.WeightResult
	Counts      map[string]int64 // test counts per package, for the plan header
	Plan        shard.Plan
	Inputs      shard.Inputs
	Digest      string
	Metrics     shard.ReceiptMetrics
	MetricsPath string
	// Gates are the result gates resolved for this run and deferred to the join (not enforced per shard).
	Gates shard.Gates
}

// Planned is this shard's packages, sorted.
func (s shardRuntime) Planned() []string {
	return s.Plan.Shards[s.Index-1]
}

// shardInputs is everything a shard plan depends on except the shard count. `canopy shard plan` builds
// it the same way, so both compute the same digest for the same config and total.
type shardInputs struct {
	Inputs      shard.Inputs // every group but [plan]
	Weights     shard.WeightResult
	Counts      map[string]int64
	Metrics     shard.ReceiptMetrics
	MetricsPath string
}

// resolveShardInputs builds the digest inputs and weights from the resolved test config.
// cfg.Runtime.Packages must hold the full (unsharded) package set and cfg.Runtime.Specifiers the
// specifiers as given (before --affected narrowing), as selectTestPackages leaves them.
func resolveShardInputs(cfg testConfig) (*shardInputs, error) {
	selection, err := shardSelectionLines(cfg)
	if err != nil {
		return nil, err
	}
	goLines, goEnv, err := shardGoLines(splitCSV(cfg.ReproEnv))
	if err != nil {
		return nil, err
	}
	run := shardRunLines(cfg)

	counts, err := shard.TestCounts(cfg.Runtime.Packages)
	if err != nil {
		return nil, fmt.Errorf("unable to count tests: %w", err)
	}
	metricsPath := filepath.Join(cfg.Shard.Dir, "metrics.json")
	m, metricsDigest, loadErr := shard.LoadMetrics(metricsPath)
	// the profile is how tests run, not which packages run, so a subset run still matches main's metrics
	profile := shard.Group{Name: shard.GroupRun, Lines: run}.Digest()
	w := shard.Weights(counts, m, loadErr, goEnv, profile)

	return &shardInputs{
		Inputs: shard.Inputs{
			Selection: selection,
			Run:       run,
			Gates:     shardGateLines(cfg),
			Go:        goLines,
			Source:    shardSourceLines(),
			Weights:   shard.WeightLines(w.Source, w.Units),
		},
		Weights:     w,
		Counts:      counts,
		Metrics:     shard.ReceiptMetrics{File: metricsDigest, Env: goEnv, Profile: profile, Ignored: w.Ignored},
		MetricsPath: metricsPath,
	}, nil
}

// forTotal plans total shards and completes the inputs with the [plan] group.
func (s shardInputs) forTotal(canopyVersion string, total int) (shard.Plan, shard.Inputs) {
	in := s.Inputs
	in.Plan = shard.PlanLines(canopyVersion, total)
	return shard.NewPlan(s.Weights.Units, total), in
}

// shardTestPackages resolves --shard, plans the split over cfg.Runtime.Packages and narrows it to this
// shard's packages, keeping the plan in cfg.Runtime.shard.
func shardTestPackages(cfg *testConfig, canopyVersion string) error {
	index, total, from, err := cfg.ShardIndex.Resolve(&env.OSEnvironmentGetter{})
	if err != nil {
		return err
	}
	auto := cfg.ShardIndex.Value == options.ShardAuto
	switch {
	case auto && from == "":
		log.Info("--shard auto: no CI parallelism detected, running all packages as shard 1/1")
	case auto:
		log.Infof("shard %d/%d (from %s)", index, total, from)
	}

	if cfg.OpenSessionOnFailure {
		log.Warn("--open-on-failure is ignored with --shard (results are reviewed in 'canopy shard join')")
		cfg.OpenSessionOnFailure = false
	}

	in, err := resolveShardInputs(*cfg)
	if err != nil {
		return err
	}
	plan, inputs := in.forTotal(canopyVersion, total)

	sh := &shardRuntime{
		Index:       index,
		Total:       total,
		From:        from,
		Auto:        auto,
		Dir:         cfg.Shard.Dir,
		Weights:     in.Weights,
		Counts:      in.Counts,
		Plan:        plan,
		Inputs:      inputs,
		Digest:      inputs.Digest(),
		Metrics:     in.Metrics,
		MetricsPath: in.MetricsPath,
	}
	if cfg.CoverMin > 0 {
		v := cfg.CoverMin
		sh.Gates.CoverMin = &v
		log.Infof("coverage threshold %g%% deferred to 'canopy shard join'", v)
	}

	all := map[string]golist.Package{}
	for _, p := range cfg.Runtime.Packages.Packages() {
		all[p.ImportPath] = p
	}
	narrowed := golist.NewPackageCollection()
	for _, p := range sh.Planned() {
		narrowed.Add(all[p])
	}

	log.Info(shardHeaderTitle(sh))
	log.WithFields("packages", sh.Planned()).Debug("shard packages")

	cfg.Runtime.Packages = narrowed
	cfg.Runtime.shard = sh
	return nil
}

func weightsSummary(w shard.WeightResult) string {
	if w.Source == shard.SourceMetrics {
		return fmt.Sprintf("metrics (%d measured, %d estimated)", w.Measured, w.Estimated)
	}
	return fmt.Sprintf("test counts (%s)", w.Ignored)
}

// shortDigest trims "sha256:<hex>" to "sha256:<8 hex chars>" for display.
func shortDigest(d string) string {
	if len(d) > len("sha256:")+8 {
		return d[:len("sha256:")+8]
	}
	return d
}

func shardSelectionLines(cfg testConfig) ([]string, error) {
	var lines []string
	for _, s := range cfg.Runtime.Specifiers {
		lines = append(lines, "specifier "+s)
	}
	for _, e := range cfg.ExcludePatterns {
		lines = append(lines, "exclude "+e)
	}
	lines = append(lines, "affected "+strconv.FormatBool(cfg.Affected))

	// the ref is resolved to a commit so shards that fetched it at different moments don't match
	since := "-"
	if cfg.Affected && cfg.AffectedSince != "" {
		out, err := exec.Command("git", "rev-parse", "--verify", "--quiet", cfg.AffectedSince+"^{commit}").Output() //nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("unable to resolve --affected-since %q to a commit: %w", cfg.AffectedSince, err)
		}
		since = strings.TrimSpace(string(out))
	}
	lines = append(lines, "affected-since "+since)

	pkgs := cfg.Runtime.Packages.ImportPaths()
	slices.Sort(pkgs)
	for _, p := range pkgs {
		lines = append(lines, "package "+p)
	}
	return lines, nil
}

// shardRunLines is the [run] group: how the tests are built and run. The rendered flags are sorted
// since they are rendered from maps (their order varies between processes).
func shardRunLines(cfg testConfig) []string {
	var lines []string
	for _, f := range sortedCopy(cfg.GoBuild.RenderedFlags) {
		lines = append(lines, "build-flag "+f)
	}
	for _, f := range sortedCopy(cfg.GoTest.RenderedFlags) {
		lines = append(lines, "test-flag "+f)
	}
	for _, f := range cfg.ExtraFlags {
		lines = append(lines, "extra-flag "+f)
	}
	return append(lines,
		"cover "+strconv.FormatBool(cfg.Cover),
		"no-cache "+strconv.FormatBool(cfg.NoCache),
		// on/off only: the seed is generated per run
		"shuffle "+strconv.FormatBool(cfg.Shuffle),
	)
}

func sortedCopy(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func shardGateLines(cfg testConfig) []string {
	if cfg.CoverMin > 0 {
		return []string{fmt.Sprintf("covermin %g", cfg.CoverMin)}
	}
	return nil
}

// shardGoLines is the [go] group, from the toolchain `go test` will use (not the one canopy was built with).
func shardGoLines(reproEnv []string) ([]string, shard.Env, error) {
	out, err := exec.Command("go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT").Output()
	if err != nil {
		return nil, shard.Env{}, fmt.Errorf("unable to run go env: %w", err)
	}
	var ge map[string]string
	if err := json.Unmarshal(out, &ge); err != nil {
		return nil, shard.Env{}, fmt.Errorf("unable to parse go env: %w", err)
	}
	lines := []string{
		"goversion " + ge["GOVERSION"],
		"goos " + ge["GOOS"],
		"goarch " + ge["GOARCH"],
		"env CGO_ENABLED=" + ge["CGO_ENABLED"],
		"env GOFLAGS=" + ge["GOFLAGS"],
		"env GOEXPERIMENT=" + ge["GOEXPERIMENT"],
	}
	for _, k := range reproEnv {
		lines = append(lines, fmt.Sprintf("env %s=%s", k, os.Getenv(k)))
	}
	return lines, shard.Env{GOOS: ge["GOOS"], GOARCH: ge["GOARCH"]}, nil
}

func shardSourceLines() []string {
	ss := source.CaptureState(".")
	if ss == nil {
		return []string{"commit -", "dirty false"}
	}
	return []string{"commit " + ss.Commit, "dirty " + strconv.FormatBool(ss.Dirty)}
}

// writeShardReceipt writes this shard's receipt (and its coverprofile copy, if any). run is nil when
// nothing ran (an empty shard, or --affected resolved to nothing).
func writeShardReceipt(sh *shardRuntime, canopyVersion string, run *gotest.Run, passed bool) error {
	r := shardReceipt(sh, canopyVersion, run, passed)

	if run != nil && run.ProfilePath() != "" {
		name := fmt.Sprintf("shard-%d.coverprofile", sh.Index)
		err := copyFile(run.ProfilePath(), filepath.Join(shard.OutDir(sh.Dir), name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// go test wrote no profile (e.g. every package failed to build)
		case err != nil:
			return fmt.Errorf("unable to copy coverprofile: %w", err)
		default:
			r.Coverprofile = name
		}
	}

	path := shard.ReceiptPath(sh.Dir, sh.Index)
	if err := shard.WriteReceipt(path, r); err != nil {
		return fmt.Errorf("unable to write shard receipt: %w", err)
	}
	log.WithFields("path", path).Debug("wrote shard receipt")
	return nil
}

// shardReceipt builds the receipt from the plan and what the run reported.
func shardReceipt(sh *shardRuntime, canopyVersion string, run *gotest.Run, passed bool) shard.Receipt {
	r := shard.Receipt{
		Version:       shard.ReceiptVersion,
		CanopyVersion: canopyVersion,
		Index:         sh.Index,
		Total:         sh.Total,
		Digest:        sh.Digest,
		Inputs:        map[string]shard.ReceiptInput{},
		Metrics:       sh.Metrics,
		Runner:        shard.ReceiptRunner{CPUs: runtime.NumCPU()},
		Units:         []string{},
		Planned:       sh.Planned(),
		LoadMS:        sh.Plan.Loads[sh.Index-1],
		Reported:      []string{},
		Passed:        passed,
		Failures:      []shard.Failure{},
		Gates:         sh.Gates,
		Observations:  map[string]int64{},
	}
	for _, u := range sh.Plan.Units {
		r.Units = append(r.Units, u.Package)
	}
	for _, g := range sh.Inputs.Groups() {
		ri := shard.ReceiptInput{Digest: g.Digest()}
		switch g.Name {
		case shard.GroupWeights:
			ri.Source, ri.Measured, ri.Estimated = sh.Weights.Source, sh.Weights.Measured, sh.Weights.Estimated
		case shard.GroupSelection:
			// package lines are too long to keep; the digest covers them
			for _, l := range g.Lines {
				if !strings.HasPrefix(l, "package ") {
					ri.Lines = append(ri.Lines, l)
				}
			}
		default:
			ri.Lines = g.Lines
		}
		r.Inputs[g.Name] = ri
	}

	if run != nil {
		addRunResults(&r, &run.Result)
	}
	return r
}

// addRunResults fills in what the run reported: packages with a final event, fresh timings, failures and counts.
func addRunResults(r *shard.Receipt, res *gotest.Result) {
	failedPkgs := map[string]bool{}
	for _, p := range res.Packages() {
		switch res.ReferenceConclusiveAction(p) {
		case gotest.PassAction:
			// only fresh results count: PackagePhases rejects packages replayed from the go test cache
			if _, fresh := res.PackagePhases(p); fresh {
				var ms int64
				if e := res.ReferenceConclusion(p); e != nil && e.Elapsed != nil {
					ms = int64(math.Round(*e.Elapsed * 1000))
				}
				r.Observations[p.Package] = ms
			}
		case gotest.FailAction:
			failedPkgs[p.Package] = true
		case gotest.SkipAction:
		default:
			continue
		}
		r.Reported = append(r.Reported, p.Package)
	}
	slices.Sort(r.Reported)

	for _, ref := range res.TestReferencesByAction(gotest.FailAction) {
		delete(failedPkgs, ref.Package)
		if len(r.Failures) < maxReceiptFailures {
			r.Failures = append(r.Failures, shard.Failure{Package: ref.Package, Test: ref.TestName(false)})
		}
	}
	for p := range failedPkgs {
		r.FailedPkgs = append(r.FailedPkgs, p)
	}
	slices.Sort(r.FailedPkgs)

	stats := res.TestStats()
	r.Tests = shard.TestTally{Passed: stats.Passed, Failed: stats.Failed, Skipped: stats.Skipped}
	r.ElapsedMS = res.Elapsed(false).Milliseconds()
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o600) //nolint:gosec // the path is under the configured shard dir
}

// announceShard sets the trailer and prints the plan header before a sharded run starts.
func announceShard(cfg *testConfig, sh *shardRuntime) {
	cfg.ShardTrailer = shardTrailer(sh)
	if !writesJSONToStdout(cfg.Writers) {
		printShardHeader(os.Stdout, cfg.Grouping.ToAPIConfig().Formatter, sh)
	}
}

// finishEmptyShard reports a shard that got no packages and still writes its receipt.
func finishEmptyShard(cfg *testConfig, sh *shardRuntime, canopyVersion string) error {
	if !writesJSONToStdout(cfg.Writers) {
		printEmptyShard(os.Stdout, sh, cfg.Color != "off")
	}
	return writeShardReceipt(sh, canopyVersion, nil, true)
}
