package presenter

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"io"
	"time"
)

var _ result = (*joinedResult)(nil)

type result interface {
	ReferenceElapsed(ref gotest.Reference, live bool) time.Duration
	Elapsed(live bool) time.Duration
	Update(e gotest.Event)
	References() []gotest.Reference
	Packages() []gotest.Reference
	Children(ref gotest.Reference) []gotest.Reference
	ReferenceEvents(ref gotest.Reference) []gotest.Event
	ReferencesByAction(action gotest.Action) []gotest.Reference
	TestReferencesByAction(action gotest.Action) []gotest.Reference
	ReferenceTestStats(ref gotest.Reference, inclusive bool) gotest.ResultStats
	TestStats() gotest.ResultStats
	ReferenceOutput(ref gotest.Reference, writer io.Writer) error
	ReferenceConclusiveAction(ref gotest.Reference) gotest.Action
	ReferenceConclusion(ref gotest.Reference) *gotest.Event
	ReferenceDuration(ref gotest.Reference) time.Duration
	SetCoverage(coverage *float64)
	Coverage() (float64, bool)
	Passed() (bool, bool)
}

type joinedResult struct {
	runs []gotest.Run
}

func newJoinedResults(runs ...gotest.Run) result {
	return &joinedResult{
		runs: runs,
	}
}

func (j joinedResult) ReferenceElapsed(ref gotest.Reference, live bool) time.Duration {
	// return maximum elapsed time for the reference across all runs
	var maxElapsed time.Duration
	for _, run := range j.runs {
		if elapsed := run.Result.ReferenceElapsed(ref, live); elapsed > maxElapsed {
			maxElapsed = elapsed
		}
	}
	return maxElapsed
}

func (j joinedResult) Elapsed(live bool) time.Duration {
	// return maximum elapsed time for the reference across all runs
	var maxElapsed time.Duration
	for _, run := range j.runs {
		if elapsed := run.Result.Elapsed(live); elapsed > maxElapsed {
			maxElapsed = elapsed
		}
	}
	return maxElapsed
}

func (j joinedResult) Update(e gotest.Event) {
	for i := range j.runs {
		j.runs[i].Result.Update(e)
	}
}

func (j joinedResult) References() []gotest.Reference {
	var refs []gotest.Reference
	for _, run := range j.runs {
		refs = append(refs, run.Result.References()...)
	}
	// TODO: deduplicate references?
	return refs
}

func (j joinedResult) Packages() []gotest.Reference {
	var refs []gotest.Reference
	for _, run := range j.runs {
		refs = append(refs, run.Result.Packages()...)
	}
	// TODO: deduplicate packages?
	return refs
}

func (j joinedResult) Children(ref gotest.Reference) []gotest.Reference {
	var children []gotest.Reference
	for _, run := range j.runs {
		childRefs := run.Result.Children(ref)
		if len(childRefs) > 0 {
			children = append(children, childRefs...)
		}
	}
	return children
}

func (j joinedResult) ReferenceEvents(ref gotest.Reference) []gotest.Event {
	var events []gotest.Event
	for _, run := range j.runs {
		runEvents := run.Result.ReferenceEvents(ref)
		if len(runEvents) > 0 {
			events = append(events, runEvents...)
		}
	}
	return events
}

func (j joinedResult) ReferencesByAction(action gotest.Action) []gotest.Reference {
	var refs []gotest.Reference
	for _, run := range j.runs {
		runRefs := run.Result.ReferencesByAction(action)
		if len(runRefs) > 0 {
			refs = append(refs, runRefs...)
		}
	}
	return refs
}

func (j joinedResult) TestReferencesByAction(action gotest.Action) []gotest.Reference {
	var refs []gotest.Reference
	for _, run := range j.runs {
		runRefs := run.Result.TestReferencesByAction(action)
		if len(runRefs) > 0 {
			refs = append(refs, runRefs...)
		}
	}
	return refs
}

func (j joinedResult) ReferenceTestStats(ref gotest.Reference, inclusive bool) gotest.ResultStats {
	var stats gotest.ResultStats
	for _, run := range j.runs {
		runStats := run.Result.ReferenceTestStats(ref, inclusive)
		stats.Merge(runStats)
	}
	return stats
}

func (j joinedResult) TestStats() gotest.ResultStats {
	var stats gotest.ResultStats
	for _, run := range j.runs {
		runStats := run.Result.TestStats()
		stats.Merge(runStats)
	}
	return stats
}

func (j joinedResult) ReferenceOutput(ref gotest.Reference, writer io.Writer) error {
	for _, run := range j.runs {
		if err := run.Result.ReferenceOutput(ref, writer); err != nil {
			return err
		}
	}
	return nil
}

func (j joinedResult) ReferenceConclusiveAction(ref gotest.Reference) gotest.Action {
	// return the first conclusive action found across all runs
	for _, run := range j.runs {
		action := run.Result.ReferenceConclusiveAction(ref)
		if action != gotest.UnknownAction {
			return action
		}
	}
	return gotest.UnknownAction
}

func (j joinedResult) ReferenceConclusion(ref gotest.Reference) *gotest.Event {
	for _, run := range j.runs {
		if event := run.Result.ReferenceConclusion(ref); event != nil {
			return event
		}
	}
	return nil
}

func (j joinedResult) ReferenceDuration(ref gotest.Reference) time.Duration {
	var maxDuration time.Duration
	for _, run := range j.runs {
		if duration := run.Result.ReferenceDuration(ref); duration > maxDuration {
			maxDuration = duration
		}
	}
	return maxDuration
}

func (j joinedResult) SetCoverage(_ *float64) {
	return // no-op for joined results, coverage is not possible
}

func (j joinedResult) Coverage() (float64, bool) {
	// coverage is not possible for joined results, return zero and false (as if it was never enabled)
	return 0, false
}

func (j joinedResult) Passed() (bool, bool) {
	var allPassed = true
	var isStillRunning = false
	for _, run := range j.runs {
		passed, stillRunning := run.Result.Passed()
		allPassed = allPassed && passed
		isStillRunning = isStillRunning || stillRunning
	}
	return allPassed, isStillRunning
}
