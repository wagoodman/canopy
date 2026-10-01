package commands

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var (
	_ ExitCoder       = (*exitError)(nil)
	_ SilentError     = (*exitError)(nil)
	_ fangs.FlagAdder = (*shardPlanConfig)(nil)
)

// exitError carries a specific exit code. Without err it is silent: the command already reported
// what went wrong in its own output.
type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}

func (e exitError) Unwrap() error  { return e.err }
func (e exitError) IsSilent() bool { return e.err == nil }
func (e exitError) ExitCode() int  { return e.code }

// Shard is the command group for CI matrix sharding (`canopy test --shard i/n` runs one shard).
func Shard(app clio.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shard",
		Short: "Plan and verify test runs split across CI jobs",
		Long: `Tools for running the test suite split across a CI matrix.

Each job runs one shard with 'canopy test --shard i/n', which runs its share of the packages
and writes a receipt. 'canopy shard join' then reads every receipt, verifies that each package
ran exactly once under the same inputs, merges coverage and timing metrics, and applies the
result gates (covermin). 'canopy shard plan' shows how packages would be split, without running
anything.

Both read the same config file, CANOPY_TEST_* env vars and flags as 'canopy test', from the
same 'test:' section.`,
	}

	cmd.AddCommand(
		ShardJoin(app),
		ShardPlan(app),
	)

	// the inherited help omits Long, which is where the overview lives
	xflagset.BindCobraHelpFromOpts(cmd, &struct{}{})

	return cmd
}

type shardJoinConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`

	Test shardJoinTestConfig `yaml:"test" json:"test" mapstructure:"test"`
}

// shardJoinTestConfig is the part of the test section the join reads, so the keys, env vars and
// flags match `canopy test`. New result gates on `canopy test` must be added here too.
type shardJoinTestConfig struct {
	options.Coverage `yaml:",inline" json:"" mapstructure:",squash"`

	Shard shardJoinShardConfig `yaml:"shard" json:"shard" mapstructure:"shard"`
}

// shardJoinShardConfig puts the join's outputs under test.shard.output, not test.output, which
// holds `canopy test`'s formats.
type shardJoinShardConfig struct {
	options.Shard  `yaml:",inline" json:"" mapstructure:",squash"`
	options.Format `yaml:",inline" json:"" mapstructure:",squash"`
}

func defaultShardJoinOptions() *shardJoinConfig {
	cov := options.DefaultCoverage()
	cov.CoverFlagDisabled = true // the join only enforces covermin; shards decide what to collect
	// NaN marks covermin as unset, so an explicit 0 on the join still overrides the receipts
	cov.CoverMin = math.NaN()
	return &shardJoinConfig{
		Test: shardJoinTestConfig{
			Coverage: cov,
			Shard: shardJoinShardConfig{
				Shard:  options.DefaultShard(),
				Format: options.DefaultShardJoinFormat(),
			},
		},
	}
}

// ShardJoin verifies and merges the receipts written by `canopy test --shard`.
func ShardJoin(app clio.Application) *cobra.Command {
	opts := defaultShardJoinOptions()

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Verify and merge the results of a sharded test run",
		Long: `Read the receipts every 'canopy test --shard i/n' job wrote to <shard-dir>/out and check that
the shards add up to one complete test run: every shard 1 to n is present, all shards ran with
the same inputs (packages, flags, toolchain, commit and weights), and every package ran exactly
once. It then merges the coverprofiles into <shard-dir>/coverage.out, applies covermin, folds
the fresh timings into <shard-dir>/metrics/ and suggests a shard count.

The join needs no checkout and no Go toolchain. covermin comes from the join's own config
(flag, env or .canopy.yaml) when set, otherwise from what the shards recorded.

Outputs (-o, repeatable, format[=path]):
  text             human readable report (the default, to stdout)
  json             the join report as versioned JSON
  github-summary   markdown appended to $GITHUB_STEP_SUMMARY (or to the given path)
