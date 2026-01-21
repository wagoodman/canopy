// Package gostd provides handlers that format test output to match standard
// Go test output, with optional enhancements for readability.
package gostd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*quietHandler)(nil)
	_ partybus.Handler = (*quietHandler)(nil)
)

// quietHandler formats test output in quiet mode, showing only failing tests and
// final package results. It buffers output and renders packages in alphabetical order
// as they complete.
type quietHandler struct {
	// config holds formatting configuration.
	config PackageConfig

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

	// groupConfig configures collapsible output groups.
	groupConfig group.Config

	// grouper handles streaming package output with optional grouping.
	grouper *group.StreamingGroupRenderer[gotest.Reference]
}

// NewQuietHandler creates a handler that formats output in quiet mode, showing
// only failures and package summaries.
func NewQuietHandler(writer io.Writer, config PackageConfig) handler.Handler {
	h := &quietHandler{
		config:   config,
		writer:   writer,
		result:   gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages: orderedset.New[gotest.Reference](),
		panic:    make(map[gotest.Reference]bool),
		formatter: presenter.NewGoQuietEventFactory(
			presenter.GoEventConfig{
				Style:                   style.NewGo(config.Color),
				IDE:                     config.IDE,
				PackageNameWidth:        config.PackageNameWidth,
				StripPackagePrefix:      config.StripPackagePrefix,
				HideExecutionTestEvents: false,
			},
		).NewEvent,
		groupConfig: config.Grouping,
	}
	h.grouper = group.NewStreamingGroupRenderer(
		h.writer,
		h.groupConfig,
		func(ref gotest.Reference) (shouldGroup bool, completed bool) {
			action := h.result.ReferenceConclusiveAction(ref)
			return h.groupConfig.ShouldGroup(action), action.Completed()
		},
		func(pkgRef gotest.Reference, writer io.Writer) {
			h.outputPackageToWriter(pkgRef, writer, h.hasFailure, func(e gotest.Event) bool {
				return !output.HasStateMarking(e.Output)
			})
		},
	)
	return h
}

