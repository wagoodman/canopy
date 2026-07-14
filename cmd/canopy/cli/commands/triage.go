package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
	"github.com/wagoodman/canopy/cmd/canopy/internal/localize"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

// Verdict classifies whether a failure is worth an agent's (or human's) attention.
type Verdict string

const (
	// VerdictFlaky means the test intermittently passes and fails; don't chase it.
	VerdictFlaky Verdict = "flaky"
	// VerdictPreExisting means this failure fingerprint predates the target run.
	VerdictPreExisting Verdict = "pre-existing"
	// VerdictNewRegression means this failure is new to the target run.
	VerdictNewRegression Verdict = "new-regression"
)

// deriveVerdict classifies a single failure. it is pure (no DB/IO) so it is trivially
// testable. precedence is decisive: flaky dominates (the signal is "don't chase this"),
// then a fingerprint seen in an earlier run is pre-existing, otherwise it is a new
// regression. the analysis passed here reflects the test's PRIOR history (the target
// run's own failure is excluded by the caller) so IsFlaky means "was flaky before now".
func deriveVerdict(analysis *flaky.Analysis, currentFingerprint string, priorFingerprints map[string]bool) Verdict {
	if analysis != nil && analysis.IsFlaky() {
		return VerdictFlaky
	}
	if priorFingerprints[currentFingerprint] {
		return VerdictPreExisting
	}
	return VerdictNewRegression
}

// triageOpts carries triage-specific flags not covered by the reused flaky options.
type triageOpts struct {
	Run        string `yaml:"run" json:"run" mapstructure:"run"`
	Reference  string `yaml:"reference" json:"reference" mapstructure:"reference"`
	ShowRepros bool   `yaml:"show-repros" json:"show-repros" mapstructure:"show-repros"`
	Cluster    bool   `yaml:"cluster" json:"cluster" mapstructure:"cluster"`
	Since      string `yaml:"since" json:"since" mapstructure:"since"`
}

func (o *triageOpts) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Run, "run", "", "run ID to triage (default: the last run)")
	flags.StringVarP(&o.Reference, "reference", "", "triage only this reference (pkg/TestName)")
	flags.BoolVarP(&o.ShowRepros, "show-repros", "", "show the `go test` repro command under each failure")
	// ponytail: --cluster is now the default (and only) view; kept as a no-op alias so existing
	// `triage --cluster` invocations keep working. drop it on the next flag-breaking release.
	flags.BoolVarP(&o.Cluster, "cluster", "", "deprecated: symptom clustering is always on now (no-op)")
	flags.StringVarP(&o.Since, "since", "", "git ref to scope the root-cause change set against (default: the dirty working tree)")
}

type triageConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	options.Flaky  `yaml:"flaky" json:"flaky" mapstructure:"flaky"`
	Triage         triageOpts `yaml:"triage" json:"triage" mapstructure:"triage"`
}

