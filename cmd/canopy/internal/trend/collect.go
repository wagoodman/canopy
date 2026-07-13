package trend

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// Collect gathers terminal outcomes for every reference across all in-scope runs,
// keyed by reference. Both package-level and individual test references are returned;
// callers filter with Reference.IsPackage() as needed.
//
// This is the shared primitive behind the duration/failures/count analyzers so they
// apply identical scoping and cache semantics.
func Collect(store Store, scope Scope) (map[gotest.Reference][]Record, error) {
	sessions, err := store.ListSessions()
	if err != nil {
		return nil, err
	}

	if len(scope.SessionIDs) > 0 {
		sessions = filterSessions(sessions, scope.SessionIDs)
	}

	runs := selectRuns(sessions, scope.Window, scope.Last)

	records := make(map[gotest.Reference][]Record)
	for i := range runs {
		events, err := store.GetTestEvents(runs[i].UUID)
		if err != nil {
			return nil, err
		}
		collectRun(&scope, runs[i].UUID, events, records)
	}

	return records, nil
}

// selectRuns flattens all runs, drops those outside the window, and keeps the most
// recent `last` (0 = keep all).
func selectRuns(sessions []test.SessionInfo, window time.Duration, last int) []test.RunInfo {
	var cutoff time.Time
	if window > 0 {
		cutoff = time.Now().Add(-window)
	}

	var runs []test.RunInfo
	for i := range sessions {
		for j := range sessions[i].Runs {
			run := sessions[i].Runs[j]
			if !cutoff.IsZero() && run.Started.Before(cutoff) {
				continue
			}
			runs = append(runs, run)
		}
	}

	// most recent first, so a `last` cap keeps the newest runs
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Started.After(runs[j].Started)
	})

	if last > 0 && len(runs) > last {
		runs = runs[:last]
	}
	return runs
}

// collectRun appends in-scope terminal records from a single run's events.
func collectRun(scope *Scope, runID uuid.UUID, events []gotest.Event, records map[gotest.Reference][]Record) {
	for k := range events {
		event := events[k]

		// only terminal actions carry a meaningful outcome and elapsed time
		if !event.Action.Completed() {
			continue
		}

		ref := event.Reference
		isPkg := ref.IsPackage()

		if !scope.matchesPackage(ref.Package) {
			continue
		}
		// test-name filtering does not apply to package-level references
		if !isPkg && !scope.matchesTest(ref.TestName(false)) {
			continue
		}

		var elapsed float64
		if event.Elapsed != nil {
			elapsed = *event.Elapsed
		}

		records[ref] = append(records[ref], Record{
			RunID:   runID,
			Time:    event.Time,
			Action:  event.Action,
			Elapsed: elapsed,
			Cached:  event.HasAnnotation(gotest.Cached),
		})
	}
}

// filterSessions returns only sessions whose UUID is in the allowed set.
func filterSessions(sessions []test.SessionInfo, ids []uuid.UUID) []test.SessionInfo {
	allowed := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}

	var out []test.SessionInfo
	for i := range sessions {
		if _, ok := allowed[sessions[i].UUID]; ok {
			out = append(out, sessions[i])
		}
	}
	return out
}
