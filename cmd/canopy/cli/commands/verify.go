package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/scylladb/go-set/strset"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ SilentError = (*ErrVerifyFailed)(nil)

// ErrVerifyFailed carries a non-zero exit for a failed verification. it is silent: the report is
// already printed, so main should not tack on an "error:" line.
type ErrVerifyFailed struct{}

func (ErrVerifyFailed) Error() string { return "verification failed" }

func (ErrVerifyFailed) IsSilent() bool { return true }

// target status values. "fixed"/"passing" satisfy the ok gate; the rest do not.
const (
	targetFixed        = "fixed"         // failed in baseline, no longer fails in target
	targetPassing      = "passing"       // passed in both runs (nothing was broken to begin with)
	targetStillFailing = "still-failing" // fails in target and failed in baseline
	targetRegressed    = "regressed"     // passed in baseline, now fails in target
	targetNotRun       = "not-run"       // never reached a terminal outcome in the target run
)

// --target selectors. anything WITHOUT a leading @ is an explicit single test reference (today's
// path); the @-selectors derive a set of targets from the git diff.
const (
	selectorDiff   = "@diff"   // changed files in the working tree
	selectorBranch = "@branch" // changed files vs the merge-base of the default branch (or --base)
	selectorAuto   = "@auto"   // @branch on a feature branch, else @diff (default)
)

// verifyOpts carries verify-specific flags. options.Flaky is deliberately NOT reused (as triage
// does) because it already binds --session/-s for flaky-history scoping, which would collide with
// verify's own --session for run selection.
type verifyOpts struct {
	Session  string `yaml:"session" json:"session" mapstructure:"session"`
	Baseline string `yaml:"baseline" json:"baseline" mapstructure:"baseline"`
	Target   string `yaml:"target" json:"target" mapstructure:"target"`
	Base     string `yaml:"base" json:"base" mapstructure:"base"`
	Output   string `yaml:"output" json:"output" mapstructure:"output"`
}

func (o *verifyOpts) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Session, "session", "", "session to resolve the target/baseline runs from (or @branch, @module, @worktree)")
	flags.StringVarP(&o.Baseline, "baseline", "", "run ID to diff against (default: the prior run in the session)")
	flags.StringVarP(&o.Target, "target", "", "what the change was meant to fix: an explicit pkg/TestName, or a selector (@auto, @branch, @diff)")
	flags.StringVarP(&o.Base, "base", "", "branch/ref to diff against for @branch/@auto target resolution (default: detected default branch)")
	flags.StringVarP(&o.Output, "output", "o", "output format: table, json")
}

type verifyConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	Verify         verifyOpts `yaml:"verify" json:"verify" mapstructure:"verify"`
}

