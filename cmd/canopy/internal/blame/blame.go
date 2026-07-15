// Package blame answers "since which commit, changing what" for a failing test by
// walking its per-run pass/fail history joined to each run's commit and detecting the
// good->bad transition. The transition detection here is pure (no DB or git IO) so it is
// unit-testable over synthetic histories; the triage command wires it to the real store
// and git, enriches the first-bad commit with the files it changed, and renders the
// annotation next to a pre-existing or flaky verdict.
package blame

import "time"

// Confidence expresses how tightly the recorded runs bracket the onset commit. CI does not
// run every commit, so a gap between the last-good and first-bad runs is the common, honest
// answer, not a bug.
type Confidence string

const (
	// ConfidenceExact means the first-bad commit is the immediate child of the last-good
	// commit, so the culprit is that one commit.
	ConfidenceExact Confidence = "exact"
	// ConfidenceRange means commits ran between the last-good and first-bad commits with no
	// test data, so the culprit is somewhere in that range rather than a single commit.
	ConfidenceRange Confidence = "range"
	// ConfidenceUnknown means the reference was never seen good before the failure (no prior
	// data), so the onset is only our earliest sighting, not a proven transition.
	ConfidenceUnknown Confidence = "unknown"
)

// RunPoint is one recorded run of a reference, joined to its commit. Passed distinguishes a
// pass from a fail; Fingerprint carries the failure fingerprint on a failing run (empty on a
// pass). Callers pass these in chronological order (oldest first).
type RunPoint struct {
	Commit      string
	Branch      string
	Time        time.Time
	Passed      bool
	Fingerprint string
}

// Since is the first-seen answer for a verdict: the commit where the failure fingerprint (or
// flakiness) first appeared, the last commit known good, and how confident the bracket is.
// ChangedFiles is left nil by the pure detection and filled by git enrichment in the caller.
type Since struct {
	Commit         string
	Branch         string
	When           time.Time
	LastGoodCommit string
	Confidence     Confidence
	ChangedFiles   []string
}

// CommitDistanceFunc returns the number of commits strictly between lastGood and firstBad in
// git history (0 when firstBad is the immediate child of lastGood), or a negative value when
// the distance cannot be determined. It is git-backed in production and stubbed in tests, so
// the pure detection stays IO-free.
type CommitDistanceFunc func(lastGood, firstBad string) int

// DetectPreExisting finds the good->bad onset for a specific failure fingerprint: the earliest
// run where that fingerprint appears, paired with the latest prior run where the reference was
// good with respect to it (passed, or failed with a different fingerprint so this one was
// absent). Returns nil when the fingerprint never appears in history (nothing to annotate).
func DetectPreExisting(history []RunPoint, fingerprint string, dist CommitDistanceFunc) *Since {
	firstBad := -1
	for i := range history {
		if !history[i].Passed && history[i].Fingerprint == fingerprint {
			firstBad = i
			break
		}
	}
	if firstBad < 0 {
		return nil
	}
	// walk back for the latest run good w.r.t. this fingerprint. a run that failed with a
	// different fingerprint means this fingerprint was absent, which still bounds the onset.
	lastGood := -1
	for i := firstBad - 1; i >= 0; i-- {
		if history[i].Passed || history[i].Fingerprint != fingerprint {
			lastGood = i
			break
		}
	}
	return newSince(history, firstBad, lastGood, dist)
}

// DetectFlaky finds when a reference first began to flake: the first failure that follows a
// prior pass (the first pass->fail flip). Returns nil when no failure follows a pass (all
// fails, or the history never flipped), which reads as "no clean onset in history".
func DetectFlaky(history []RunPoint, dist CommitDistanceFunc) *Since {
	lastPass := -1
	for i := range history {
		if history[i].Passed {
			lastPass = i
			continue
		}
		// a failure after we have seen a pass is the onset of flapping; the pass just before
		// it is the last-good bracket.
		if lastPass >= 0 {
			return newSince(history, i, lastPass, dist)
		}
	}
	return nil
}

// newSince assembles a Since from the first-bad and last-good indices, classifying confidence
// from the commit distance. lastGood < 0 means the reference was never seen good beforehand.
func newSince(history []RunPoint, firstBad, lastGood int, dist CommitDistanceFunc) *Since {
	bad := history[firstBad]
	s := &Since{
		Commit:     bad.Commit,
		Branch:     bad.Branch,
		When:       bad.Time,
		Confidence: ConfidenceUnknown,
	}
	if lastGood < 0 {
		return s
	}
	good := history[lastGood]
	s.LastGoodCommit = good.Commit
	s.Confidence = classify(good.Commit, bad.Commit, dist)
	return s
}

// classify turns a commit distance into a confidence. an unknown distance degrades to range
// rather than claiming a single culprit, keeping the annotation honest about CI gaps.
func classify(good, bad string, dist CommitDistanceFunc) Confidence {
	if good == bad {
		// the reference passed and failed on the SAME commit: environmental flakiness on
		// unchanged code, so no commit is the culprit. don't assert an exact one.
		return ConfidenceRange
	}
	if dist == nil {
		return ConfidenceRange
	}
	switch d := dist(good, bad); {
	case d < 0:
		return ConfidenceRange
	case d == 0:
		return ConfidenceExact
	default:
		return ConfidenceRange
	}
}
