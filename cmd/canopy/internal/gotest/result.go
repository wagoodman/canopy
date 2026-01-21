package gotest

import (
	"io"
	"sync"
	"time"

	"github.com/lindell/go-ordered-set/orderedset"
)

// ResultConfig controls what events and output the Result will track and store.
type ResultConfig struct {
	TrackOtherOutput   bool
	TrackFailingOutput bool
}

// Result aggregates test execution events into queryable state with thread-safe access.
// It maintains multiple indices optimized for different access patterns and provides
// real-time statistics as tests execute.
type Result struct {
	lock   *sync.RWMutex
	config ResultConfig

	references *orderedset.OrderedSet[Reference]
	packages   *orderedset.OrderedSet[Reference]
	children   map[Reference]*orderedset.OrderedSet[Reference]
	// testEventsByReference *orderedmap.OrderedMap[Reference, []Event]   // all action types except "output"
	testEventsByReference  map[Reference][]Event                        // all action types // TODO rethink this
	testOutputByReference  map[Reference][]Event                        // only "output" action // TODO rethink this
	referencesByAction     map[Action]*orderedset.OrderedSet[Reference] // all action types except "output"
	testReferencesByAction map[Action]*orderedset.OrderedSet[Reference]
	conclusionEvent        map[Reference]Event
	start                  time.Time
	startOffset            time.Duration
	lastEventTime          time.Time
	totalElapsed           time.Duration

	coverage *float64
}

// ResultStats provides counts of tests in each execution state for quick status reporting.
type ResultStats struct {
	Passed              int
	Failed              int
	Skipped             int
	Running             int
	PackagesWithNoTests int
}

func (s ResultStats) Total() int {
	return s.Passed + s.Failed + s.Skipped
}

func (s *ResultStats) Merge(others ...ResultStats) {
	for _, other := range others {
		s.Passed += other.Passed
		s.Failed += other.Failed
		s.Skipped += other.Skipped
		s.Running += other.Running
		s.PackagesWithNoTests += other.PackagesWithNoTests
	}
}

// NewResult creates a new Result aggregator with the specified configuration.
// Initializes all internal data structures and indices for efficient querying.
func NewResult(config ResultConfig) *Result {
	referencesByAction := make(map[Action]*orderedset.OrderedSet[Reference])
	testReferencesByAction := make(map[Action]*orderedset.OrderedSet[Reference])
	for _, action := range []Action{RunAction, PassAction, FailAction, SkipAction} {
		referencesByAction[action] = orderedset.New[Reference]()
		testReferencesByAction[action] = orderedset.New[Reference]()
	}
	return &Result{
		lock:   &sync.RWMutex{},
		config: config,

		//testEventsByReference:  orderedmap.NewOrderedMap[Reference, []Event](),
		references:             orderedset.New[Reference](),
		packages:               orderedset.New[Reference](),
		children:               make(map[Reference]*orderedset.OrderedSet[Reference]),
		testEventsByReference:  make(map[Reference][]Event),
		testOutputByReference:  make(map[Reference][]Event),
		referencesByAction:     referencesByAction,
		testReferencesByAction: testReferencesByAction,
		conclusionEvent:        make(map[Reference]Event),
	}
}

func (r *Result) ReferenceElapsed(ref Reference, live bool) time.Duration {
	r.lock.RLock()
	defer r.lock.RUnlock()

	events := r.testEventsByReference[ref]
	if len(events) == 0 {
		return 0
	}

	if r.lastEventTime.IsZero() {
		return 0
	}

	start := events[0].Time
	end := events[len(events)-1].Time
	if len(events) == 1 {
		if live {
			end = time.Now()
		} else {
			end = time.Now().Add(-r.startOffset)
		}
	}

	return end.Sub(start)
}

func (r *Result) Elapsed(live bool) time.Duration {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.lastEventTime.IsZero() {
		return 0
	}

	if !live {
		return r.lastEventTime.Sub(r.start)
	}

	// we want to use the timestamps as the source of truth for calculating elapsed, however, we also need to ensure
	// this is always relative to time.Now() so that subsequent calls will result in updated (non-static) values.
	// Note: don't use r.totalElapsed as input into determining this since we don't know if there is a single
	// track of tests being run or if t.Parallel() is being used.
	return time.Now().Add(-r.startOffset).Sub(r.start)
}

