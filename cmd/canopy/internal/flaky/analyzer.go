package flaky

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// Store defines the interface for retrieving historical test data.
// This is satisfied by the test.Manager type.
type Store interface {
	// ListSessions returns all stored test sessions.
	ListSessions() ([]test.SessionInfo, error)
	// GetTestEvents retrieves all events for a test run.
	GetTestEvents(runID uuid.UUID) ([]gotest.Event, error)
	// GetFailuresByRun retrieves all failure data for a specific test run.
	GetFailuresByRun(runID uuid.UUID) ([]db.FailedTestDetails, error)
}

// Analyzer performs flakiness analysis on historical test data.
type Analyzer struct {
	store  Store
	config Config
}

// NewAnalyzer creates a new flaky test analyzer.
func NewAnalyzer(store Store, cfg Config) *Analyzer {
	return &Analyzer{
		store:  store,
		config: cfg,
	}
}

// AnalyzeAll analyzes all tests in the store and returns flakiness data for tests that meet the criteria.
func (a *Analyzer) AnalyzeAll() ([]Analysis, error) {
	outcomes, err := a.collectOutcomes()
	if err != nil {
		return nil, err
	}

	var results []Analysis
	for ref, outs := range outcomes {
		analysis := a.analyze(ref, outs)

		// apply filters
		if analysis.TotalRuns < a.config.MinRuns {
			continue
		}
		// only include tests that are actually flaky (have both passes and fails)
		if !analysis.IsFlaky() {
			continue
		}
		if analysis.Score < a.config.Threshold {
			continue
		}

		results = append(results, analysis)
	}

	// sort by flaky score descending, then by reference string for stability
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Reference.String(false) < results[j].Reference.String(false)
	})

	return results, nil
}

// Analyze analyzes a specific test reference for flakiness.
func (a *Analyzer) Analyze(ref gotest.Reference) (*Analysis, error) {
	outcomes, err := a.collectOutcomes()
	if err != nil {
		return nil, err
	}

	outs, ok := outcomes[ref]
	if !ok {
		return nil, nil
	}

	analysis := a.analyze(ref, outs)
	return &analysis, nil
}

// collectOutcomes gathers all terminal test outcomes from the store, filtered by config options.
func (a *Analyzer) collectOutcomes() (map[gotest.Reference][]Outcome, error) {
	sessions, err := a.store.ListSessions()
	if err != nil {
		return nil, err
	}

	// filter sessions if specific ones are requested
	if len(a.config.SessionIDs) > 0 {
		sessions = a.filterSessions(sessions)
	}

	outcomes := make(map[gotest.Reference][]Outcome)
	cutoff := a.getWindowCutoff()

	for i := range sessions {
		if err := a.collectSessionOutcomes(&sessions[i], cutoff, outcomes); err != nil {
			return nil, err
		}
	}

	return outcomes, nil
}

// getWindowCutoff returns the cutoff time for the analysis window, or zero time if no window.
func (a *Analyzer) getWindowCutoff() time.Time {
	if a.config.Window > 0 {
		return time.Now().Add(-a.config.Window)
	}
	return time.Time{}
}

// collectSessionOutcomes collects outcomes from a single session into the outcomes map.
func (a *Analyzer) collectSessionOutcomes(session *test.SessionInfo, cutoff time.Time, outcomes map[gotest.Reference][]Outcome) error {
	for j := range session.Runs {
		run := session.Runs[j]
		// skip runs outside the time window
		if !cutoff.IsZero() && run.Started.Before(cutoff) {
			continue
		}

		events, err := a.store.GetTestEvents(run.UUID)
		if err != nil {
			return err
		}

		// get failure data for this run to enrich outcomes
		failures, err := a.store.GetFailuresByRun(run.UUID)
		if err != nil {
			return err
		}

		a.collectEventOutcomes(run.UUID, events, failures, outcomes)
	}
	return nil
}

// collectEventOutcomes filters and collects outcomes from events into the outcomes map.
//
// Failure rows are written one-per-fail-event in event order during ingestion (see
// db.Store.addFailureData), and both events and failures are returned in that same
// insertion order, so we correlate them positionally: the n-th fail event maps to the
// n-th failure row.
//
// ponytail: positional correlation. the ceiling is that it assumes exactly one failure
// row per fail event. a fail event with empty aggregated output writes no row (db.go's
// `if aggregatedOutput != ""` guard), which would shift every later fingerprint by one.
// in practice every real fail event has output, so this holds. upgrade path if it ever
// stops holding: thread the DB event Index onto gotest.Event and join failures by Index.
func (a *Analyzer) collectEventOutcomes(runID uuid.UUID, events []gotest.Event, failures []db.FailedTestDetails, outcomes map[gotest.Reference][]Outcome) {
	failIdx := 0
	for k := range events {
		event := events[k]

		// advance the failure cursor on EVERY fail event, not just analyzer-included ones:
		// the store writes a row for all fail events (including package-level and
		// pattern-filtered ones), so the cursor must track the full failures slice to stay
		// aligned.
		var fail *db.FailedTestDetails
		if event.Action == gotest.FailAction {
			if failIdx < len(failures) {
				fail = &failures[failIdx]
			}
			failIdx++
		}

		if !a.shouldIncludeEvent(&event) {
			continue
		}

		outcome := Outcome{
			Action: event.Action,
			Time:   event.Time,
			RunID:  runID,
		}
		if fail != nil {
			outcome.Failure = &FailureInfo{
				Fingerprint: fail.Fingerprint,
				Type:        failure.Type(fail.Type),
			}
		}

		outcomes[event.Reference] = append(outcomes[event.Reference], outcome)
	}
}

