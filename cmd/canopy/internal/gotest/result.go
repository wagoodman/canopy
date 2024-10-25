package gotest

import (
	"strings"
	"sync"
	"time"

	"github.com/lindell/go-ordered-set/orderedset"
)

type Result struct {
	lock   *sync.RWMutex
	config ResultConfig

	references *orderedset.OrderedSet[Reference]
	// testEventsByReference *orderedmap.OrderedMap[Reference, []Event]   // all action types except "output"
	testEventsByReference map[Reference][]Event                        // all action types // TODO rethink this
	testOutputByReference map[Reference][]Event                        // only "output" action // TODO rethink this
	referencesByAction    map[Action]*orderedset.OrderedSet[Reference] // all action types except "output"

	testReferencesByAction map[Action]*orderedset.OrderedSet[Reference]
	conclusion             map[Reference]Action
	start                  time.Time
	offset                 time.Duration
	lastEventTime          time.Time

	coverage *float64
}

type ResultConfig struct {
	TrackOtherOutput   bool
	TrackFailingOutput bool
}

type ResultStats struct {
	Passed  int
	Failed  int
	Skipped int
	Running int
}

func (s ResultStats) Total() int {
	return s.Passed + s.Failed + s.Skipped
}

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
		testEventsByReference:  make(map[Reference][]Event),
		testOutputByReference:  make(map[Reference][]Event),
		referencesByAction:     referencesByAction,
		testReferencesByAction: testReferencesByAction,
		conclusion:             make(map[Reference]Action),
	}
}

func (r *Result) Elapsed() time.Duration {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.lastEventTime.IsZero() {
		return time.Now().Add(r.offset).Sub(r.start)
	}
	return r.lastEventTime.Sub(r.start)
}

func (r *Result) Update(e Event) {
	// TODO: check for e.Error and report to the UI when found...

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.start.IsZero() {
		r.start = e.Time
		r.offset = time.Since(e.Time)
	}

	r.lastEventTime = e.Time

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

	// process conclusion
	switch e.Action {
	case PassAction, SkipAction, FailAction:

		r.conclusion[e.Reference] = e.Action
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

func (r Result) References() []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.references.Values()
}

func (r Result) ReferenceEvents(reference Reference) []Event {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.testEventsByReference[reference]
}

func (r Result) ReferencesByAction(action Action) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if refs, ok := r.referencesByAction[action]; ok {
		return refs.Values()
	}
	return nil
}

func (r Result) TestReferencesByAction(action Action) []Reference {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if refs, ok := r.testReferencesByAction[action]; ok {
		return refs.Values()
	}
	return nil
}

func (r Result) TestStats() ResultStats {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return ResultStats{
		Passed:  r.testReferencesByAction[PassAction].Size(),
		Failed:  r.testReferencesByAction[FailAction].Size(),
		Skipped: r.testReferencesByAction[SkipAction].Size(),
		Running: r.testReferencesByAction[RunAction].Size(),
	}
}

func (r Result) ReferenceOutput(reference Reference) string {
	r.lock.RLock()
	defer r.lock.RUnlock()

	sb := strings.Builder{}
	for _, e := range r.testOutputByReference[reference] {
		_, err := sb.WriteString(e.Output)
		if err != nil {
			// TODO
			panic(err)
		}
	}
	return sb.String()
}

func (r Result) ReferenceConclusion(reference Reference) Action {
	r.lock.RLock()
	defer r.lock.RUnlock()

	act, ok := r.conclusion[reference]
	if !ok {
		return ""
	}
	return act
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
	hasMirroredRefs := len(r.conclusion) == r.references.Size()
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