// Update processes a new event and updates all internal indices and state.
// This method is thread-safe and maintains the hierarchical structure of test results.
// Called for every event received during test execution.
func (r *Result) Update(e Event) {
	// TODO: check for e.Error and report to the UI when found...

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.start.IsZero() {
		r.start = e.Time
		r.startOffset = time.Since(e.Time)
	}
	if !r.lastEventTime.IsZero() {
		r.totalElapsed += e.Time.Sub(r.lastEventTime)
	}
	r.lastEventTime = e.Time

	// keep track of children for each reference to be able to walk a tree of tests
	parentRef := e.Reference.ParentRef()
	if parentRef != nil {
		parent := *parentRef
		if _, ok := r.children[parent]; !ok {
			r.children[parent] = orderedset.New[Reference]()
		}
		// if !e.Reference.IsPackage() {
		r.children[parent].Add(e.Reference)
		//}
	}

	// process output and events
	switch e.Action {
	case OutputAction:
		// keep track of new test output
		r.testOutputByReference[e.Reference] = append(r.testOutputByReference[e.Reference], e)

	case PassAction, SkipAction:
		// clear test output for passed/skipped tests
		if !r.config.TrackOtherOutput {
			r.testOutputByReference[e.Reference] = nil
		}

	default:
		// keep track of all other test events
		// existing, _ := r.testEventsByReference.Get(e.Reference)
		// r.testEventsByReference.Set(e.Reference, append(existing, e))
	}

	// all events
	r.testEventsByReference[e.Reference] = append(r.testEventsByReference[e.Reference], e)

	r.references.Add(e.Reference)
	if e.Reference.IsPackage() {
		r.packages.Add(e.Reference)
	}

	// process conclusion
	switch e.Action {
	case PassAction, SkipAction, FailAction:

		r.conclusionEvent[e.Reference] = e
		r.referencesByAction[RunAction].Delete(e.Reference)
		r.testReferencesByAction[RunAction].Delete(e.Reference)
	}

	// keep track of test results (actions) for each test reference
	if e.Action != OutputAction {
		if _, ok := r.referencesByAction[e.Action]; !ok {
			r.referencesByAction[e.Action] = orderedset.New[Reference]()
		}
		r.referencesByAction[e.Action].Add(e.Reference)

		if _, ok := r.testReferencesByAction[e.Action]; !ok {
			r.testReferencesByAction[e.Action] = orderedset.New[Reference]()
		}
		if !e.Reference.IsPackage() {
			r.testReferencesByAction[e.Action].Add(e.Reference)
		}
	}
}

// References returns all test references tracked by this result, optionally filtered.
// The references maintain their insertion order and include packages, functions, and subtests.
func (r Result) References(removeFilters ...func(Reference) bool) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if len(removeFilters) == 0 {
		return r.references.Values()
	}

	var values []Reference
all:
	for _, ref := range r.references.Values() {
		// apply filters to references
		for _, filter := range removeFilters {
			if filter(ref) {
				continue all
			}
		}

		values = append(values, ref)
	}

	return values
}

// Packages returns all package-level references that have been tracked.
func (r Result) Packages() []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.packages.Values()
}

func (r Result) Children(ref Reference) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if children, ok := r.children[ref]; ok {
		return children.Values()
	}
	return nil
}

func (r Result) ReferenceEvents(ref Reference) []Event {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.testEventsByReference[ref]
}

// ReferencesByAction returns all references (packages, functions, subtests) that
// have reached the specified action state (run, pass, fail, skip).
func (r Result) ReferencesByAction(action Action) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if refs, ok := r.referencesByAction[action]; ok {
		return refs.Values()
	}
	return nil
}

// TestReferencesByAction returns only function and subtest references (excluding packages)
// that have reached the specified action state.
func (r Result) TestReferencesByAction(action Action) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if refs, ok := r.testReferencesByAction[action]; ok {
		return refs.Values()
	}
	return nil
}