// shouldIncludeEvent returns true if the event should be included in the analysis.
func (a *Analyzer) shouldIncludeEvent(event *gotest.Event) bool {
	// only consider terminal actions (pass, fail, skip)
	if !event.Action.Completed() {
		return false
	}

	// skip package-level references (we want individual tests)
	if event.Reference.IsPackage() {
		return false
	}

	// apply package pattern filtering
	if !a.matchesPackagePatterns(event.Reference.Package) {
		return false
	}

	// apply test pattern filtering
	if !a.matchesTestPattern(event.Reference.TestName(false)) {
		return false
	}

	return true
}

// filterSessions returns only sessions matching the configured SessionIDs.
func (a *Analyzer) filterSessions(sessions []test.SessionInfo) []test.SessionInfo {
	sessionSet := make(map[uuid.UUID]struct{})
	for _, id := range a.config.SessionIDs {
		sessionSet[id] = struct{}{}
	}

	var filtered []test.SessionInfo
	for i := range sessions {
		if _, ok := sessionSet[sessions[i].UUID]; ok {
			filtered = append(filtered, sessions[i])
		}
	}
	return filtered
}

// matchesPackagePatterns returns true if the package matches the configured patterns.
// If no patterns are configured, all packages match.
func (a *Analyzer) matchesPackagePatterns(pkg string) bool {
	// check exclude patterns first
	for _, pattern := range a.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, pkg); matched {
			return false
		}
		// also try matching against the pattern as a prefix for ** style patterns
		if matchGlobPrefix(pattern, pkg) {
			return false
		}
	}

	// if no include patterns specified, include everything (that wasn't excluded)
	if len(a.config.PackagePatterns) == 0 {
		return true
	}

	// check include patterns
	for _, pattern := range a.config.PackagePatterns {
		if matched, _ := filepath.Match(pattern, pkg); matched {
			return true
		}
		// also try prefix/suffix matching for patterns like "github.com/foo/..."
		if matchGlobPrefix(pattern, pkg) {
			return true
		}
	}

	return false
}

// matchGlobPrefix handles Go-style "..." patterns and more flexible matching.
func matchGlobPrefix(pattern, pkg string) bool {
	// handle trailing /... pattern (common in Go). "foo/..." matches "foo" and "foo/bar"
	// but NOT "foobar", so require an exact match or a "/" boundary after the prefix.
	if prefix, ok := strings.CutSuffix(pattern, "/..."); ok {
		return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
	}

	// handle ** patterns, e.g. "**/mocks" or "**/internal/*". filepath.Match is start-anchored,
	// so match the portion after **/ against every path suffix — that lets "**/mocks" match
	// "github.com/x/y/mocks" and "**/internal/*" match ".../internal/foo".
	if after, ok := strings.CutPrefix(pattern, "**"); ok {
		rest := strings.TrimPrefix(after, "/")
		if rest == "" {
			return true
		}
		segs := strings.Split(pkg, "/")
		for i := range segs {
			if matched, _ := filepath.Match(rest, strings.Join(segs[i:], "/")); matched {
				return true
			}
		}
	}

	return false
}

// matchesTestPattern returns true if the test name matches the configured regex.
// If no pattern is configured, all tests match.
func (a *Analyzer) matchesTestPattern(testName string) bool {
	if a.config.TestPattern == nil {
		return true
	}
	return a.config.TestPattern.MatchString(testName)
}

// analyze computes the flakiness analysis for a reference given its historical outcomes.
func (a *Analyzer) analyze(ref gotest.Reference, outcomes []Outcome) Analysis {
	analysis := Analysis{
		Reference: ref,
		TotalRuns: len(outcomes),
	}

	if len(outcomes) == 0 {
		return analysis
	}

	// sort outcomes by time for flip detection
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Time.Before(outcomes[j].Time)
	})

	analysis.FirstRun = RunInfo{ID: outcomes[0].RunID, Time: outcomes[0].Time}
	analysis.LastRun = RunInfo{ID: outcomes[len(outcomes)-1].RunID, Time: outcomes[len(outcomes)-1].Time}

	// process outcomes to count results, track failure modes, and detect flips
	failureModes := a.processOutcomes(&analysis, outcomes)

	// convert failure modes map to sorted slice
	analysis.FailureModes = sortedFailureModes(failureModes)

	// calculate flaky score
	analysis.Score = calculateFlakyScore(analysis.PassCount, analysis.FailCount)

	return analysis
}

