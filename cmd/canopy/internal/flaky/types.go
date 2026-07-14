// Package flaky provides analysis of test flakiness based on historical test results.
// It identifies tests that intermittently pass or fail by examining patterns in test outcomes
// stored in the SQLite database.
package flaky

import (
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
)

// Analysis contains the flakiness analysis for a single test reference.
type Analysis struct {
	// Reference identifies the test being analyzed.
	Reference gotest.Reference
	// TotalRuns is the number of times this test was executed in the analysis window.
	TotalRuns int
	// PassCount is the number of times this test passed.
	PassCount int
	// FailCount is the number of times this test failed.
	FailCount int
	// SkipCount is the number of times this test was skipped.
	SkipCount int
	// Score is a value from 0 to 1 where 0 = always stable (always passes or always fails),
	// and 1 = maximally flaky (50% pass rate).
	Score float64
	// LastFlip is the run where the test outcome most recently changed from pass to fail or vice versa.
	LastFlip *RunInfo
	// FailureModes represents distinct ways this test has failed, grouped by fingerprint.
	FailureModes []FailureMode
	// FirstRun is the first run in the analysis window.
	FirstRun RunInfo
	// LastRun is the most recent run.
	LastRun RunInfo
	// NotableRuns contains runs at flip points (state transitions) in chronological order.
	// Each entry represents a run where the outcome changed from the previous run.
	// The first entry is the "before" state, subsequent entries show transitions.
	NotableRuns []NotableRun
	// Sequence is every run outcome in chronological order (oldest first). Unlike the
	// counts and NotableRuns, this preserves the full timeline so callers can render a
	// pass/fail trend of what's actually been happening.
	Sequence []gotest.Action
}

// FailureMode represents a distinct way a test can fail.
// Failures are grouped by fingerprint to identify different failure symptoms.
type FailureMode struct {
	// Fingerprint is a semantic hash identifying this failure mode.
	Fingerprint string
	// Type is the failure category (assertion, panic, diff, timeout, unknown).
	Type failure.Type
	// Runs contains all runs where this failure mode occurred.
	Runs []RunInfo
	// Summary is a human-readable description of the failure.
	Summary string
}

// Count returns how many times this failure mode occurred.
func (f FailureMode) Count() int {
	return len(f.Runs)
}

// LastSeen returns the most recent occurrence of this failure mode.
func (f FailureMode) LastSeen() time.Time {
	if len(f.Runs) == 0 {
		return time.Time{}
	}
	latest := f.Runs[0].Time
	for _, r := range f.Runs[1:] {
		if r.Time.After(latest) {
			latest = r.Time
		}
	}
	return latest
}

// RunInfo represents a specific test run occurrence.
type RunInfo struct {
	// ID is the UUID of the test run.
	ID uuid.UUID
	// Time is when this run occurred.
	Time time.Time
}

// NotableRun represents a run at a flip point (state transition).
type NotableRun struct {
	// ID is the UUID of the test run.
	ID uuid.UUID
	// Time is when this run occurred.
	Time time.Time
	// State is the outcome of this run (pass, fail, skip).
	State gotest.Action
	// Fingerprint is the failure fingerprint if this is a failure run.
	Fingerprint string
}

// IsFlaky returns true if the test has exhibited flaky behavior (both passes and failures).
func (a Analysis) IsFlaky() bool {
	return a.PassCount > 0 && a.FailCount > 0
}

// PassRate returns the percentage of runs that passed (0.0-1.0).
func (a Analysis) PassRate() float64 {
	total := a.PassCount + a.FailCount
	if total == 0 {
		return 0
	}
	return float64(a.PassCount) / float64(total)
}

// FailRate returns the percentage of runs that failed (0.0-1.0).
func (a Analysis) FailRate() float64 {
	total := a.PassCount + a.FailCount
	if total == 0 {
		return 0
	}
	return float64(a.FailCount) / float64(total)
}

// HasDistinctFailures returns true if the test has failed with multiple different outputs,
// suggesting different failure modes or causes.
func (a Analysis) HasDistinctFailures() bool {
	return len(a.FailureModes) > 1
}

// FailureHashes returns the fingerprints of all distinct failure modes.
// Provided for backward compatibility.
func (a Analysis) FailureHashes() []string {
	hashes := make([]string, len(a.FailureModes))
	for i, m := range a.FailureModes {
		hashes[i] = m.Fingerprint
	}
	return hashes
}

// Outcome represents a single test run outcome for historical analysis.
type Outcome struct {
	// Action is the terminal action (pass, fail, skip).
	Action gotest.Action
	// Time is when this outcome was recorded.
	Time time.Time
	// RunID is the UUID of the test run this outcome came from.
	RunID uuid.UUID
	// Failure contains structured failure details if this is a failure outcome.
	// This comes from the FailedTestDetails table.
	Failure *FailureInfo
}

// FailureInfo contains failure metadata from the database.
type FailureInfo struct {
	// Fingerprint is the semantic hash of this failure.
	Fingerprint string
	// Type is the failure category.
	Type failure.Type
}

// Config configures the flaky analysis behavior.
type Config struct {
	// Window limits analysis to runs within this duration from now.
	// Zero means no time limit.
	Window time.Duration
	// MinRuns is the minimum number of runs required to consider a test for flakiness.
	// Tests with fewer runs are excluded from analysis.
	MinRuns int
	// Threshold is the minimum flaky score to report (0.0-1.0).
	// Tests with scores below this are not reported as flaky.
	Threshold float64

	// Scoping options

	// PackagePatterns are glob patterns to match against stored package paths.
	// Empty means include all packages.
	PackagePatterns []string
	// ExcludePatterns are glob patterns to exclude packages from analysis.
	ExcludePatterns []string
	// TestPattern is a regex to filter test function names.
	// Nil means include all tests.
	TestPattern *regexp.Regexp
	// SessionIDs limits analysis to specific sessions.
	// Empty means analyze all sessions.
	SessionIDs []uuid.UUID
}

// DefaultConfig returns a sensible default configuration for flaky analysis.
func DefaultConfig() Config {
	return Config{
		Window:    0, // no time limit
		MinRuns:   2, // need at least 2 runs to detect flakiness
		Threshold: 0, // report any flakiness
	}
}
