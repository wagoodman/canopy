package gotest

import "time"

// NotableStartup is the shortest startup worth calling out: a package's startup before its result line's elapsed time
// is split into startup+tests, or the run's wait for its first test before "(started after X)" is. Below this (e.g. a
// warm machine, or Linux where exec is cheap) the split only repeats go's own number.
const NotableStartup = time.Second

// PackagePhases splits a concluded package's time, as go test reports it, at the package's first test event. All
// times come from go test's own event timestamps, so this works the same for live runs and replays.
type PackagePhases struct {
	// Startup runs from the package's "start" event (go test is about to check the test cache and exec the binary)
	// to its first test event. It covers exec'ing the test binary (on macOS including the OS scan of a freshly
	// linked binary), package init, and any TestMain setup that runs before the first test.
	Startup time.Duration

	// Tests runs from the package's first test event to its last. It is wall time, so parallel tests overlap
	// rather than add up, and it leaves out what follows the last test (TestMain teardown, coverage writing, exit).
	Tests time.Duration
}

type timeSpan struct {
	first, last time.Time
}

// PackagePhases returns the startup/tests split for a package. It reports false while the package is still in
// flight, when the result was replayed from the go test cache (nothing was launched), or when there is no split
// to make: no "start" event (older go versions) or no test ever ran (e.g. [no tests to run], or the binary died
// before its first test).
func (r Result) PackagePhases(pkg Reference) (PackagePhases, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if _, ok := r.conclusionEvent[pkg]; !ok {
		return PackagePhases{}, false
	}

	events := r.testEventsByReference[pkg]
	if len(events) == 0 || events[0].Action != StartAction {
		return PackagePhases{}, false
	}
	for _, e := range events {
		if e.HasAnnotation(Cached) {
			return PackagePhases{}, false
		}
	}

	span, ok := r.testSpanByPackage[pkg.Package]
	if !ok {
		return PackagePhases{}, false
	}

	return PackagePhases{
		Startup: span.first.Sub(events[0].Time),
		Tests:   span.last.Sub(span.first),
	}, true
}