// Verify creates a command that diffs the current test run against a baseline and reports whether
// a change fixed its target and broke nothing new. It is the fix-verify loop closer: `triage` says
// what to ignore, `verify` says when to stop.
func Verify(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true
	opts := &verifyConfig{
		Store:  store,
		Verify: verifyOpts{Session: defaultSessionName, Target: selectorAuto, Output: formatTable},
	}

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "diff the current run against a baseline to confirm a fix broke nothing new",
		Long: `Diff the most recent run in a session against a baseline run and report one verdict:
did the change fix its target and break nothing new?

Failures in the target run are bucketed against the baseline, keyed by (reference, fingerprint):
  new-regression - fails now, its fingerprint was absent from the baseline (most likely yours)
  pre-existing   - the same failure was already present in the baseline (ignore)
  flaky          - the test intermittently passes and fails (dominates; don't chase)
and references that failed in the baseline but no longer fail are reported as fixed.

The --target names what the change was meant to fix. It resolves to a SET of test references:
  pkg/TestName - an explicit single test reference
  @diff        - tests in packages touched by the working tree
  @branch      - tests in packages touched since the merge-base of the default branch (or --base)
  @auto        - @branch on a feature branch, else @diff (the default)
Target references are exempt from the flaky bucket, so a genuine break-then-fix in the diff is
reported as fixed rather than swallowed as flakiness. Use --base to override the branch the
@branch/@auto diff is taken against (default: the detected default branch).

The baseline defaults to the prior run in the session; a first run with no baseline is a valid
state (all failures are treated as new), not an error.

'ok' is the one boolean to branch on: every target is fixed/passing (or no target resolved) AND
there are no new regressions. Exit code is 0 when ok, non-zero otherwise.

verify reads TWO runs (target vs baseline); that baseline is what lets it report a 'fixed'
bucket and decide the ok gate. To diagnose a single failing run (what's flaky vs real, its
distinct symptoms, and the likely cause) without a baseline, see 'triage'.

Examples:
  # verify the latest run: derive the targets from what changed on this branch
  canopy verify

  # scope the targets to the working-tree diff only
  canopy verify --target @diff

  # diff the branch against a specific base
  canopy verify --target @branch --base origin/main

  # name a single reference the change was meant to fix
  canopy verify --target 'github.com/org/repo/pkg/TestUserLogin'

  # diff against a specific baseline run
  canopy verify --baseline <run-id>

  # machine-readable JSON for an agent loop
  canopy verify --output json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runVerify(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runVerify(cfg verifyConfig) error {
	log.Info("verifying the latest run against a baseline")

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

	targetRun, baselineRun, noBaselineReason, err := resolveVerifyRuns(mgr, cfg.Verify)
	if err != nil {
		return err
	}
	log.WithFields("target", targetRun, "baseline", baselineRun, "reason", noBaselineReason).
		Debug("resolved runs to diff")

	targetFailures, baselineFailures, err := collectDiffFailures(mgr, targetRun, baselineRun)
	if err != nil {
		return err
	}

	// flaky determination reuses the analyzer over the union of involved refs; IsFlaky() is
	// independent of window/threshold, so a bare config is sufficient.
	involved := involvedRefs(targetFailures, baselineFailures)
	flakyRefs, err := flakyRefSet(mgr, involved)
	if err != nil {
		return fmt.Errorf("unable to analyze flakiness: %w", err)
	}
	log.WithFields("involved", len(involved), "flaky", len(flakyRefs)).
		Debug("analyzed flakiness over involved references")

	// resolve the target set (explicit ref or a diff-derived selector) before diffing so its refs
	// can bypass the flaky bucket and be classified normally.
	res, err := resolveTargets(mgr, cfg.Verify, targetRun, targetFailures, baselineFailures)
	if err != nil {
		return err
	}
	logTargetProvenance(res.Provenance, res.Reason)

	diff := diffRuns(targetFailures, baselineFailures, flakyRefs, res.Set)
	log.WithFields(
		"new-regressions", len(diff.NewRegressions),
		"still-failing", len(diff.StillFailing),
		"fixed", len(diff.Fixed),
		"flaky", len(diff.Flaky),
	).Debug("bucketed target failures against baseline")

	result := buildVerifyResult(diff, res.Targets, res.Provenance, noBaselineReason, res.Reason)

	if cfg.Verify.Output == formatJSON {
		if err := displayVerifyJSON(result); err != nil {
			return err
		}
	} else {
		displayVerifySummary(result, noBaselineReason, res.Reason)
	}

	if !result.OK {
		return ErrVerifyFailed{}
	}
	return nil
}

// logTargetProvenance emits the debug line describing what the targets were derived from.
func logTargetProvenance(p targetProvenance, reason string) {
	log.WithFields(
		"selector", p.Selector,
		"basis", firstNonEmpty(p.Basis, "none"),
		"changed-files", p.ChangedFiles,
		"affected-packages", p.AffectedPackages,
		"target-tests", p.TargetTests,
		"reason", reason,
	).Debug("resolved target set from the diff")
}

// collectDiffFailures gathers the target run's failures and, when a baseline exists, the
// baseline's. A nil baseline run is valid (a first run): baselineFailures comes back empty.
func collectDiffFailures(mgr *test.Manager, targetRun, baselineRun uuid.UUID) (targetFailures, baselineFailures []runFailure, err error) {
	targetFailures, err = collectRunFailures(mgr, targetRun)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to collect target failures: %w", err)
	}
	if baselineRun != uuid.Nil {
		baselineFailures, err = collectRunFailures(mgr, baselineRun)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to collect baseline failures: %w", err)
		}
	}
	log.WithFields("target", len(targetFailures), "baseline", len(baselineFailures)).
		Debug("collected run failures")
	return targetFailures, baselineFailures, nil
}

// gitSource is a seam over the source-package git file discovery so the selector dispatch is
// unit-testable without a repo. the production wiring is defaultGitSource.
type gitSource struct {
	changedGoFiles      func(dir string) ([]string, error)
	changedGoFilesSince func(dir, ref string) ([]string, error)
	currentBranch       func(dir string) string // "" if not a repo, "HEAD" if detached
	defaultBranch       func(dir string) (string, error)
	mergeBase           func(dir, baseRef string) (string, error)
}

func defaultGitSource() gitSource {
	return gitSource{
		changedGoFiles:      source.ChangedGoFiles,
		changedGoFilesSince: source.ChangedGoFilesSince,
		currentBranch:       currentBranch,
		defaultBranch:       source.DefaultBranch,
		mergeBase:           source.MergeBase,
	}
}

// currentBranch reports the working tree's branch, or "" outside a repo and "HEAD" when detached.
func currentBranch(dir string) string {
	state := source.CaptureState(dir)
	if state == nil {
		return ""
	}
	return state.Branch
}

// basisWorkingTree is the provenance phrase for a working-tree diff.
const basisWorkingTree = "working-tree changes"

// targetProvenance records how the target set was derived so the logs and the report can state
// what the diff was taken against instead of asking the reader to trust the numbers.
type targetProvenance struct {
	Selector         string `json:"selector"`          // @auto, @diff, @branch, or "explicit"
	Basis            string `json:"basis"`             // what was diffed, e.g. "working-tree changes"
	ChangedFiles     int    `json:"changed_files"`     // count of changed .go files
	AffectedPackages int    `json:"affected_packages"` // count of affected import paths
	TargetTests      int    `json:"target_tests"`      // count of resolved target references
}

// targetResolution bundles the resolved target set with its provenance so resolveTargets keeps a
// single return value as it grew.
type targetResolution struct {
	Targets    []verifyTargetJSON
	Set        map[string]bool // keyed by ref.String(false), for the flaky exemption
	Reason     string          // set when the set degraded or came up empty
	Provenance targetProvenance
}

// resolveTargets turns cfg.Target into the resolved target set and its provenance. An explicit ref
// (no leading @) keeps today's single-target behavior; the @-selectors derive their set from the
// git diff via the affected-package analysis.
func resolveTargets(mgr *test.Manager, opts verifyOpts, targetRun uuid.UUID, targetFailures, baselineFailures []runFailure) (targetResolution, error) {
	terminal, err := terminalRefs(mgr, targetRun)
	if err != nil {
		return targetResolution{}, fmt.Errorf("unable to read target run outcomes: %w", err)
	}

	// explicit single ref: preserve today's behavior, represented as a slice of one.
	if !strings.HasPrefix(opts.Target, "@") {
		ran := terminalRefSet(terminal)
		status := targetStatus(
			ran[opts.Target],
			refFails(opts.Target, targetFailures),
			refFails(opts.Target, baselineFailures),
		)
		return targetResolution{
			Targets:    []verifyTargetJSON{{Reference: opts.Target, Status: status}},
			Set:        map[string]bool{opts.Target: true},
			Provenance: targetProvenance{Selector: "explicit", Basis: "explicit reference", TargetTests: 1},
		}, nil
	}

	// diff-based selector: changed files -> affected import paths -> target refs among the run.
	files, basis, reason, ferr := resolveTargetFiles(defaultGitSource(), opts.Target, opts.Base)
	prov := targetProvenance{Selector: opts.Target, Basis: basis, ChangedFiles: len(files)}
	if ferr != nil {
		// degrade to targetless with a reason rather than failing the whole verify (e.g. outside a repo)
		prov.Basis = ""
		return targetResolution{Set: map[string]bool{}, Reason: fmt.Sprintf("could not resolve changed files (%v); no targets derived", ferr), Provenance: prov}, nil
	}
	if len(files) == 0 {
		return targetResolution{Set: map[string]bool{}, Reason: firstNonEmpty(reason, "no changed .go files; no targets derived"), Provenance: prov}, nil
	}

	affected, err := affectedImportPathsFromFiles([]string{options.DefaultPackageSpecifier}, files)
	if err != nil {
		return targetResolution{}, fmt.Errorf("unable to analyze affected packages: %w", err)
	}
	prov.AffectedPackages = affected.Size()

	refs := intersectTargets(terminal, affected)
	targets := classifyTargets(refs, targetFailures, baselineFailures)
	prov.TargetTests = len(targets)
	set := map[string]bool{}
	for _, t := range targets {
		set[t.Reference] = true
	}
	if len(targets) == 0 {
		reason = firstNonEmpty(reason, "no target tests in the changed packages")
	}
	return targetResolution{Targets: targets, Set: set, Reason: reason, Provenance: prov}, nil
}

// resolveTargetFiles resolves an @-selector to the set of changed .go files, plus a human basis
// phrase for the report and a reason string noting any degradation to a working-tree diff. Pure
// over the injected gitSource so the dispatch is unit-testable. @auto is sugar: a feature branch
// diffs against the default branch's merge-base, otherwise the working tree.
func resolveTargetFiles(g gitSource, selector, base string) (files []string, basis, reason string, err error) {
	switch selector {
	case selectorDiff:
		files, err = g.changedGoFiles(".")
		return files, basisWorkingTree, "", err
	case selectorBranch:
		return resolveBranchFiles(g, base)
	case selectorAuto:
		// an explicit --base means "diff the branch", so honor it via the @branch path
		if base != "" {
			return resolveBranchFiles(g, base)
		}
		branch := g.currentBranch(".")
		def, derr := g.defaultBranch(".")
		// on the default branch, detached HEAD, non-repo, or unknown default: working tree only
		if branch == "" || branch == "HEAD" || derr != nil || branch == def {
			files, err = g.changedGoFiles(".")
			return files, basisWorkingTree, "", err
		}
		return resolveBranchFiles(g, def)
	default:
		return nil, "", "", fmt.Errorf("unknown target selector %q", selector)
	}
}

// resolveBranchFiles diffs against the merge-base of base (or the detected default branch). Any
// step that fails degrades to a working-tree diff with a reason rather than erroring.
func resolveBranchFiles(g gitSource, base string) (files []string, basis, reason string, err error) {
	if base == "" {
		branch, dberr := g.defaultBranch(".")
		if dberr != nil {
			files, err = g.changedGoFiles(".")
			return files, basisWorkingTree, "could not determine default branch; diffed the working tree", err
		}
		base = branch
	}
	mb, err := g.mergeBase(".", base)
	if err != nil {
		files, err = g.changedGoFiles(".")
		return files, basisWorkingTree, fmt.Sprintf("could not find merge-base with %s; diffed the working tree", base), err
	}
	files, err = g.changedGoFilesSince(".", mb)
	return files, fmt.Sprintf("changes since merge-base with %s (%s)", base, shortSHA(mb)), "", err
}

// shortSHA abbreviates a commit hash for display.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// intersectTargets returns the run's terminal refs whose package is in the affected import-path
// set, sorted. Pure so the intersection is unit-testable.
func intersectTargets(terminal []gotest.Reference, affected *strset.Set) []gotest.Reference {
	var out []gotest.Reference
	for _, r := range terminal {
		if affected.Has(r.Package) {
			out = append(out, r)
		}
	}
	sortRefs(out)
	return out
}

// classifyTargets classifies each target ref with targetStatus. Terminal refs ran by definition,
// so the pass/fail flip against the baseline decides fixed/passing/still-failing/regressed.
func classifyTargets(refs []gotest.Reference, targetFailures, baselineFailures []runFailure) []verifyTargetJSON {
	out := make([]verifyTargetJSON, 0, len(refs))
	for _, r := range refs {
		ref := r.String(false)
		status := targetStatus(true, refFails(ref, targetFailures), refFails(ref, baselineFailures))
		out = append(out, verifyTargetJSON{Reference: ref, Status: status})
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// resolveVerifyRuns picks the target run (the most recent run in the resolved session) and the
// baseline run to diff against (explicit --baseline, else the prior run in the same session). A
// missing baseline is not an error: the returned reason explains it and callers treat every target
// failure as new.
func resolveVerifyRuns(mgr *test.Manager, opts verifyOpts) (target, baseline uuid.UUID, noBaselineReason string, err error) {
	store := mgr.DBStore()
	if store == nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("no persistent store available")
	}

	sessionName := resolveSessionName(opts.Session)
	log.WithFields("session", sessionName, "requested", opts.Session).Trace("resolving verify runs from session")
	sess, err := store.GetTestSessionByName(sessionName)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("unable to resolve session %q: %w", sessionName, err)
	}
	if sess == nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("no session %q found; run tests first", sessionName)
	}

	// GetTestSessionByName preloads TestRuns; use them directly. (db.GetSessionTestRuns keys the
	// int64 session_id column with a UUID string and so returns nothing.)
	var runs []db.TestRun
	if sess.TestRuns != nil {
		runs = *sess.TestRuns
	}
	if len(runs) == 0 {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("session %q has no runs", sessionName)
	}

	// most recent first
	sort.Slice(runs, func(i, j int) bool { return runs[i].Started.After(runs[j].Started) })
	runIDs := make([]string, len(runs))
	for i, r := range runs {
		runIDs[i] = r.UUID
	}

	log.WithFields("runs", len(runIDs), "explicit-baseline", opts.Baseline).Trace("picking target/baseline from session runs")
	targetID, baselineID, noBaselineReason, err := pickRuns(runIDs, opts.Baseline)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	target, err = uuid.Parse(targetID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("invalid run uuid %q: %w", targetID, err)
	}
	if baselineID != "" {
		baseline, err = uuid.Parse(baselineID)
		if err != nil {
			return uuid.Nil, uuid.Nil, "", fmt.Errorf("invalid baseline run ID %q: %w", baselineID, err)
		}
	}
	return target, baseline, noBaselineReason, nil
}

// pickRuns decides the target and baseline runs from a session's runs (already sorted most-recent
// first) and an optional explicit baseline. Pure so the resolution rules are testable without a DB:
// explicit --baseline wins; otherwise the baseline is the prior run in the session; a lone first run
// yields no baseline (a valid state, not an error).
func pickRuns(runIDs []string, explicitBaseline string) (target, baseline, noBaselineReason string, err error) {
	if len(runIDs) == 0 {
		return "", "", "", fmt.Errorf("session has no runs")
	}
	target = runIDs[0]
	if explicitBaseline != "" {
		return target, explicitBaseline, "", nil
	}
	if len(runIDs) < 2 {
		return target, "", "no baseline; treating all failures as new", nil
	}
	return target, runIDs[1], "", nil
}

// verifyDiff is the bucketed run-to-run comparison of the COLLATERAL failures: references that are
// not resolved targets (target refs are reported separately, via classifyTargets). References in
// Flaky are excluded from the other buckets: flaky dominates, since the signal is "don't chase this".
type verifyDiff struct {
	NewRegressions []runFailure       // fails now, fingerprint absent from baseline
	StillFailing   []runFailure       // same (reference, fingerprint) in both runs
	Fixed          []gotest.Reference // failed in baseline, no longer fails in target
	Flaky          []gotest.Reference // flaky references seen in either run
}

// diffRuns classifies target-run failures against baseline-run failures keyed by (reference,
// fingerprint). It is pure (no DB/IO) so the bucketing is trivially testable. flakyRefs is keyed by
// ref.String(false); any involved flaky reference is reported only in Flaky, UNLESS it is in
// targetSet: a resolved target bypasses the flaky bucket so a real break-then-fix in the diff is
// classified normally rather than swallowed as flakiness. targetSet is keyed by ref.String(false)
// (raw and clean forms are both matched, so an explicit --target given in either form works).
func diffRuns(target, baseline []runFailure, flakyRefs, targetSet map[string]bool) verifyDiff {
	baselineKeys := map[string]bool{}      // (reference, fingerprint)
	baselineRefFailed := map[string]bool{} // reference
	for _, f := range baseline {
		baselineKeys[fpKey(f.ref, f.detail.Fingerprint)] = true
		baselineRefFailed[f.ref.String(false)] = true
	}
	targetRefFailed := map[string]bool{}
	for _, f := range target {
		targetRefFailed[f.ref.String(false)] = true
	}

	isTarget := func(ref gotest.Reference) bool {
		return targetSet[ref.String(false)] || targetSet[ref.String(true)]
	}

	var d verifyDiff
	flakySeen := map[string]bool{}
	addFlaky := func(ref gotest.Reference) {
		rk := ref.String(false)
		if flakySeen[rk] {
			return
		}
		flakySeen[rk] = true
		d.Flaky = append(d.Flaky, ref)
	}

	// classify the target run's failures. target refs are omitted from every bucket: their status
	// is reported in the target list (classifyTargets), so keeping them here would double-report the
	// same failure as both a regressed target and a collateral new-regression.
	for _, f := range target {
		if isTarget(f.ref) {
			continue
		}
		rk := f.ref.String(false)
		if flakyRefs[rk] {
			addFlaky(f.ref)
			continue
		}
		if baselineKeys[fpKey(f.ref, f.detail.Fingerprint)] {
			d.StillFailing = append(d.StillFailing, f)
		} else {
			d.NewRegressions = append(d.NewRegressions, f)
		}
	}

	// fixed: baseline references (deduped) that no longer fail in the target run. target refs are
	// again omitted, reported in the target list instead.
	fixedSeen := map[string]bool{}
	for _, f := range baseline {
		if isTarget(f.ref) {
			continue
		}
		rk := f.ref.String(false)
		if targetRefFailed[rk] {
			continue
		}
		if flakyRefs[rk] {
			addFlaky(f.ref)
			continue
		}
		if fixedSeen[rk] {
			continue
		}
		fixedSeen[rk] = true
		d.Fixed = append(d.Fixed, f.ref)
	}

	sortFailures(d.NewRegressions)
	sortFailures(d.StillFailing)
	sortRefs(d.Fixed)
	sortRefs(d.Flaky)
	return d
}

// targetStatus classifies the named target reference. It is pure so the mapping is testable. A
// target that never ran is "not-run"; otherwise the pass/fail flip against the baseline decides.
func targetStatus(ran, failingNow, failingBefore bool) string {
	switch {
	case failingNow && failingBefore:
		return targetStillFailing
	case failingNow:
		return targetRegressed
	case !ran:
		return targetNotRun
	case failingBefore:
		return targetFixed
	default:
		return targetPassing
	}
}

// verifyResultJSON is the compact, agent-first verdict. 'ok' is the one boolean to branch on;
// 'summary' is the one line a human reads.
type verifyResultJSON struct {
	Targets        []verifyTargetJSON     `json:"targets"`
	TargetSource   targetProvenance       `json:"target_source"`
	NewRegressions []verifyRegressionJSON `json:"new_regressions"`
	// NewRegressionClusters collapses new_regressions by shared symptom so an agent fixes K
	// distinct symptoms, not N failures. omitted when there are no regressions.
	NewRegressionClusters []clusterJSON     `json:"new_regression_clusters,omitempty"`
	StillFailing          []verifyStillJSON `json:"still_failing"`
	FlakyIgnored          []string          `json:"flaky_ignored"`
	Summary               string            `json:"summary"`
	OK                    bool              `json:"ok"`
}

type verifyTargetJSON struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

// verifyRegressionJSON mirrors the triage failure shape (reused via buildFailure/buildRepro) so the
// two commands describe a failure identically.
type verifyRegressionJSON struct {
	Reference   string            `json:"reference"`
	Fingerprint string            `json:"fingerprint"`
	Failure     triageFailureJSON `json:"failure"`
	Repro       string            `json:"repro"`
}

type verifyStillJSON struct {
	Reference string  `json:"reference"`
	Verdict   Verdict `json:"verdict"`
}

// buildVerifyResult assembles the JSON verdict from the diff. ok = (every target fixed/passing OR
// no targets resolved) AND no new regressions.
func buildVerifyResult(d verifyDiff, targets []verifyTargetJSON, provenance targetProvenance, noBaselineReason, targetReason string) verifyResultJSON {
	if targets == nil {
		targets = []verifyTargetJSON{}
	}
	res := verifyResultJSON{
		Targets:        targets,
		TargetSource:   provenance,
		NewRegressions: make([]verifyRegressionJSON, 0, len(d.NewRegressions)),
		StillFailing:   make([]verifyStillJSON, 0, len(d.StillFailing)),
		FlakyIgnored:   make([]string, 0, len(d.Flaky)),
	}
	for _, f := range d.NewRegressions {
		res.NewRegressions = append(res.NewRegressions, verifyRegressionJSON{
			Reference:   f.ref.String(false),
			Fingerprint: f.detail.Fingerprint,
			Failure:     buildFailure(f.detail),
			Repro:       buildRepro(f.ref),
		})
	}
	for _, f := range d.StillFailing {
		res.StillFailing = append(res.StillFailing, verifyStillJSON{
			Reference: f.ref.String(false),
			Verdict:   VerdictPreExisting,
		})
	}
	for _, ref := range d.Flaky {
		res.FlakyIgnored = append(res.FlakyIgnored, ref.String(false))
	}
	// fold in the clustered view of the new regressions (reusing triage's clustering) so the
	// fan-out case reads as "K distinct symptoms" here too. left nil when there are none so JSON omits it.
	if len(d.NewRegressions) > 0 {
		res.NewRegressionClusters = clusterFailures(d.NewRegressions).Clusters
	}

	res.OK = targetsSatisfied(res.Targets) && len(res.NewRegressions) == 0
	res.Summary = verifySummary(res, noBaselineReason, targetReason)
	return res
}

// targetsSatisfied reports whether every resolved target is fixed or passing. An empty set is
// satisfied (a diff that touched no in-scope target tests is not a failure by itself).
func targetsSatisfied(targets []verifyTargetJSON) bool {
	for _, t := range targets {
		if t.Status != targetFixed && t.Status != targetPassing {
			return false
		}
	}
	return true
}

// verifySummary renders the one-line human summary, listing only non-empty buckets.
func verifySummary(res verifyResultJSON, noBaselineReason, targetReason string) string {
	var parts []string
	if noBaselineReason != "" {
		parts = append(parts, noBaselineReason)
	}
	if targetReason != "" {
		parts = append(parts, targetReason)
	}
	if s := targetSummary(res.Targets); s != "" {
		parts = append(parts, s)
	}
	if n := len(res.NewRegressions); n == 0 {
		parts = append(parts, "no new regressions")
	} else if k := len(res.NewRegressionClusters); k > 0 && k < n {
		// only surface clusters when they actually collapse the count (K < N), so a single
		// regression still reads plainly as "1 new regression".
		parts = append(parts, fmt.Sprintf("%d %s across %d distinct %s",
			n, plural(n, "new regression", "new regressions"),
			k, plural(k, "symptom", "symptoms")))
	} else {
		parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, "new regression", "new regressions")))
	}
	if n := len(res.StillFailing); n > 0 {
		parts = append(parts, fmt.Sprintf("%d pre-existing", n))
	}
	if n := len(res.FlakyIgnored); n > 0 {
		parts = append(parts, fmt.Sprintf("%d flaky ignored", n))
	}
	return strings.Join(parts, "; ")
}

// targetSummary condenses the resolved target set into one phrase: "target <status>" for a single
// target, "N targets (2 fixed, 1 still-failing)" for many, and "" when there are none.
func targetSummary(targets []verifyTargetJSON) string {
	switch len(targets) {
	case 0:
		return ""
	case 1:
		return "target " + targets[0].Status
	}
	counts := map[string]int{}
	for _, t := range targets {
		counts[t.Status]++
	}
	var parts []string
	for _, s := range []string{targetFixed, targetPassing, targetRegressed, targetStillFailing, targetNotRun} {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[s], s))
		}
	}
	return fmt.Sprintf("%d targets (%s)", len(targets), strings.Join(parts, ", "))
}

func displayVerifyJSON(res verifyResultJSON) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(res)
}

// styleOK is the green used for a satisfied target / passing verdict, completing the shared palette
// in affected.go (red = new/attributable, yellow = pre-existing, magenta = flaky).
var styleOK = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

// provenanceHeader renders the one-line "what were the targets derived from" header. Empty when no
// selector ran (nothing to justify).
func provenanceHeader(p targetProvenance) string {
	switch {
	case p.Selector == "":
		return ""
	case p.Selector == "explicit":
		return "target: explicit reference"
	case p.Basis == "":
		return fmt.Sprintf("targets: %s (unresolved)", p.Selector)
	}
	return fmt.Sprintf("targets: %s from %s (%d changed %s, %d affected %s)",
		p.Selector, p.Basis,
		p.ChangedFiles, plural(p.ChangedFiles, "file", "files"),
		p.AffectedPackages, plural(p.AffectedPackages, "package", "packages"))
}

func displayVerifySummary(res verifyResultJSON, noBaselineReason, targetReason string) {
	// provenance header states what the targets were derived from, so the report justifies its own
	// numbers rather than reading as trust-me. aux -> stderr, keeping stdout the pure report.
	if h := provenanceHeader(res.TargetSource); h != "" {
		fmt.Fprintln(os.Stderr, styleAux.Render(h))
	}
	if noBaselineReason != "" {
		fmt.Fprintln(os.Stderr, styleAux.Render(noBaselineReason))
	}
	if targetReason != "" {
		fmt.Fprintln(os.Stderr, styleAux.Render(targetReason))
	}

	// print each non-passing target; collapse passing ones to a single count so a big package
	// doesn't spam the report.
	passing := 0
	for _, t := range res.Targets {
		if t.Status == targetPassing {
			passing++
			continue
		}
		fmt.Printf("%s %s\n", verifyTargetStyle(t.Status).Width(15).Render(t.Status), t.Reference)
	}
	for _, r := range res.NewRegressions {
		fmt.Printf("%s %s\n", styleChange.Width(15).Render("new-regression"), r.Reference)
	}
	for _, r := range res.StillFailing {
		fmt.Printf("%s %s\n", styleCaution.Width(15).Render("pre-existing"), r.Reference)
	}
	for _, ref := range res.FlakyIgnored {
		fmt.Printf("%s %s\n", styleFlaky.Width(15).Render("flaky"), ref)
	}
	// passing targets are good news; trail them so the count never sits between problem rows.
	if passing > 0 {
		fmt.Printf("%s %d %s\n", styleOK.Width(15).Render("passing"), passing, plural(passing, "target", "targets"))
	}

	// verdict footer is aux and goes to stderr so stdout stays the pure report
	verdict := styleOK.Render("ok")
	if !res.OK {
		verdict = styleChange.Render("not ok")
	}
	fmt.Fprintf(os.Stderr, "\n%s: %s\n", verdict, styleAux.Render(res.Summary))
}

// verifyTargetStyle colors the target status line: green when satisfied, red/yellow otherwise.
func verifyTargetStyle(status string) lipgloss.Style {
	switch status {
	case targetFixed, targetPassing:
		return styleOK
	case targetStillFailing, targetNotRun:
		return styleCaution
	case targetRegressed:
		return styleChange
	default:
		return lipgloss.NewStyle()
	}
}

// flakyRefSet returns the set of references (keyed by ref.String(false)) that the analyzer deems
// flaky over their full history.
func flakyRefSet(mgr *test.Manager, refs []gotest.Reference) (map[string]bool, error) {
	out := map[string]bool{}
	if len(refs) == 0 {
		return out, nil
	}
	analyzer := flaky.NewAnalyzer(mgr, flaky.Config{})
	byRef, err := analyzer.AnalyzeRefs(refs)
	if err != nil {
		return nil, err
	}
	for ref, a := range byRef {
		if a != nil && a.IsFlaky() {
			out[ref.String(false)] = true
		}
	}
	return out, nil
}

// involvedRefs is the deduped union of references that failed in either run.
func involvedRefs(target, baseline []runFailure) []gotest.Reference {
	seen := map[string]bool{}
	var refs []gotest.Reference
	for _, set := range [][]runFailure{target, baseline} {
		for _, f := range set {
			k := f.ref.String(false)
			if seen[k] {
				continue
			}
			seen[k] = true
			refs = append(refs, f.ref)
		}
	}
	return refs
}

// terminalRefs returns the deduped function/subtest references that reached a terminal
// (pass/fail/skip) action in the run (packages excluded). Callers derive both the string lookup
// (terminalRefSet, to tell "target passed" from "target never ran") and the package intersection
// for diff-derived targets from it.
func terminalRefs(mgr *test.Manager, runID uuid.UUID) ([]gotest.Reference, error) {
	events, err := mgr.GetTestEvents(runID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var refs []gotest.Reference
	for i := range events {
		e := events[i]
		switch e.Action {
		case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
			if e.Reference.IsPackage() {
				continue
			}
			k := e.Reference.String(false)
			if seen[k] {
				continue
			}
			seen[k] = true
			refs = append(refs, e.Reference)
		}
	}
	return refs, nil
}

// terminalRefSet keys terminal refs by both raw and cleaned forms so a --target given in either
// form matches.
func terminalRefSet(refs []gotest.Reference) map[string]bool {
	out := map[string]bool{}
	for _, r := range refs {
		out[r.String(false)] = true
		out[r.String(true)] = true
	}
	return out
}

// refFails reports whether the named reference (matched against both raw and cleaned forms) has a
// failure in the given set.
func refFails(ref string, failures []runFailure) bool {
	for _, f := range failures {
		if f.ref.String(false) == ref || f.ref.String(true) == ref {
			return true
		}
	}
	return false
}

func fpKey(ref gotest.Reference, fingerprint string) string {
	return ref.String(false) + "\x00" + fingerprint
}

func sortFailures(fs []runFailure) {
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].ref.String(false) < fs[j].ref.String(false) })
}

func sortRefs(refs []gotest.Reference) {
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].String(false) < refs[j].String(false) })
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