// processOutcomes iterates through outcomes to count pass/fail/skip, track failure modes, and detect flips.
func (a *Analyzer) processOutcomes(analysis *Analysis, outcomes []Outcome) map[string]*FailureMode {
	failureModeMap := make(map[string]*FailureMode)
	var lastAction gotest.Action
	var lastOutcome *Outcome

	for i := range outcomes {
		out := &outcomes[i]
		fingerprint := outcomeFingerprint(out)

		a.countOutcome(analysis, out, fingerprint, failureModeMap)
		a.detectFlip(analysis, out, fingerprint, i, lastAction, lastOutcome)

		if out.Action != gotest.SkipAction {
			lastAction = out.Action
			lastOutcome = out
		}
	}

	return failureModeMap
}

// outcomeFingerprint returns the failure fingerprint for an outcome, or empty string if not a failure.
func outcomeFingerprint(out *Outcome) string {
	if out.Failure != nil {
		return out.Failure.Fingerprint
	}
	return ""
}

// countOutcome increments the appropriate counter and tracks failure modes.
func (a *Analyzer) countOutcome(analysis *Analysis, out *Outcome, fingerprint string, failureModeMap map[string]*FailureMode) {
	switch out.Action {
	case gotest.PassAction:
		analysis.PassCount++
	case gotest.FailAction:
		analysis.FailCount++
		trackFailureMode(out, fingerprint, failureModeMap)
	case gotest.SkipAction:
		analysis.SkipCount++
	}
}

// trackFailureMode adds a failure to the failure mode map.
func trackFailureMode(out *Outcome, fingerprint string, failureModeMap map[string]*FailureMode) {
	if fingerprint == "" {
		return
	}
	runInfo := RunInfo{ID: out.RunID, Time: out.Time}
	if mode, exists := failureModeMap[fingerprint]; exists {
		mode.Runs = append(mode.Runs, runInfo)
	} else {
		failureModeMap[fingerprint] = &FailureMode{
			Fingerprint: fingerprint,
			Type:        out.Failure.Type,
			Runs:        []RunInfo{runInfo},
		}
	}
}

// detectFlip checks if this outcome represents a state change and records notable runs.
func (a *Analyzer) detectFlip(analysis *Analysis, out *Outcome, fingerprint string, idx int, lastAction gotest.Action, lastOutcome *Outcome) {
	if out.Action == gotest.SkipAction {
		return
	}
	if idx == 0 || lastAction == "" || lastAction == out.Action {
		return
	}

	// this is a flip - record the before and after runs
	if len(analysis.NotableRuns) == 0 && lastOutcome != nil {
		analysis.NotableRuns = append(analysis.NotableRuns, NotableRun{
			ID:          lastOutcome.RunID,
			Time:        lastOutcome.Time,
			State:       lastOutcome.Action,
			Fingerprint: outcomeFingerprint(lastOutcome),
		})
	}

	analysis.NotableRuns = append(analysis.NotableRuns, NotableRun{
		ID:          out.RunID,
		Time:        out.Time,
		State:       out.Action,
		Fingerprint: fingerprint,
	})
	analysis.LastFlip = &RunInfo{ID: out.RunID, Time: out.Time}
}

// sortedFailureModes converts a failure mode map to a slice sorted by count descending.
func sortedFailureModes(failureModeMap map[string]*FailureMode) []FailureMode {
	modes := make([]FailureMode, 0, len(failureModeMap))
	for _, mode := range failureModeMap {
		modes = append(modes, *mode)
	}
	sort.Slice(modes, func(i, j int) bool {
		if modes[i].Count() != modes[j].Count() {
			return modes[i].Count() > modes[j].Count()
		}
		return modes[i].Fingerprint < modes[j].Fingerprint
	})
	return modes
}

// calculateFlakyScore computes a score from 0 to 1 representing how flaky a test is.
// 0 = completely stable (always passes or always fails)
// 1 = maximally flaky (50% pass rate)
//
// The formula is based on the entropy of the pass/fail distribution.
// A test that always passes or always fails has score 0.
// A test that passes 50% and fails 50% has score 1.
func calculateFlakyScore(passes, fails int) float64 {
	total := passes + fails
	if total == 0 {
		return 0
	}

	// if all passes or all fails, not flaky
	if passes == 0 || fails == 0 {
		return 0
	}

	// use a simple formula: 4 * p * (1-p) where p is pass rate
	// this gives 0 when p=0 or p=1, and 1 when p=0.5
	p := float64(passes) / float64(total)
	return 4 * p * (1 - p)
}

// CalculateFlakyScoreFromRate computes the flaky score from a pass rate (0.0-1.0).
// Exported for use in other packages.
func CalculateFlakyScoreFromRate(passRate float64) float64 {
	// clamp to valid range
	p := math.Max(0, math.Min(1, passRate))
	return 4 * p * (1 - p)
}
