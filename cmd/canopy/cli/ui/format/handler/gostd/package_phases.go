package gostd

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
)

// startupPie marks how much of a package's elapsed time went to startup, to the nearest quarter: ◔ ◑ ◕ ●.
var startupPie = []string{"", "◔", "◑", "◕", "●"}

// withTestsElapsed marks go test's elapsed time on a package's "ok"/"FAIL" summary line with how much of it was
// startup, e.g. "7.832s" becomes "7.832s ●". Go's number covers the whole test binary process, so when startup
// dominates it hides how fast the tests themselves were. The time itself stays go's wall clock, so it still means
// the same thing on every line. It is only marked when startup is notable (see gotest.NotableStartup) and at least
// the nearest quarter; any other event is returned unchanged.
func withTestsElapsed(result *gotest.Result, pkgRef gotest.Reference, e gotest.Event) gotest.Event {
	phases, ok := result.PackagePhases(pkgRef)
	if !ok || phases.Startup < gotest.NotableStartup {
		return e
	}

	return editSummaryLine(e, func(fields []string) []string {
		total, err := time.ParseDuration(fields[2])
		if err != nil || total <= 0 {
			return fields
		}
		quarters := int(math.Round(4 * min(1, phases.Startup.Seconds()/total.Seconds())))
		if quarters > 0 {
			fields[2] += " " + startupPie[quarters]
		}
		return fields
	})
}

// firstTestNote puts how long the run took to reach its first test at the far right of the first package result line
// whose tests actually ran, e.g. "(started after 14.08s)". It is a one-time fact about the stretch before that line
// (compiling plus launching test binaries), so it sits beside the first result it explains instead of in the live
// summary, which is for status.
//
// Both marks are wall clock times taken as the events reach this handler, so they line up with the summary timer.
type firstTestNote struct {
	// launchedAt is when the run request arrived, published right after canopy starts the go test process
	launchedAt time.Time

	// firstTestAt is when the first test event (any package) arrived
	firstTestAt time.Time

	// done is set once a package result line with tests has been rendered, noted or not
	done bool
}

func (n *firstTestNote) observeRunRequest() {
	if n.launchedAt.IsZero() {
		n.launchedAt = time.Now()
	}
}

func (n *firstTestNote) observe(e gotest.Event) {
	if n.firstTestAt.IsZero() && !n.launchedAt.IsZero() && !e.Reference.IsPackage() {
		n.firstTestAt = time.Now()
	}
}

// annotate adds the note to e when it is the first summary line of a package whose tests ran. Only that first line is
// considered: if the wait wasn't notable (see gotest.NotableStartup), no later line gets the note either.
func (n *firstTestNote) annotate(result *gotest.Result, pkgRef gotest.Reference, e gotest.Event) gotest.Event {
	if n.done || !isSummaryLine(e) {
		return e
	}
	if _, ok := result.PackagePhases(pkgRef); !ok {
		// cached, no tests ran, or no start event: not a package whose tests actually ran
		return e
	}
	n.done = true

	if n.launchedAt.IsZero() || n.firstTestAt.IsZero() {
		return e
	}
	delay := n.firstTestAt.Sub(n.launchedAt)
	if delay < gotest.NotableStartup {
		return e
	}

	return editSummaryLine(e, func(fields []string) []string {
		// the parentheses tell the line formatter this field is already formatted (no brackets)
		return append(fields, fmt.Sprintf("(started after %.2fs)", delay.Seconds()))
	})
}

// isSummaryLine reports whether e is go test's per-package "ok"/"FAIL" line with an elapsed time field.
func isSummaryLine(e gotest.Event) bool {
	_, ok := summaryLineFields(e.Output)
	return ok
}

// summaryLineFields splits go test's per-package "ok"/"FAIL" line (status, package, elapsed, then optional extras
// such as coverage) into its tab-separated fields. Lines without an elapsed field, like "[build failed]", don't
// count.
func summaryLineFields(out string) ([]string, bool) {
	if !output.HasAny(output.HasPackageOKMarking, output.HasFailedPackageMarking)(out) {
		return nil, false
	}
	line, _, _ := strings.Cut(out, "\n")
	fields := strings.Split(line, "\t")
	if len(fields) < 3 || !output.HasTimeMarker(fields[2]) {
		return nil, false
	}
	return fields, true
}

// editSummaryLine rewrites the fields of go test's per-package summary line, keeping anything after the line as is.
// Any other event is returned unchanged.
func editSummaryLine(e gotest.Event, edit func([]string) []string) gotest.Event {
	fields, ok := summaryLineFields(e.Output)
	if !ok {
		return e
	}
	_, trailer, hasNewline := strings.Cut(e.Output, "\n")

	e.Output = strings.Join(edit(fields), "\t")
	if hasNewline {
		e.Output += "\n" + trailer
	}
	return e
}