With no -o, text is written, plus github-summary when $GITHUB_STEP_SUMMARY is set.

Exit codes (the most fundamental problem wins, every problem is reported):
  0  everything passed
  1  tests failed in a shard
  2  couldn't evaluate (no or unreadable receipts)
  3  verification failed (missing shard, different inputs, a package dropped or run twice)
  4  a gate failed (coverage below covermin, or no coverage to check)`,
		Example: fmt.Sprintf(`%[1]s shard join
  %[1]s shard join -o text -o json=join.json -o github-summary
  %[1]s shard join --covermin 80 --shard-dir .canopy/shard`, app.ID().Name),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runShardJoin(app.ID().Version, *opts)
		},
	}

	xflagset.BindCobraHelpFromOpts(cmd, opts)

	cmd = app.SetupCommand(cmd, opts)
	hideNaNDefault(opts.Test.NamedFlagSet, "covermin")
	return cmd
}

// hideNaNDefault keeps the unset sentinel out of the --help text.
func hideNaNDefault(nfs *xflagset.Named, name string) {
	for _, fs := range nfs.FlagSets {
		if f := fs.Lookup(name); f != nil {
			f.DefValue = "0"
		}
	}
}

func runShardJoin(canopyVersion string, cfg shardJoinConfig) error {
	sc := cfg.Test.Shard
	outDir := shard.OutDir(sc.Dir)
	receipts, err := shard.LoadDir(outDir)
	if err != nil {
		_ = sc.Writers.Close()
		return exitError{code: shard.ExitCannotRun, err: fmt.Errorf("unable to read receipts: %w", err)}
	}

	var coverMin *float64
	if !math.IsNaN(cfg.Test.CoverMin) {
		v := cfg.Test.CoverMin
		coverMin = &v
	}

	// metrics are always written here; whether they persist is up to the workflow
	report, joinErr := shard.Join(shard.JoinInput{
		Receipts:      receipts,
		OutDir:        outDir,
		ShardDir:      sc.Dir,
		CoverMin:      coverMin,
		WriteMetrics:  true,
		Overhead:      sc.ParsedOverhead(),
		CanopyVersion: canopyVersion,
	})
	for _, w := range report.Warnings {
		log.Warn(w)
	}

	// every output is written, even for a failed join, before the exit code is decided
	err = writeOutputs(sc.Writers, func(w io.Writer, format string, color bool) error {
		switch format {
		case formatText:
			return renderJoinText(w, report, color)
		case formatJSON:
			return writeJSON(w, report)
		case "github-summary":
			return renderJoinMarkdown(w, report)
		}
		return nil
	})
	if err = errors.Join(joinErr, err); err != nil {
		return exitError{code: shard.ExitCannotRun, err: err}
	}
	if report.ExitCode != shard.ExitPass {
		return exitError{code: report.ExitCode}
	}
	return nil
}

// writeOutputs renders to every writer (stdout when it has no file), carrying on past a failed
// one so every output that can be written is, then closes them.
func writeOutputs(writers options.FormatWriters, render func(w io.Writer, format string, color bool) error) error {
	var errs []error
	for _, fw := range writers {
		var w io.Writer = os.Stdout
		if fw.Writer != nil {
			w = fw.Writer
		}
		if err := render(w, fw.Name, fw.IsTTY); err != nil {
			errs = append(errs, fmt.Errorf("unable to write %s output: %w", fw.Name, err))
		}
	}
	if err := writers.Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// shardPlanConfig is the whole `canopy test` config, so the plan and its digest come out exactly as
// a shard with the same config and flags computes them.
type shardPlanConfig struct {
	TestCoreConfig `yaml:",inline" mapstructure:",squash"`

	// Shards is the shard count to plan for, 0 for every candidate count.
	Shards int            `yaml:"-" json:"-" mapstructure:"-"`
	Output options.Format `yaml:"-" json:"-" mapstructure:"-"`
}

func (o *shardPlanConfig) AddFlags(flags fangs.FlagSet) {
	flags.IntVarP(&o.Shards, "shards", "", "shard count to plan for (default: only show the shard count suggestion)")
}

func defaultShardPlanOptions() *shardPlanConfig {
	o := &shardPlanConfig{
		TestCoreConfig: *defaultTestOptions(withoutOpenOpts()),
		Output: options.Format{
			Outputs:          []string{formatText},
			AllowMultiple:    true,
			AllowableFormats: []string{formatText, formatJSON},
		},
	}
	// the plan never opens the store; forcing it on just avoids a temp dir for an ephemeral one
	o.HideEnabledFlag = true
	o.Test.ShardIndex.Disabled = true
	// test.output holds `canopy test`'s formats; the plan's -o is flag only
	o.Test.Format.Disabled = true
	return o
}

// ShardPlan shows how packages would be split across shards without running anything.
func ShardPlan(app clio.Application) *cobra.Command {
	opts := defaultShardPlanOptions()

	cmd := &cobra.Command{
		Use:   "plan [GO-PKG-SPECIFIER...]",
		Short: "Show how packages would be split across shards",
		Long: `Compute the shard plan 'canopy test --shard i/n' would run, without running any tests: the
