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
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
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
}

func (o *triageOpts) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Run, "run", "", "run ID to triage (default: the last run)")
	flags.StringVarP(&o.Reference, "reference", "", "triage only this reference (pkg/TestName)")
	flags.BoolVarP(&o.ShowRepros, "show-repros", "", "show the `go test` repro command under each failure")
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
		Short: "classify the failures in a run as flaky, pre-existing, or new-regression",
		Long: `Emit a verdict for each failure in a test run so an agent (or human) knows
which failures are worth acting on.

For each failed test in the target run, the verdict is one of:
  flaky          - the test intermittently passes and fails; don't chase it
  pre-existing   - this failure predates the target run; you likely didn't cause it
  new-regression - a new failure unique to this run; most likely worth fixing

Examples:
  # triage the last run
  canopy triage

  # triage a specific run
  canopy triage --run <run-id>

  # triage a single test
  canopy triage --reference 'github.com/org/repo/pkg/TestUserLogin'

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

	results, err := triageFailures(analyzer, failures, cfg.Triage.Reference, runInfo.Started)
	if err != nil {
		return err
	}

	if cfg.Output == formatJSON {
		return displayTriageJSON(results)
	}
	displayTriageSummary(results, runID.String(), cfg.Triage.ShowRepros)
	return nil
}

// triageFailures selects the failures to report (optionally narrowed to a single
// reference), analyzes their history in ONE pass over the store, and returns results
// sorted most-actionable first. analyzing per-ref would re-scan the whole store for
// each failure, so refs are batched through AnalyzeRefs.
func triageFailures(analyzer *flaky.Analyzer, failures []runFailure, only string, targetStart time.Time) ([]triageResultJSON, error) {
	selected := failures[:0:0]
	for _, f := range failures {
		if only != "" && f.ref.String(false) != only && f.ref.String(true) != only {
			continue
		}
		selected = append(selected, f)
	}

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
	return fmt.Sprintf("go test %s -run '^%s$'", ref.Package, pattern)
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
	return out, nil
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

func displayTriageJSON(results []triageResultJSON) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
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

// triageBreakdown renders the aux rollup, listing only the non-zero verdict categories:
//
//	"55 failures (all pre-existing)" when one category covers everything,
//	"55 failures (54 pre-existing, 1 flaky)" otherwise.
func triageBreakdown(total int, counts [4]int) string {
	labels := [3]Verdict{VerdictNewRegression, VerdictPreExisting, VerdictFlaky}
	var parts []string
	var lastName string
	for i, label := range labels {
		if counts[i] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[i], label))
			lastName = string(label)
		}
	}

	noun := "failures"
	if total == 1 {
		noun = "failure"
	}

	switch len(parts) {
	case 0:
		return fmt.Sprintf("%d %s", total, noun)
	case 1:
		return fmt.Sprintf("%d %s (all %s)", total, noun, lastName)
	default:
		return fmt.Sprintf("%d %s (%s)", total, noun, strings.Join(parts, ", "))
	}
}

func displayTriageSummary(results []triageResultJSON, runID string, showRepros bool) {
	if len(results) == 0 {
		fmt.Printf("No failures in run %s.\n", runID)
		return
	}

	var counts [4]int
	for _, r := range results {
		counts[verdictRank(r.Verdict)]++
	}

	for _, r := range results {
		// Width pads to align refs by visible width, ignoring ANSI escapes
		label := verdictStyle(r.Verdict).Width(15).Render(string(r.Verdict))
		fmt.Printf("%s %s\n", label, r.Reference)
		if showRepros {
			fmt.Printf("%-15s %s\n", "", styleAux.Render(r.Repro))
		}
	}

	// rollup footer (stats + hints) is aux: grey, and to stderr so stdout stays the pure report
	fmt.Fprintf(os.Stderr, "\n%s\n", styleAux.Render(triageBreakdown(len(results), counts)))

	if !showRepros {
		fmt.Fprintln(os.Stderr, styleAux.Render("run with --show-repros to see a `go test` command per failure"))
	}
}
