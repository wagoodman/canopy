package gostd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/internal"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*verboseHandler)(nil)
	_ partybus.Handler = (*verboseHandler)(nil)
)

// PackageConfig holds configuration for gostd package handlers.
type PackageConfig struct {
	// Color enables colored output.
	Color bool

	// PackageNameWidth sets the width for package name alignment.
	PackageNameWidth int

	// IDE is the IDE context for generating clickable links.
	IDE ide.Context

	// HidePackagesWithNoTestFiles controls visibility of packages without tests.
	HidePackagesWithNoTestFiles bool

	// StripPackagePrefix is removed from package names in output.
	StripPackagePrefix string

	// LoosePackageOrder allows skipping stale packages for more real-time output.
	LoosePackageOrder bool

	// StalePackageDuration is the threshold for considering a package stale.
	StalePackageDuration time.Duration
}

// verboseHandler formats test output in verbose mode, showing all test output
// including RUN, PASS, and FAIL markers. Outputs packages in alphabetical order.
type verboseHandler struct {
	// writer is where formatted output is written.
	writer io.Writer

	// result tracks all test events and outcomes.
	result *gotest.Result

	// packages tracks package references in order seen.
	packages *orderedset.OrderedSet[gotest.Reference]

	// panic tracks which test references have panic output.
	panic map[gotest.Reference]bool

	// formatter converts test events to formatted output.
	formatter func(gotest.Event, bool) fmt.Stringer
}

// NewVerboseHandler creates a handler that formats output in verbose mode,
// showing all test execution details.
func NewVerboseHandler(writer io.Writer, config PackageConfig) handler.Handler {
	return &verboseHandler{
		writer:   writer,
		result:   gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages: orderedset.New[gotest.Reference](),
		panic:    make(map[gotest.Reference]bool),
		formatter: presenter.NewGoVerboseEventFactory(
			presenter.GoEventConfig{
				Style:                   style.NewGo(config.Color),
				IDE:                     config.IDE,
				PackageNameWidth:        config.PackageNameWidth,
				StripPackagePrefix:      config.StripPackagePrefix,
				HideExecutionTestEvents: false,
			},
		).NewEvent,
	}
}

// Handle processes partybus events, routing test events to the handler.
func (h *verboseHandler) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		return h.OnGoTestEvent(goTestEvent)
	}
	return nil
}

// OnGoTestEvent processes test events, updating result state and rendering
// completed packages.
func (h *verboseHandler) OnGoTestEvent(e gotest.Event) error {
	h.result.Update(e)
	if e.Reference.IsPackage() {
		h.packages.Add(e.Reference)
	}

	if output.HasPanicMarking(e.Output) {
		h.panic[e.Reference] = true
	}

	switch e.Action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		if e.Reference.IsPackage() {
			// try to output completed packages in start order
			h.render()
		}
	}

	return nil
}

// render outputs completed packages in strict alphabetical order.
func (h *verboseHandler) render() {
	// only render packages that are done, and render them in alphabetical order
	// this is the reason why we cannot use a package handler (since order of packages is important, independent of the order of completion)
	pkgs := h.packages.Values()
	sort.Sort(gotest.References(pkgs))
	for len(pkgs) > 0 {
		pkgRef := pkgs[0]
		action := h.result.ReferenceConclusiveAction(pkgRef)

		if !action.Completed() {
			// this package isn't done yet, so we can't output anything after it
			return
		}

		h.outputPackage(
			pkgRef,
		)

		h.packages.Delete(pkgRef)
		pkgs = h.packages.Values()
	}
}

// outputPackage writes all output for a package, including test logs and conclusions.
func (h *verboseHandler) outputPackage(pkgRef gotest.Reference) {
	for _, testRef := range h.result.Children(pkgRef) {
		// output run/pause/continue and logs
		h.outputTest(testRef, false, func(e gotest.Event) bool {
			return !output.HasConclusionMarking(e.Output)
		})
	}
	for _, testRef := range h.result.Children(pkgRef) {
		// output pass/failed
		h.outputTest(testRef, true, func(e gotest.Event) bool {
			return output.HasConclusionMarking(e.Output)
		})
	}

	// output package conclusions
	outputEvents := h.result.ReferenceEvents(pkgRef)
	for _, e := range outputEvents {
		if output.HasAny(output.HasPackagePassMarking, output.HasPackageCoverageMarking)(e.Output) {
			// if the package passed or there is a final coverage line, we don't need to output anything
			continue
		}
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(h.writer, fmtr.String())
		}
	}
}

// outputTest writes output for a test and its children, optionally indenting conclusions.
func (h *verboseHandler) outputTest(testRef gotest.Reference, indent bool, include func(gotest.Event) bool) {
	outputEvents := h.getEvents(testRef, include)

	for _, e := range outputEvents {
		writer := h.writer
		if indent {
			writer = internal.NewIndentWriterForReference(writer, e.Reference)
		}
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(writer, fmtr.String())
		}
	}
}

// getEvents collects events for a test and its children, filtered by the include function.
func (h *verboseHandler) getEvents(testRef gotest.Reference, include func(gotest.Event) bool) []gotest.Event {
	outputEvents := filterEvents(h.result.ReferenceEvents(testRef), include)

	for _, childRef := range h.result.Children(testRef) {
		outputEvents = append(outputEvents, h.getEvents(childRef, include)...)
	}

	return outputEvents
}

// filterEvents returns only events that pass the include filter.
func filterEvents(events []gotest.Event, include func(gotest.Event) bool) []gotest.Event {
	var filtered []gotest.Event
	for _, e := range events {
		if include(e) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// String returns any remaining buffered output (none in this implementation).
func (h *verboseHandler) String() string {
	return ""
}