packages each shard gets, their estimated load, the input digest and where the weights came from
(timing metrics in <shard-dir>/metrics/, or test counts without them), plus a shard count
suggestion.

The plan takes the same config, package selection and flags as 'canopy test', so given the same
ones it prints the digest every shard records. Without --shards only the suggestion is shown
(and -o json has a plan for every candidate shard count).`,
		Example: fmt.Sprintf(`%[1]s shard plan ./... --shards 4
  %[1]s shard plan ./... --no-cache --cover --shards 4 -o json
  %[1]s shard plan ./...`, app.ID().Name),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Test.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.Shards < 0 {
				return fmt.Errorf("invalid --shards %d (want n >= 1)", opts.Shards)
			}
			report, err := buildShardPlan(opts, app.ID().Version)
			if err != nil {
				return err
			}
			return writeOutputs(opts.Output.Writers, func(w io.Writer, format string, color bool) error {
				if format == formatJSON {
					return writeJSON(w, report)
				}
				return renderPlanText(w, report, color)
			})
		},
	}

	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

// shardPlanReport is the `canopy shard plan -o json` output.
type shardPlanReport struct {
	Packages int `json:"packages"`
	// Weights says where the weights came from; loads and weights are ms with metrics, test counts without.
	Weights    shard.ReportWeights `json:"weights"`
	Plans      []shardPlanTotal    `json:"plans"`
	Suggestion *shard.Suggestion   `json:"suggestion,omitempty"`
}

type shardPlanTotal struct {
	Total  int              `json:"total"`
	Digest string           `json:"digest"`
	Shards []shardPlanShard `json:"shards"`
}

type shardPlanShard struct {
	Index    int          `json:"index"`
	Load     int64        `json:"load"`
	Packages []shard.Unit `json:"packages"`
}

func buildShardPlan(opts *shardPlanConfig, canopyVersion string) (*shardPlanReport, error) {
	cfg := &opts.Test
	if _, err := selectTestPackages(cfg, canopyVersion); err != nil {
		return nil, err
	}
	in, err := resolveShardInputs(*cfg)
	if err != nil {
		return nil, err
	}

	w := in.Weights
	r := &shardPlanReport{
		Packages: len(w.Units),
		Weights: shard.ReportWeights{
			Source: w.Source, Measured: w.Measured, Estimated: w.Estimated,
			MetricsFile: in.Metrics.File, Ignored: w.Ignored,
		},
		Plans: []shardPlanTotal{},
	}

	totals := []int{opts.Shards}
	if opts.Shards == 0 {
		totals = nil
		for n := 1; n <= min(shard.MaxSuggestedShards, len(w.Units)); n++ {
			totals = append(totals, n)
		}
	}
	weights := map[string]shard.Unit{}
	for _, u := range w.Units {
		weights[u.Package] = u
	}
	for _, n := range totals {
		plan, inputs := in.forTotal(canopyVersion, n)
		pt := shardPlanTotal{Total: n, Digest: inputs.Digest()}
		for i, pkgs := range plan.Shards {
			s := shardPlanShard{Index: i + 1, Load: plan.Loads[i], Packages: []shard.Unit{}}
			for _, p := range pkgs {
				s.Packages = append(s.Packages, weights[p])
			}
			pt.Shards = append(pt.Shards, s)
		}
		r.Plans = append(r.Plans, pt)
	}

	if len(w.Units) > 0 {
		// the p the metrics were measured with, which is what `go test` would use on the same runners
		p := shard.SuggestionP(in.metrics, runtime.NumCPU())
		r.Suggestion = shard.NewSuggestion(w.Units, opts.Shards, cfg.Shard.ParsedOverhead(), p)
	}
	return r, nil
}

// renderPlanText writes the plan for a terminal in the join's style: one row per shard with its
// heaviest packages when planning for a count, then the shard count suggestion.
func renderPlanText(w io.Writer, r *shardPlanReport, color bool) error {
	var b strings.Builder
	st := style.NewGo(color)
	var paths []string
	for _, p := range r.Plans {
		for _, s := range p.Shards {
			for _, u := range s.Packages {
				paths = append(paths, u.Package)
			}
		}
	}
	if r.Suggestion != nil && r.Suggestion.Slowest != "" {
		paths = append(paths, r.Suggestion.Slowest)
	}
	trim := prefixTrimmer(paths)

	// a single plan is shown per shard; without --shards only the suggestion
	if len(r.Plans) == 1 {
		p := r.Plans[0]
		meta := strings.Join([]string{countOf(p.Total, "shard"), countOf(r.Packages, "package"), "inputs " + shortDigest(p.Digest)}, " · ")
		b.WriteString(st.Bold.Render("canopy shard plan") + "   " + st.Aux.Render(meta) + "\n\n")
		writePlanTable(&b, st, r, p, trim)
		b.WriteString("\n  " + st.Aux.Render("–") + " " + planWeightsText(r.Weights) + "\n")
		if line := planSuggestionLine(st, r.Suggestion, trim); line != "" {
			b.WriteString("  " + line + "\n")
		}
	} else {
		meta := countOf(r.Packages, "package") + " · " + planWeightsText(r.Weights)
		b.WriteString(st.Bold.Render("canopy shard plan") + "   " + st.Aux.Render(meta) + "\n")
		if r.Suggestion != nil {
			b.WriteString("\n")
			writePlanSuggestion(&b, st, r.Suggestion, trim)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// planWeightsText is `weights from metrics (23 measured, 34 estimated)`, or test counts and why.
func planWeightsText(wt shard.ReportWeights) string {
	if wt.Source == shard.SourceMetrics {
		return fmt.Sprintf("weights from metrics (%d measured, %d estimated)", wt.Measured, wt.Estimated)
	}
	return fmt.Sprintf("weights from test counts (%s)", wt.Ignored)
}

// writePlanTable writes one row per shard: package count, load and its three heaviest packages.
func writePlanTable(b *strings.Builder, st style.Go, r *shardPlanReport, p shardPlanTotal, trim func(string) string) {
	load := func(v int64) string {
		if r.Weights.Source == shard.SourceMetrics {
			return fmtMS(v)
		}
		return countOf(int(v), "test")
	}
	rows := [][]string{{"shard", colPkgs, "load", "heaviest"}}
	for _, s := range p.Shards {
		units := slices.Clone(s.Packages)
		slices.SortStableFunc(units, func(a, b shard.Unit) int { return cmp.Compare(b.Weight, a.Weight) })
		var heavy []string
		for _, u := range units[:min(3, len(units))] {
			v := load(u.Weight)
			if u.Estimated && r.Weights.Source == shard.SourceMetrics {
				v = "~" + v
			}
			heavy = append(heavy, trim(u.Package)+" "+v)
		}
		rows = append(rows, []string{fmt.Sprintf("%d/%d", s.Index, p.Total), fmt.Sprint(len(s.Packages)), load(s.Load), strings.Join(heavy, " · ")})
	}
	writeAlignedTable(b, st, rows, 1, 2)
}

// writeAlignedTable writes rows with an aux header row, right aligning the given numeric columns.
func writeAlignedTable(b *strings.Builder, st style.Go, rows [][]string, right ...int) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], len(c))
		}
	}
	for i, row := range rows {
		var line string
		for j, c := range row {
			pad := strings.Repeat(" ", widths[j]-len(c))
			switch {
			case slices.Contains(right, j):
				c = pad + c
			case j < len(row)-1:
				c += pad
			}
			gap := "   "
			if j == 0 {
				gap = "  "
			}
			line += gap + c
		}
		line = strings.TrimRight(line, " ")
		if i == 0 {
			line = st.Aux.Render(line)
		}
		b.WriteString(line + "\n")
	}
}

// planSuggestionLine is the one line verdict on a planned shard count: `✓ 3 shards is right` or
// `→ 4 shards would be ~6s faster`, and why when one package sets the floor.
func planSuggestionLine(st style.Go, s *shard.Suggestion, trim func(string) string) string {
	switch {
	case s == nil:
		return ""
	case s.Static:
		return st.Aux.Render("–") + " no shard count suggestion   " + st.Aux.Render("no timing data yet")
	case s.Best == s.Current:
		return st.Success.Render("✓") + " " + countOf(s.Current, "shard") + " is right   " + st.Aux.Render(fmtMS(suggestionWall(s, s.Best))+" est. wall")
	}
	note := suggestionEstimates(s)
	if s.Best < s.Current && s.Slowest != "" {
		note = fmt.Sprintf("%s alone takes %s", trim(s.Slowest), fmtMS(s.SlowestMS))
	}
	return "→ " + suggestionVerdict(s) + "   " + st.Aux.Render(note)
}

// writePlanSuggestion writes the verdict and the estimates worth comparing: up to two past the suggested
// count, skipping counts that only add runner time over the one before them.
func writePlanSuggestion(b *strings.Builder, st style.Go, s *shard.Suggestion, trim func(string) string) {
	if s.Static {
		b.WriteString(st.Aux.Render("–") + " no shard count suggestion   " + st.Aux.Render("no timing data yet") + "\n")
		return
	}
	verdict, why := countOf(s.Best, "shard"), ""
	one := suggestionWall(s, 1)
	switch {
	case s.Best == 1 && s.Slowest != "":
		verdict += " is enough"
		why = fmt.Sprintf("%s alone takes %s; more shards only add runner time", trim(s.Slowest), fmtMS(s.SlowestMS))
	case s.Best == 1:
		verdict += " is enough"
	default:
		why = fmt.Sprintf("~%s wall, down from %s on one runner", fmtMS(suggestionWall(s, s.Best)), fmtMS(one))
	}
	b.WriteString("→ " + verdict + "   " + st.Aux.Render(why) + "\n\n")

	rows := [][]string{{"shards", "est. wall", "runner time", ""}}
	var prev int64
	for _, e := range s.Estimates {
		if e.Shards > s.Best+2 {
			break
		}
		if e.Shards > 1 && e.Shards < s.Best && e.WallMS >= prev {
			continue
		}
		prev = e.WallMS
		mark := ""
		if e.Shards == s.Best {
			mark = st.Aux.Render("← suggested")
		}
		rows = append(rows, []string{fmt.Sprint(e.Shards), fmtMS(e.WallMS), fmtMS(e.RunnerMS), mark})
	}
	writeAlignedTable(b, st, rows, 0, 1, 2)
}

// suggestionWall is the est. wall for n shards, 0 when it wasn't estimated.
func suggestionWall(s *shard.Suggestion, n int) int64 {
	for _, e := range s.Estimates {
		if e.Shards == n {
			return e.WallMS
		}
	}
	return 0
}
