package gostd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
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

	// ExecutionMarkers controls visibility of test state markers (=== RUN/PAUSE/CONT).
	// Valid values: "none" (hide all), "all" (show all), "parallel-only" (show only PAUSE/CONT).
	ExecutionMarkers string

	// Grouping configures collapsible output groups for CI environments.
	Grouping group.Config
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

	// groupConfig configures collapsible output groups.
	groupConfig group.Config

	// grouper handles streaming package output with optional grouping.
	grouper *group.StreamingGroupRenderer[gotest.Reference]

	// hidePackagesWithNoTestFiles controls visibility of packages without tests.
	hidePackagesWithNoTestFiles bool

	// executionMarkers controls visibility of test state markers (=== RUN/PAUSE/CONT).
	executionMarkers string
}

// NewVerboseHandler creates a handler that formats output in verbose mode,
// showing all test execution details.
func NewVerboseHandler(writer io.Writer, config PackageConfig) handler.Handler {
	h := &verboseHandler{
		writer:   writer,
		result:   gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages: orderedset.New[gotest.Reference](),
		panic:    make(map[gotest.Reference]bool),
		formatter: presenter.NewGoVerboseEventFactory(
			presenter.GoEventConfig{
				Style:              style.NewGo(config.Color),
				IDE:                config.IDE,
				PackageNameWidth:   config.PackageNameWidth,
				StripPackagePrefix: config.StripPackagePrefix,
				ExecutionMarkers:   config.ExecutionMarkers,
			},
		).NewEvent,
		groupConfig:                 config.Grouping,
		hidePackagesWithNoTestFiles: config.HidePackagesWithNoTestFiles,
		executionMarkers:            config.ExecutionMarkers,
	}
	h.grouper = group.NewStreamingGroupRenderer(
		h.writer,
		h.groupConfig,
		func(ref gotest.Reference) (shouldGroup bool, completed bool) {
			action := h.result.ReferenceConclusiveAction(ref)
			return h.groupConfig.ShouldGroup(action), action.Completed()
		},
		h.outputPackageToWriter,
	)
	return h
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
	// skip packages with no tests if configured to hide them
	if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && h.hidePackagesWithNoTestFiles {
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

// render outputs completed packages in strict alphabetical order.
func (h *verboseHandler) render() {
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
	writer, done := h.writerForPackage(pkgRef)
	defer done()

	h.outputPackageToWriter(pkgRef, writer)
}

// writerForPackage returns the appropriate writer for a package based on grouping config.
// The returned done function must be called to flush any buffered group output.
func (h *verboseHandler) writerForPackage(pkgRef gotest.Reference) (io.Writer, func()) {
	action := h.result.ReferenceConclusiveAction(pkgRef)

	if !h.groupConfig.ShouldGroup(action) {
		return h.writer, func() {}
	}

	groupWriter := group.NewWriter(h.writer, pkgRef.Package, h.groupConfig.Formatter)
	return groupWriter, func() {
		_, _ = groupWriter.Flush()
	}
}

// outputPackageToWriter writes all output for a package to the specified writer.
func (h *verboseHandler) outputPackageToWriter(pkgRef gotest.Reference, writer io.Writer) {
	// check if we need AcrossTests grouping, which requires separating non-conclusions from conclusions
	_, isGroupWriter := writer.(*group.Writer)
	_, isStreamingGroupWriter := writer.(*group.StreamingGroupWriter)
	useAcrossTestsGrouping := h.groupConfig.AcrossTests && h.groupConfig.Formatter != nil && !isGroupWriter && !isStreamingGroupWriter

	if useAcrossTestsGrouping {
		// AcrossTests grouping: first output all non-conclusions, then group conclusions together
		for _, testRef := range h.result.Children(pkgRef) {
			h.outputTestToWriter(testRef, writer, false, false, func(e gotest.Event) bool {
				return !output.HasConclusionMarking(e.Output)
			})
		}
		h.outputConclusionsWithGrouping(pkgRef, writer)
	} else {
		// standard verbose output: output each test's events together before moving to the next test.
		// This matches go test's output order where each test's RUN, logs, and PASS/FAIL appear together.
		for _, testRef := range h.result.Children(pkgRef) {
			// output run/pause/continue and logs (forConclusions=false, indent=false)
			h.outputTestToWriter(testRef, writer, false, false, func(e gotest.Event) bool {
				return !output.HasConclusionMarking(e.Output)
			})
			// output conclusion (forConclusions=true, indent=true)
			h.outputTestToWriter(testRef, writer, true, true, func(e gotest.Event) bool {
				return output.HasConclusionMarking(e.Output)
			})
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

// outputConclusionsWithGrouping outputs test conclusions, grouping consecutive tests when their
// status matches an enabled grouping option. This helps reduce noise when a package has many
// passing/skipped tests and a few failures by collapsing them into a single collapsible group.
func (h *verboseHandler) outputConclusionsWithGrouping(pkgRef gotest.Reference, writer io.Writer) {
	children := h.result.Children(pkgRef)

	var groupBuffer []gotest.Reference

	flushGrouped := func() {
		if len(groupBuffer) <= 1 {
			// single test or empty - output without grouping (forConclusions=true, indent=true)
			for _, ref := range groupBuffer {
				h.outputTestToWriter(ref, writer, true, true, func(e gotest.Event) bool {
					return output.HasConclusionMarking(e.Output)
				})
			}
		} else {
			// multiple consecutive groupable tests - group them (forConclusions=true, indent=true)
			statusLabel := h.groupConfig.GroupedStatusLabel()
			title := fmt.Sprintf("%d %s tests", len(groupBuffer), statusLabel)
			groupWriter := group.NewWriter(writer, title, h.groupConfig.Formatter)
			for _, ref := range groupBuffer {
				h.outputTestToWriter(ref, groupWriter, true, true, func(e gotest.Event) bool {
					return output.HasConclusionMarking(e.Output)
				})
			}
			_, _ = groupWriter.Flush()
		}
		groupBuffer = nil
	}

	for _, testRef := range children {
		action := h.result.ReferenceConclusiveAction(testRef)

		if h.groupConfig.ShouldGroup(action) {
			groupBuffer = append(groupBuffer, testRef)
		} else {
			// flush any accumulated tests
			flushGrouped()
			// output this test directly (forConclusions=true, indent=true)
			h.outputTestToWriter(testRef, writer, true, true, func(e gotest.Event) bool {
				return output.HasConclusionMarking(e.Output)
			})
		}
	}

	// flush remaining tests
	flushGrouped()
}

// outputTestToWriter writes output for a test and its children to the specified writer.
// Parameters:
// - forConclusions: true when outputting conclusion events (PASS/FAIL/SKIP), false for execution events (RUN/PAUSE/CONT)
// - indent: whether to indent output based on test hierarchy depth
func (h *verboseHandler) outputTestToWriter(testRef gotest.Reference, writer io.Writer, forConclusions, indent bool, include func(gotest.Event) bool) {
	children := h.result.Children(testRef)

	// check if we should apply AcrossCases grouping:
	// - AcrossCases is enabled
	// - formatter is set
	// - this test has children
	// - we're outputting conclusions, not execution events (RUN/PAUSE/CONT)
	// - not already writing to a group (avoid nesting)
	_, isGroupWriter := writer.(*group.Writer)
	_, isStreamingGroupWriter := writer.(*group.StreamingGroupWriter)
	shouldApplyCaseGrouping := h.groupConfig.AcrossCases &&
		h.groupConfig.Formatter != nil &&
		len(children) > 0 &&
		forConclusions &&
		!isGroupWriter && !isStreamingGroupWriter

	if shouldApplyCaseGrouping {
		// output this test's own events (not children's)
		h.outputOwnEvents(testRef, writer, indent, include)
		// output children with grouping applied
		h.outputChildrenWithCaseGrouping(testRef, writer, forConclusions, indent, include)
	} else {
		// standard behavior: collect all events from test and children
		outputEvents := h.getEvents(testRef, include)
		for _, e := range outputEvents {
			w := writer
			if indent {
				w = internal.NewIndentWriterForReference(writer, e.Reference)
			}
			fmtr := h.formatter(e, h.panic[e.Reference])
			if strings.TrimSpace(e.Output) != "" {
				fmt.Fprint(w, fmtr.String())
			}
		}
	}
}

// outputOwnEvents outputs only the test's own events (not its children's events).
func (h *verboseHandler) outputOwnEvents(testRef gotest.Reference, writer io.Writer, indent bool, include func(gotest.Event) bool) {
	outputEvents := filterEvents(h.result.ReferenceEvents(testRef), include)
	for _, e := range outputEvents {
		w := writer
		if indent {
			w = internal.NewIndentWriterForReference(writer, e.Reference)
		}
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(w, fmtr.String())
		}
	}
}

// outputChildrenWithCaseGrouping outputs children of a test, grouping consecutive
// children that match the grouping config (similar to outputConclusionsWithGrouping
// but for subtests within a parent test).
func (h *verboseHandler) outputChildrenWithCaseGrouping(testRef gotest.Reference, writer io.Writer, forConclusions, indent bool, include func(gotest.Event) bool) {
	children := h.result.Children(testRef)

	var groupBuffer []gotest.Reference

	flushGrouped := func() {
		if len(groupBuffer) == 0 {
			return
		}
		// group consecutive groupable cases (even single items to highlight failures)
		statusLabel := h.groupConfig.GroupedStatusLabel()
		title := statusLabel + " cases"
		groupWriter := group.NewWriter(writer, title, h.groupConfig.Formatter)
		for _, ref := range groupBuffer {
			h.outputTestToWriter(ref, groupWriter, forConclusions, indent, include)
		}
		_, _ = groupWriter.Flush()
		groupBuffer = nil
	}

	for _, childRef := range children {
		action := h.result.ReferenceConclusiveAction(childRef)

		if h.groupConfig.ShouldGroup(action) {
			groupBuffer = append(groupBuffer, childRef)
		} else {
			// flush any accumulated cases
			flushGrouped()
			// output this case directly (it may recursively apply case grouping to its children)
			h.outputTestToWriter(childRef, writer, forConclusions, indent, include)
		}
	}

	// flush remaining cases
	flushGrouped()
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

// String returns any remaining buffered output and closes any open streaming group.
func (h *verboseHandler) String() string {
	h.grouper.Close()
	return ""
}