// Handle processes partybus events, routing test events to the handler.
func (h *quietHandler) Handle(e partybus.Event) error {
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
func (h *quietHandler) OnGoTestEvent(e gotest.Event) error {
	// skip packages with no tests if configured to hide them
	if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && h.config.HidePackagesWithNoTestFiles {
		return nil
	}

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

// render outputs completed packages in alphabetical order, with optional loose
// ordering that skips stale packages.
func (h *quietHandler) render() {
	// only render packages that are done, and render them in alphabetical order
	// this is the reason why we cannot use a package handler (since order of packages is important, independent of the order of completion)
	pkgs := h.packages.Values()
	sort.Sort(gotest.References(pkgs))

	// check if across-packages grouping is enabled
	if h.groupConfig.AcrossPackages && h.groupConfig.Formatter != nil {
		h.grouper.RenderWithGrouping(pkgs, func(ref gotest.Reference) []gotest.Reference {
			h.packages.Delete(ref)
			return h.packages.Values()
		})
		return
	}

	offset := 0
	for len(pkgs) > 0 && offset < len(pkgs) {
		pkgRef := pkgs[offset]
		action := h.result.ReferenceConclusiveAction(pkgRef)

		if !action.Completed() {
			if h.config.LoosePackageOrder {
				// attempt alphabetical order... unless a package is running for "too long"
				elapsed := h.result.ReferenceElapsed(pkgRef, true)
				if elapsed > h.config.StalePackageDuration {
					// this package has been running for too long, blocking the result output of other packages.
					// Let's skip around this and render the other results that are done. This will allow for package
					// results to show up in a more realtime manner, but sacrifice the strict alphabetical order
					// of packages.
					offset++
					continue
				}
			}
			// strictly alphabetical order... or this package isn't stale yet.
			// this package isn't done yet, so we can't output anything else after it
			return
		}

		h.outputPackage(pkgRef)
		h.packages.Delete(pkgRef)
		pkgs = h.packages.Values()
	}
}

// hasFailure recursively checks if a test reference or any of its children failed.
func (h *quietHandler) hasFailure(testRef gotest.Reference) bool {
	if h.result.ReferenceConclusiveAction(testRef) == gotest.FailAction {
		return true
	}
	for _, child := range h.result.Children(testRef) {
		if h.hasFailure(child) {
			return true
		}
	}
	return false
}

// outputPackage writes all output for a completed package.
func (h *quietHandler) outputPackage(pkgRef gotest.Reference) {
	writer, done := h.writerForPackage(pkgRef)
	defer done()

	h.outputPackageToWriter(pkgRef, writer, h.hasFailure, func(e gotest.Event) bool {
		return !output.HasStateMarking(e.Output)
	})
}

// writerForPackage returns the appropriate writer for a package based on grouping config.
// The returned done function must be called to flush any buffered group output.
func (h *quietHandler) writerForPackage(pkgRef gotest.Reference) (io.Writer, func()) {
	action := h.result.ReferenceConclusiveAction(pkgRef)

	if !h.groupConfig.ShouldGroup(action) {
		return h.writer, func() {}
	}

	groupWriter := group.NewWriter(h.writer, pkgRef.Package, h.groupConfig.Formatter)
	return groupWriter, func() {
		_, _ = groupWriter.Flush()
	}
}

// outputPackageToWriter writes output for a completed package to the specified writer.
func (h *quietHandler) outputPackageToWriter(pkgRef gotest.Reference, writer io.Writer, include func(gotest.Reference) bool, render func(gotest.Event) bool) {
	for _, testRef := range h.result.Children(pkgRef) {
		if include(testRef) {
			h.outputTestToWriter(testRef, writer, include, render)
		}
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
			fmt.Fprint(writer, fmtr.String())
		}
	}
}

// outputTestToWriter writes output for a test and its children to the specified writer.
func (h *quietHandler) outputTestToWriter(testRef gotest.Reference, writer io.Writer, include func(gotest.Reference) bool, render func(gotest.Event) bool) {
	outputEvents := h.getEvents(testRef, include)

	sort.Slice(outputEvents, func(i, j int) bool {
		return outputEvents[i].Index < outputEvents[j].Index
	})

	for _, e := range outputEvents {
		if !render(e) {
			continue
		}
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(writer, fmtr.String())
		}
	}
}

// getEvents collects events for a test reference and its children, recursively.
func (h *quietHandler) getEvents(testRef gotest.Reference, include func(gotest.Reference) bool) []gotest.Event {
	if !include(testRef) {
		return nil
	}

	outputEvents := spliceConclusionFirst(h.result.ReferenceEvents(testRef))

	for _, childRef := range h.result.Children(testRef) {
		if include(childRef) {
			outputEvents = append(outputEvents, h.getEvents(childRef, include)...)
		}
	}

	return outputEvents
}

// String returns any remaining buffered output and closes any open streaming group.
func (h *quietHandler) String() string {
	h.grouper.Close()
	return ""
}

// spliceConclusionFirst moves the conclusion event (--- FAIL) to replace the
// run event (=== RUN) to make output more compact.
func spliceConclusionFirst(es []gotest.Event) (outputEvents []gotest.Event) {
	// find the conclusion event (the last event with a conclusion marking of --- FAIL)
	rmIdx := -1
	var finalEvent *gotest.Event
	for i := len(es) - 1; i >= 0; i-- {
		e := es[i]
		if output.HasConclusionMarking(e.Output) {
			finalEvent = &e
			rmIdx = i
			break
		}
	}

	// find the event where the === RUN marking is found, then replace that event with the final event
	if finalEvent == nil {
		return es
	}

	for i, e := range es {
		if output.HasRunMarking(e.Output) {
			// replace the run event with the final event
			finalEvent.Index = e.Index // preserve the index of the === RUN event
			es[i] = *finalEvent

			break
		}
	}

	// remove the final event from the output events
	outputEvents = make([]gotest.Event, 0, len(es)-1)
	outputEvents = append(outputEvents, es[:rmIdx]...)
	outputEvents = append(outputEvents, es[rmIdx+1:]...)

	return outputEvents
}
