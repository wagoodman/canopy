package commands

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/blame"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// sinceJSON is the since/first-seen enrichment attached to a pre-existing or flaky verdict:
// the commit where the failure fingerprint (or flakiness) first appeared, the last commit
// known good, the files that commit changed, and how confident the bracket is.
type sinceJSON struct {
	Verdict      Verdict  `json:"verdict"`
	Commit       string   `json:"commit"`
	LastGood     string   `json:"last_good,omitempty"`
	ChangedFiles []string `json:"changed_files,omitempty"`
	Confidence   string   `json:"confidence"`
}

// computeSinceByRef derives the since annotation for each failing reference whose verdict is
// pre-existing or flaky. it reuses the flaky analyzer's history walk (per-run outcomes) joined
// to each run's recorded commit, runs the pure transition detection, then enriches the
// first-bad commit with the files it changed. returns nil (no annotation anywhere) when there
// is no persistence or no supporting history, mirroring how localization is absent without a
// diff. all errors are logged and swallowed: this is best-effort enrichment, never a reason to
// fail the report.
func computeSinceByRef(mgr *test.Manager, analyzer *flaky.Analyzer, selected []runFailure, verdictByRef map[string]Verdict) map[string]*sinceJSON { //nolint:funlen
	store := mgr.DBStore()
	if store == nil {
		return nil // no persistence, nothing to bisect
	}

	var refs []gotest.Reference
	for _, f := range selected {
		if annotatable(verdictByRef[f.ref.String(false)]) {
			refs = append(refs, f.ref)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	log.WithFields("references", len(refs)).Debug("resolving since/first-seen for pre-existing and flaky failures")

	outcomesByRef, err := analyzer.OutcomesForRefs(refs)
	if err != nil {
		log.WithFields("error", err).Debug("since annotation skipped: unable to collect outcomes")
		return nil
	}

	commitCache := map[uuid.UUID]*db.SourceState{}
	// git-backed distance; swallow errors to -1 so an unknowable distance reads as a range.
	dist := func(good, bad string) int {
		d, err := source.CommitsBetween(".", good, bad)
		if err != nil {
			return -1
		}
		return d
	}

	out := map[string]*sinceJSON{}
	for _, f := range selected {
		refStr := f.ref.String(false)
		verdict := verdictByRef[refStr]
		if !annotatable(verdict) {
			continue
		}
		history := buildBlameHistory(store, outcomesByRef[f.ref], commitCache)
		if len(history) == 0 {
			log.WithFields("reference", refStr).Debug("no commit-joined history for failure; skipping since annotation")
			continue
		}

		var since *blame.Since
		switch verdict {
		case VerdictPreExisting:
			since = blame.DetectPreExisting(history, f.detail.Fingerprint, dist)
		case VerdictFlaky:
			since = blame.DetectFlaky(history, dist)
		}
		if since == nil {
			log.WithFields("reference", refStr, "verdict", verdict).Debug("no good->bad transition in history; no since annotation")
			continue
		}

		// pre-existing names the changed files (likely culprits); flaky is just "since <commit>".
		if verdict == VerdictPreExisting {
			if files, err := source.FilesChangedInCommit(".", since.Commit); err == nil {
				since.ChangedFiles = files
			}
		}

		out[refStr] = &sinceJSON{
			Verdict:      verdict,
			Commit:       since.Commit,
			LastGood:     since.LastGoodCommit,
			ChangedFiles: since.ChangedFiles,
			Confidence:   string(since.Confidence),
		}
		log.WithFields("reference", refStr, "verdict", verdict, "commit", shortCommit(since.Commit), "confidence", since.Confidence, "changed-files", len(since.ChangedFiles)).Debug("resolved first-seen commit for failure")
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// annotatable reports whether a verdict gets a since/first-seen annotation. only pre-existing
// and flaky do; a new-regression is by definition new to this run, so there is nothing to bisect.
func annotatable(v Verdict) bool {
	return v == VerdictPreExisting || v == VerdictFlaky
}

// buildBlameHistory turns a reference's per-run outcomes into commit-joined RunPoints in
// chronological order, dropping runs with no recorded commit (those cannot be located). source
// states are cached by run so a shared run is not re-queried per reference.
func buildBlameHistory(store *db.Store, outcomes []flaky.Outcome, cache map[uuid.UUID]*db.SourceState) []blame.RunPoint {
	var history []blame.RunPoint
	for i := range outcomes {
		o := outcomes[i]
		ss, ok := cache[o.RunID]
		if !ok {
			ss, _ = store.GetSourceState(o.RunID) // best-effort; nil when absent
			cache[o.RunID] = ss
		}
		if ss == nil || ss.Commit == "" {
			continue
		}
		rp := blame.RunPoint{
			Commit: ss.Commit,
			Branch: ss.Branch,
			Time:   o.Time,
			Passed: o.Action == gotest.PassAction,
		}
		if o.Failure != nil {
			rp.Fingerprint = o.Failure.Fingerprint
		}
		history = append(history, rp)
	}
	return history
}

// sinceForGroup returns the since annotation for a symptom group: the first member reference
// that has one. a cluster usually shares one fingerprint, so the first hit represents the group.
func sinceForGroup(refs []string, sinceByRef map[string]*sinceJSON) *sinceJSON {
	for _, r := range refs {
		if s, ok := sinceByRef[r]; ok {
			return s
		}
	}
	return nil
}