// ReferenceTestStats returns the test events for all children of the given reference (recursively),
// and optionally the given reference itself.
func (r Result) ReferenceTestStats(ref Reference, inclusive bool) ResultStats {
	r.lock.RLock()
	defer r.lock.RUnlock()

	stats := ResultStats{}

	if inclusive {
		action := r.ReferenceConclusiveAction(ref)
		switch action {
		case PassAction:
			stats.Passed++
		case FailAction:
			stats.Failed++
		case SkipAction:
			stats.Skipped++
		case RunAction:
			stats.Running++
		}
	}

	for _, childRef := range r.Children(ref) {
		childStats := r.ReferenceTestStats(childRef, true)
		stats.Passed += childStats.Passed
		stats.Failed += childStats.Failed
		stats.Skipped += childStats.Skipped
		stats.Running += childStats.Running
	}

	return stats
}

func (r Result) TestStats() ResultStats {
	r.lock.RLock()
	defer r.lock.RUnlock()

	// count packages with no tests
	packagesWithNoTests := 0
	for _, pkg := range r.packages.Values() {
		events := r.testEventsByReference[pkg]
		for _, e := range events {
			if e.HasAnnotation(NoTestFiles, NoTestsToRun) {
				packagesWithNoTests++
				break
			}
		}
	}

	return ResultStats{
		Passed:              r.testReferencesByAction[PassAction].Size(),
		Failed:              r.testReferencesByAction[FailAction].Size(),
		Skipped:             r.testReferencesByAction[SkipAction].Size(),
		Running:             r.testReferencesByAction[RunAction].Size(),
		PackagesWithNoTests: packagesWithNoTests,
	}
}

func (r Result) ReferenceOutput(ref Reference, writer io.Writer) error {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, e := range r.testOutputByReference[ref] {
		_, err := writer.Write([]byte(e.Output))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Result) ReferenceConclusiveAction(ref Reference) Action {
	r.lock.RLock()
	defer r.lock.RUnlock()

	e, ok := r.conclusionEvent[ref]
	if !ok {
		return ""
	}
	return e.Action
}

func (r Result) ReferenceConclusion(ref Reference) *Event {
	r.lock.RLock()
	defer r.lock.RUnlock()

	e, ok := r.conclusionEvent[ref]
	if !ok {
		return nil
	}
	return &e
}

func (r Result) ReferenceDuration(ref Reference) time.Duration {
	r.lock.RLock()
	defer r.lock.RUnlock()

	e1, ok := r.testEventsByReference[ref]
	if !ok || len(e1) == 0 {
		return 0
	}
	e2, ok := r.conclusionEvent[ref]
	if ok {
		return e2.Time.Sub(e1[0].Time)
	}

	// if no conclusion event, return the duration from the first event to the last event
	return e1[len(e1)-1].Time.Sub(e1[0].Time)
}

func (r *Result) SetCoverage(coverage *float64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if coverage == nil {
		r.coverage = nil
		return
	}
	cpy := *coverage

	r.coverage = &cpy
}

func (r Result) Coverage() (float64, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.coverage == nil {
		return 0, false
	}

	return *r.coverage, true
}

func (r Result) Passed() (bool, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	runningTestRefs := r.testReferencesByAction[RunAction]
	passedTestRefs := r.testReferencesByAction[PassAction]
	failedTestRefs := r.testReferencesByAction[FailAction]
	skippedTestRefs := r.testReferencesByAction[SkipAction]
	hasMirroredRefs := len(r.conclusionEvent) == r.references.Size()
	isStarting := (refCount(passedTestRefs) + refCount(failedTestRefs) + refCount(skippedTestRefs)) == 0
	isStillRunning := hasRefs(runningTestRefs) || isStarting || (!hasMirroredRefs && r.references.Size() > 0)
	passed := refCount(failedTestRefs) == 0

	if isStarting {
		// we may be starting... or there will be no test refs (only package refs) since there is a
		// compilation error or some such. No tests now doesn't mean we should expect tests later.
		failedTestRefs = r.referencesByAction[FailAction]

		isStillRunning = true
		passed = refCount(failedTestRefs) == 0
		if r.references.Size() == 0 {
			passed = false
		}
	}

	return passed, isStillRunning
}

func hasRefs(set *orderedset.OrderedSet[Reference]) bool {
	return set != nil && set.Size() > 0
}

func refCount(set *orderedset.OrderedSet[Reference]) int {
	if set == nil {
		return 0
	}
	return set.Size()
}