// Triage creates a command that emits a per-failure verdict (flaky, pre-existing, or
// new-regression) for the failures in a test run. This is the signal raw `go test`
// cannot provide: whether a failure is worth acting on.
func Triage(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true
	opts := &triageConfig{
		Store: store,
		Flaky: options.DefaultFlaky(),
	}

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "fuse a run's failures into symptoms, verdicts, and root causes",
		Long: `Produce one holistic report for a test run: X failures grouped into Y distinct
symptoms, each pointing to Z changed root causes, with every symptom showing its
verdict and its root cause side by side.

For each failed test, the verdict is one of:
  flaky          - the test intermittently passes and fails; don't chase it
  pre-existing   - this failure predates the target run; you likely didn't cause it
  new-regression - a new failure unique to this run; most likely worth fixing

Failures are grouped by symptom and, when a diff is available, every failure is
localized to the changed symbols that statically explain it, regardless of verdict.
A flaky failure that still reaches a changed symbol keeps its flaky label but also
shows the diff-based root cause ("flaky per history, but explained by your diff").

triage reads ONE run (this run plus its history and the diff), so it describes what
is wrong now and why, never what got fixed. To gate a change against a baseline run
(did I fix the target and break nothing new), see 'verify'.

Examples:
  # triage the last run
  canopy triage

  # triage a specific run
  canopy triage --run <run-id>

  # triage a single test
  canopy triage --reference 'github.com/org/repo/pkg/TestUserLogin'

  # scope the root-cause change set against a git ref (default: the dirty working tree)
  canopy triage --since main

  # emit machine-readable JSON
  canopy triage --output json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTriage(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

// triageResultJSON is the per-failure object emitted in JSON mode.
type triageResultJSON struct {
	Reference  string            `json:"reference"`
	Verdict    Verdict           `json:"verdict"`
	Real       bool              `json:"real"`
	FlakyScore float64           `json:"flaky_score"`
	History    triageHistoryJSON `json:"history"`
	Failure    triageFailureJSON `json:"failure"`
	Repro      string            `json:"repro"`
}

// triageHistoryJSON summarizes the test's history strictly before the target run.
type triageHistoryJSON struct {
	RecentRuns  int        `json:"recent_runs"`
	PassRate    float64    `json:"pass_rate"`
	LastFailure *time.Time `json:"last_failure"`
}

// triageFailureJSON is the distilled failure detail. type-specific fields are omitted
// when they don't apply (e.g. panics carry no expected/actual).
type triageFailureJSON struct {
	Type     failure.Type `json:"type"`
	Expected string       `json:"expected,omitempty"`
	Actual   string       `json:"actual,omitempty"`
	Location string       `json:"location,omitempty"`
}

// runFailure pairs a failed test reference with its structured failure detail.
type runFailure struct {
	ref    gotest.Reference
	detail db.FailedTestDetails
}

func runTriage(cfg triageConfig) error {
	log.Info("triaging failures from the target run")

	mgr, err := test.NewManager(test.Config{
		DBRoot:    cfg.Root,
		Ephemeral: cfg.Ephemeral,
	})
	if err != nil {
		return fmt.Errorf("unable to create test manager: %w", err)
	}
	defer func() {
		if err := mgr.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test manager")
		}
	}()

	// resolve the target run (reuses the last-run resolution from the show command)
	runID, err := resolveRunID(mgr, cfg.Triage.Run)
	if err != nil {
		return err
	}

	runInfo, err := mgr.GetRunInfo(runID)
	if err != nil {
		return fmt.Errorf("unable to get run info: %w", err)
	}

	failures, err := collectRunFailures(mgr, runID)
	if err != nil {
		return fmt.Errorf("unable to collect run failures: %w", err)
	}

	// reuse the flaky analyzer as the single history-walking engine; mirror its threshold
	// and window knobs rather than inventing new ones.
	analyzer := flaky.NewAnalyzer(mgr, flaky.Config{
		Window:          cfg.Window,
		MinRuns:         cfg.MinRuns,
		Threshold:       cfg.Threshold,
		ExcludePatterns: cfg.ExcludePatterns,
	})

	selected := selectFailures(failures, cfg.Triage.Reference)

	// the holistic report fuses three layers over the same failures: group by symptom, verdict each
	// failure by its history, and localize EVERY failure to the changed symbols that explain it.
	// localization runs regardless of verdict: a flaky failure that still reaches a changed symbol
	// keeps its flaky label but also shows the diff-based root cause, so "flaky per history, but
	// explained by your diff" is visible instead of hidden. the --cluster flag is now the default
	// view (kept as a harmless alias); both paths converge on this one report.
	clusters := clusterFailures(selected)

	results, err := triageResults(analyzer, selected, runInfo.Started)
	if err != nil {
		return err
	}
	verdictByRef := make(map[string]Verdict, len(results))
	for _, r := range results {
		verdictByRef[r.Reference] = r.Verdict
	}

	// localize is enrichment, not a gate: it engages only when a diff is available and any
	// localization failure degrades to the plain symptom-grouped report rather than sinking triage.
	loc := localizeFailures(cfg.Triage.Since, allReferences(selected))

	groups := fuseGroups(clusters, verdictByRef, loc)
	distinct := distinctTopCauses(groups)

	if cfg.Output == formatJSON {
		return displayTriageReportJSON(groups, len(selected), len(groups), len(distinct), loc)
	}
	displayTriageReport(groups, fusedSummary(clusters.Summary, distinct), runID.String(), abbrevRefs(selected), cfg.Triage.ShowRepros)
	return nil
}

// allReferences returns every selected failure's reference (the localization input). all failures
// are localized regardless of verdict: reachability to a changed symbol is evidence worth showing
// even for a flaky failure, so the reader sees "flaky per history, but explained by your diff"
// rather than having the flaky verdict mask a real diff-based cause.
func allReferences(selected []runFailure) []gotest.Reference {
	out := make([]gotest.Reference, len(selected))
	for i, f := range selected {
		out[i] = f.ref
	}
	return out
}

// localizeFailures resolves the change set (the dirty working tree, or --since <ref>), extracts
// its changed symbols, scopes a call graph to the affected packages, and ranks the changed symbols
// by how many failures reach them. Returns nil (no annotation) when there is no diff, no changed
// symbols, or nothing to attribute, in which case triage reads as plain symptom-grouped verdicts. Errors
// are logged and swallowed: localization is a best-effort layer, never a reason to fail the report.
func localizeFailures(since string, failures []gotest.Reference) *localize.Result {
	if len(failures) == 0 {
		return nil
	}

	var (
		changedFiles []string
		err          error
	)
	if since != "" {
		changedFiles, err = source.ChangedGoFilesSince(".", since)
	} else {
		changedFiles, err = source.ChangedGoFiles(".")
	}
	if err != nil {
		log.WithFields("error", err).Debug("root-cause localization skipped: unable to resolve changed files")
		return nil
	}
	if len(changedFiles) == 0 {
		return nil
	}

	changed, err := localize.ChangedSymbols(changedFiles)
	if err != nil {
		log.WithFields("error", err).Debug("root-cause localization skipped: unable to extract changed symbols")
		return nil
	}
	if len(changed) == 0 {
		return nil
	}

	affected, err := affectedImportPathsFromFiles([]string{options.DefaultPackageSpecifier}, changedFiles)
	if err != nil {
		log.WithFields("error", err).Debug("root-cause localization skipped: unable to compute affected packages")
		return nil
	}

	res, err := localize.Localize(affected.List(), changed, failures)
	if err != nil {
		log.WithFields("error", err).Warn("root-cause localization failed; reporting symptom-grouped verdicts only")
		return nil
	}
	return res
}

// triageResults analyzes the selected failures' history in ONE pass over the store and returns
// results sorted most-actionable first. analyzing per-ref would re-scan the whole store for each
// failure, so refs are batched through AnalyzeRefs.
func triageResults(analyzer *flaky.Analyzer, selected []runFailure, targetStart time.Time) ([]triageResultJSON, error) {
	refs := make([]gotest.Reference, len(selected))
	for i, f := range selected {
		refs[i] = f.ref
	}
	analysisByRef, err := analyzer.AnalyzeRefs(refs)
	if err != nil {
		return nil, fmt.Errorf("unable to analyze failures: %w", err)
	}

	results := make([]triageResultJSON, 0, len(selected))
	for _, f := range selected {
		results = append(results, buildResult(f, analysisByRef[f.ref], targetStart))
	}

	// most-actionable first: new-regression, then pre-existing, then flaky.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Verdict != results[j].Verdict {
			return verdictRank(results[i].Verdict) < verdictRank(results[j].Verdict)
		}
		return results[i].Reference < results[j].Reference
	})
	return results, nil
}

// selectFailures narrows failures to a single reference (matched against both raw and cleaned
// forms) when only is set, else returns all. shared by the triage and cluster paths.
func selectFailures(failures []runFailure, only string) []runFailure {
	if only == "" {
		return failures
	}
	out := failures[:0:0]
	for _, f := range failures {
		if f.ref.String(false) == only || f.ref.String(true) == only {
			out = append(out, f)
		}
	}
	return out
}

// buildResult produces a single per-failure result. the flaky determination and history
// use the PRIOR view (target run excluded) so a test that only ever passed before now
// reads as a regression, not as flaky.
func buildResult(f runFailure, analysis *flaky.Analysis, targetStart time.Time) triageResultJSON {
	priorPass, priorFail, priorPrints, lastFailure := priorView(analysis, targetStart)

	priorAnalysis := &flaky.Analysis{PassCount: priorPass, FailCount: priorFail}
	verdict := deriveVerdict(priorAnalysis, f.detail.Fingerprint, priorPrints)

	var score, passRate float64
	if total := priorPass + priorFail; total > 0 {
		passRate = float64(priorPass) / float64(total)
		score = flaky.CalculateFlakyScoreFromRate(passRate)
	}

	return triageResultJSON{
		Reference:  f.ref.String(false),
		Verdict:    verdict,
		Real:       verdict != VerdictFlaky,
		FlakyScore: score,
		History: triageHistoryJSON{
			RecentRuns:  priorPass + priorFail,
			PassRate:    passRate,
			LastFailure: lastFailure,
		},
		Failure: buildFailure(f.detail),
		Repro:   buildRepro(f.ref),
	}
}

// priorView projects a flaky analysis onto the window strictly before the target run,
// removing the target run's own failure. failures are counted from the analyzer's
// failure modes (which carry run times); passes are taken as-is since the target run
// failed for this reference and nothing runs after the last run.
//
// ponytail: prior passes are not time-filtered (the Analysis does not timestamp passes).
// for the default "last run" target this is exact. an explicit older --run could include
// later passes in the count; the verdict itself is unaffected. upgrade path if it matters:
// have the analyzer expose per-run pass times.
func priorView(analysis *flaky.Analysis, targetStart time.Time) (passCount, failCount int, fingerprints map[string]bool, lastFailure *time.Time) {
	fingerprints = map[string]bool{}
	if analysis == nil {
		return 0, 0, fingerprints, nil
	}
	passCount = analysis.PassCount
	for _, fm := range analysis.FailureModes {
		for _, r := range fm.Runs {
			if !r.Time.Before(targetStart) {
				continue
			}
			failCount++
			fingerprints[fm.Fingerprint] = true
			if lastFailure == nil || r.Time.After(*lastFailure) {
				t := r.Time
				lastFailure = &t
			}
		}
	}
	return passCount, failCount, fingerprints, lastFailure
}

// buildFailure distills a stored failure into the compact triage shape.
func buildFailure(detail db.FailedTestDetails) triageFailureJSON {
	f := triageFailureJSON{Type: failure.Type(detail.Type)}
	if detail.LocationFile != "" {
		f.Location = fmt.Sprintf("%s:%d", detail.LocationFile, detail.LocationLine)
	}
	// expected/actual only apply to assertion failures
	if f.Type == failure.AssertionFailure && len(detail.Details) > 0 {
		var ai failure.AssertionInfo
		if err := json.Unmarshal(detail.Details, &ai); err == nil {
			f.Expected = ai.Expected
			f.Actual = ai.Actual
		}
	}
	return f
}

// buildRepro renders the `go test` command that reproduces just this failure.
func buildRepro(ref gotest.Reference) string {
	pattern := ref.FuncName
	if ref.TRunName != "" {
		pattern = ref.FuncName + "/" + ref.SubTestName(true)
	}
	return reproCommand(ref.Package, pattern)
}

// reproCommand renders a `go test` invocation anchoring pattern as a full-match -run regex. shared
// by the single-failure and cluster repros so the anchoring/quoting lives in one place.
func reproCommand(pkg, pattern string) string {
	return fmt.Sprintf("go test %s -run '^%s$'", pkg, pattern)
}

// collectRunFailures returns the individual (non-package) test failures of a run, paired
// with their structured detail. failure rows are written one-per-fail-event in event
// order, so we correlate them positionally exactly as the flaky analyzer does.
//
// ponytail: positional correlation assumes one failure row per fail event. a fail event
// with empty output writes no row, which would shift alignment; in practice every real
// fail event has output. upgrade path: join by the DB event index.
func collectRunFailures(mgr *test.Manager, runID uuid.UUID) ([]runFailure, error) {
	events, err := mgr.GetTestEvents(runID)
	if err != nil {
		return nil, err
	}
	details, err := mgr.GetFailuresByRun(runID)
	if err != nil {
		return nil, err
	}

	var out []runFailure
	failIdx := 0
	for i := range events {
		e := events[i]
		if e.Action != gotest.FailAction {
			continue
		}
		// advance the cursor on every fail event (package-level included) to stay aligned.
		var detail *db.FailedTestDetails
		if failIdx < len(details) {
			detail = &details[failIdx]
		}
		failIdx++

		if e.Reference.IsPackage() || detail == nil {
			continue
		}
		out = append(out, runFailure{ref: e.Reference, detail: *detail})
	}
	return dropRedundantParents(out), nil
}

// dropRedundantParents removes a failing test's own failure when BOTH a subtest descendant in the
// same run also failed AND this failure carries no independent detail (a pure "subtests failed"
// aggregate). go reports a parent as failed whenever any subtest fails, so without this the parent
// shows up as a phantom failure alongside its real subtest causes. shared collection point, so
// triage, verify, and cluster all benefit.
//
// ponytail: aggregate-detection is a heuristic (generic type + no assertion + no location), not
// explicit event linkage. a parent that ran its own t.Error before spawning subtests keeps a real
// assertion, so it is NOT a pure aggregate and is preserved. upgrade path if the heuristic proves
// too coarse: join parent/child by explicit go-test event parentage.
func dropRedundantParents(failures []runFailure) []runFailure {
	out := failures[:0:0]
	for i, f := range failures {
		if isPureAggregate(f.detail) && hasFailingDescendant(f.ref, failures, i) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// isPureAggregate reports whether a failure carries no independent detail of its own: a generic
// (non-structured) type with no assertion expected/actual and no source location. such a failure is
// just "a child failed", not a parent-level assertion.
func isPureAggregate(detail db.FailedTestDetails) bool {
	switch failure.Type(detail.Type) {
	case failure.AssertionFailure, failure.PanicFailure, failure.DiffFailure, failure.TimeoutFailure:
		return false
	}
	f := buildFailure(detail)
	return f.Location == "" && f.Expected == "" && f.Actual == ""
}

// hasFailingDescendant reports whether any other failure (skipping index self) is a subtest
// descendant of ancestor within the same run.
func hasFailingDescendant(ancestor gotest.Reference, failures []runFailure, self int) bool {
	for j, other := range failures {
		if j == self {
			continue
		}
		if isSubtestDescendant(ancestor, other.ref) {
			return true
		}
	}
	return false
}

// isSubtestDescendant reports whether descendant is a subtest nested under ancestor: same package,
// same top-level function, and a subtest path that extends the ancestor's. a func-level ancestor
// (no TRunName) owns any subtest of that function; an intermediate subtest A/B owns A/B/C.
func isSubtestDescendant(ancestor, descendant gotest.Reference) bool {
	if ancestor.Package != descendant.Package || ancestor.FuncName != descendant.FuncName {
		return false
	}
	if ancestor.TRunName == descendant.TRunName {
		return false
	}
	if ancestor.TRunName == "" {
		return descendant.TRunName != ""
	}
	return strings.HasPrefix(descendant.TRunName, ancestor.TRunName+"/")
}

func verdictRank(v Verdict) int {
	switch v {
	case VerdictNewRegression:
		return 0
	case VerdictPreExisting:
		return 1
	case VerdictFlaky:
		return 2
	default:
		return 3
	}
}

// triageGroup fuses one symptom cluster with its members' verdicts and the root-cause candidates
// reachable from those members (in localize's global rank order, top first). pure intermediate
// shared by the table and JSON renderers.
type triageGroup struct {
	cluster    clusterJSON
	verdicts   []Verdict
	candidates []localize.Candidate
}

// fuseGroups correlates the three layers of the holistic report: it takes the symptom clusters,
// the per-failure verdicts, and the localization candidates, and attaches to each symptom group
// its members' distinct verdicts and every changed symbol reachable from a member (globally ranked
// so the symbol explaining the most failures leads). pure so the correlation is unit-testable.
func fuseGroups(clusters clusterResultJSON, verdictByRef map[string]Verdict, loc *localize.Result) []triageGroup {
	groups := make([]triageGroup, 0, len(clusters.Clusters))
	for _, c := range clusters.Clusters {
		groups = append(groups, triageGroup{
			cluster:    c,
			verdicts:   distinctVerdicts(c.References, verdictByRef),
			candidates: candidatesForGroup(c.References, loc),
		})
	}
	return groups
}

// distinctVerdicts returns the distinct verdicts across a group's members, most-actionable first.
// a symptom cluster usually shares one verdict, but distinct histories can split it; showing all
// present verdicts keeps the label honest.
func distinctVerdicts(refs []string, verdictByRef map[string]Verdict) []Verdict {
	seen := map[Verdict]bool{}
	var out []Verdict
	for _, ref := range refs {
		v, ok := verdictByRef[ref]
		if !ok || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return verdictRank(out[i]) < verdictRank(out[j]) })
	return out
}

// candidatesForGroup returns the changed symbols reachable from any of the group's members, keeping
// localize's global rank order (reached_by desc) so the symbol explaining the most failures across
// the whole run leads even within a single-member group. each candidate's references are narrowed
// to the members it actually reaches so the per-group reached_by is honest to the symptom.
func candidatesForGroup(refs []string, loc *localize.Result) []localize.Candidate {
	if loc == nil {
		return nil
	}
	member := make(map[string]bool, len(refs))
	for _, r := range refs {
		member[r] = true
	}
	var out []localize.Candidate
	for _, c := range loc.Candidates {
		var hit []string
		for _, r := range c.References {
			if member[r] {
				hit = append(hit, r)
			}
		}
		if len(hit) == 0 {
			continue
		}
		out = append(out, localize.Candidate{
			Symbol:     c.Symbol,
			Location:   c.Location,
			ReachedBy:  len(hit),
			References: hit,
		})
	}
	return out
}

// distinctTopCauses returns the distinct top-of-group root causes across all groups (first-seen
// order), the Z in "X failures across Y symptoms → Z root causes". a symptom's top cause is the
// globally-highest-ranked changed symbol reaching it; distinct symbols across groups are the
// separate things a reader would actually go fix, so two symptoms sharing one cause count as Z=1.
func distinctTopCauses(groups []triageGroup) []localize.Candidate {
	seen := map[string]bool{}
	var out []localize.Candidate
	for _, g := range groups {
		if len(g.candidates) == 0 {
			continue
		}
		top := g.candidates[0]
		if seen[top.Symbol] {
			continue
		}
		seen[top.Symbol] = true
		out = append(out, top)
	}
	return out
}

// triageGroupJSON is one symptom group in the holistic report: the clustered failures, their
// verdict(s), and the root-cause candidates reachable from the group's members.
type triageGroupJSON struct {
	Symptom             string               `json:"symptom"`
	Location            string               `json:"location,omitempty"`
	Count               int                  `json:"count"`
	References          []string             `json:"references"`
	Verdicts            []Verdict            `json:"verdicts"`
	RootCauseCandidates []localize.Candidate `json:"root_cause_candidates,omitempty"`
	SampleRepro         string               `json:"sample_repro"`
}

// triageReportJSON is the holistic triage report: X failures across Y symptoms → Z root causes,
// each symptom carrying its verdict(s) and root-cause candidates side by side. call_graph /
// localization_summary are omitted when no diff is present, so the report degrades to
// symptom-grouped verdicts.
type triageReportJSON struct {
	Failures            int               `json:"failures"`
	Symptoms            int               `json:"symptoms"`
	RootCauses          int               `json:"root_causes"`
	Groups              []triageGroupJSON `json:"groups"`
	CallGraph           string            `json:"call_graph,omitempty"`
	LocalizationSummary string            `json:"localization_summary,omitempty"`
}

func displayTriageReportJSON(groups []triageGroup, failures, symptoms, rootCauses int, loc *localize.Result) error {
	report := triageReportJSON{
		Failures:   failures,
		Symptoms:   symptoms,
		RootCauses: rootCauses,
		Groups:     make([]triageGroupJSON, 0, len(groups)),
	}
	for _, g := range groups {
		report.Groups = append(report.Groups, triageGroupJSON{
			Symptom:             g.cluster.Symptom,
			Location:            g.cluster.Location,
			Count:               g.cluster.Count,
			References:          g.cluster.References,
			Verdicts:            g.verdicts,
			RootCauseCandidates: g.candidates,
			SampleRepro:         g.cluster.SampleRepro,
		})
	}
	if loc != nil {
		report.CallGraph = loc.CallGraph
		report.LocalizationSummary = loc.Summary
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

// verdictStyle maps each verdict to a shared output style (see the style vars in affected.go)
// so triage and affected use color consistently: red = new/attributable to the current change,
// yellow = pre-existing caution, magenta = flaky anomaly.
func verdictStyle(v Verdict) lipgloss.Style {
	switch v {
	case VerdictNewRegression:
		return styleChange
	case VerdictPreExisting:
		return styleCaution
	case VerdictFlaky:
		return styleFlaky
	default:
		return lipgloss.NewStyle()
	}
}

// fusedSummary appends the localization layer to the symptom-grouping summary: "→ 1 root cause:
// <symbol>" when one changed symbol explains the symptoms, "→ N root causes" for several, and an
// honest "→ no root cause in the diff" when nothing in the change set is reachable.
func fusedSummary(clusterSummary string, distinct []localize.Candidate) string {
	switch len(distinct) {
	case 0:
		return clusterSummary + " → no root cause in the diff"
	case 1:
		return clusterSummary + " → 1 root cause: " + causeLabel(distinct[0])
	default:
		return fmt.Sprintf("%s → %d root causes", clusterSummary, len(distinct))
	}
}

// causeLabel renders a candidate for a one-line display, abbreviating the symbol and location to
// their last path segment (…/internal/flaky.calculateFlakyScore → flaky.calculateFlakyScore,
// cmd/…/analyzer.go:475 → analyzer.go:475). the JSON keeps the fully-qualified forms.
func causeLabel(c localize.Candidate) string {
	return fmt.Sprintf("%s (%s)", lastSegment(c.Symbol), lastSegment(c.Location))
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// displayTriageReport renders the holistic report: one line per symptom (count + symptom +
// location + verdict + root cause, side by side) with the member references indented beneath it,
// then the X/Y/Z summary. this is the same layout as the cluster view, fused with the verdict and
// root-cause pieces so all three layers appear together.
func displayTriageReport(groups []triageGroup, summary, runID string, abbrevByRef map[string]string, showRepros bool) {
	if len(groups) == 0 {
		fmt.Printf("No failures in run %s.\n", runID)
		return
	}
	for _, g := range groups {
		head := fmt.Sprintf("%s %s", styleChange.Render(fmt.Sprintf("×%d", g.cluster.Count)), g.cluster.Symptom)
		if g.cluster.Location != "" {
			// abbreviate to the basename for display (matching the root-cause location); the JSON
			// keeps the full stored path.
			head += " " + styleAux.Render("("+lastSegment(g.cluster.Location)+")")
		}
		if len(g.verdicts) > 0 {
			head += "  " + verdictLabel(g.verdicts)
		}
		if len(g.candidates) > 0 {
			head += "  " + styleAux.Render("↳ root cause: "+causeLabel(g.candidates[0]))
		}
		fmt.Println(head)
		for _, ref := range g.cluster.References {
			display := ref
			if a, ok := abbrevByRef[ref]; ok {
				display = a
			}
			fmt.Printf("  %s\n", display)
		}
		if showRepros {
			fmt.Printf("  %s\n", styleAux.Render(g.cluster.SampleRepro))
		}
	}

	// footer (summary + hint) is aux and on stderr so stdout stays the pure report.
	fmt.Fprintf(os.Stderr, "\n%s\n", styleAux.Render(summary))
	if !showRepros {
		fmt.Fprintln(os.Stderr, styleAux.Render("run with --show-repros to see a `go test` command per symptom"))
	}
}

// verdictLabel renders a group's distinct verdicts as a bracketed, color-coded label ("[flaky]",
// or "[new-regression, flaky]" when a symptom's members disagree).
func verdictLabel(vs []Verdict) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = verdictStyle(v).Render(string(v))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// abbrevRefs maps each selected failure's full reference to a compact display form (…/TestName),
// dropping the package path so the member list under each symptom stays readable.
func abbrevRefs(selected []runFailure) map[string]string {
	out := make(map[string]string, len(selected))
	for _, f := range selected {
		out[f.ref.String(false)] = "…/" + f.ref.TestName(true)
	}
	return out
}
