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

// eventPhase represents which part of test output we're rendering.
// Tests output in two phases matching go test's output order:
//   - execution: RUN/PAUSE/CONT markers and log lines (no indentation)
//   - conclusion: PASS/FAIL/SKIP markers (with hierarchy indentation)
type eventPhase int

const (
	executionPhase  eventPhase = iota // RUN/PAUSE/CONT + logs, no indent
	conclusionPhase                   // PASS/FAIL/SKIP, with indent
)

func (p eventPhase) shouldInclude(e gotest.Event) bool {
	hasConclusion := output.HasConclusionMarking(e.Output)
	return (p == conclusionPhase) == hasConclusion
}

func (p eventPhase) shouldIndent() bool {
	return p == conclusionPhase
}

// isNestedInGroup returns true if we're already writing inside a group writer.
// Used to avoid nesting groups within groups.
func isNestedInGroup(w io.Writer) bool {
	_, isGroup := w.(*group.Writer)
	_, isStreaming := w.(*group.StreamingGroupWriter)
	return isGroup || isStreaming
}

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

// writeEvent formats and writes a single event to the writer.
func (h *verboseHandler) writeEvent(e gotest.Event, w io.Writer) {
	if strings.TrimSpace(e.Output) == "" {
		return
	}
	fmt.Fprint(w, h.formatter(e, h.panic[e.Reference]).String())
}

// outputTestEvents outputs all events for a single test in go-test order:
// execution events (RUN, logs) followed by conclusion events (PASS/FAIL).
// This is the fundamental output primitive that maintains correct event ordering.
func (h *verboseHandler) outputTestEvents(testRef gotest.Reference, writer io.Writer) {
	h.outputTestPhase(testRef, writer, executionPhase)
	h.outputTestPhase(testRef, writer, conclusionPhase)
}

// outputPackageToWriter writes all output for a package to the specified writer.
func (h *verboseHandler) outputPackageToWriter(pkgRef gotest.Reference, writer io.Writer) {
	// AcrossTests grouping requires separating execution from conclusions across all tests
	useAcrossTestsGrouping := h.groupConfig.AcrossTests &&
		h.groupConfig.Formatter != nil &&
		!isNestedInGroup(writer)

	if useAcrossTestsGrouping {
		// output all execution events first, then group conclusions together
		for _, testRef := range h.result.Children(pkgRef) {
			h.outputTestPhase(testRef, writer, executionPhase)
		}
		h.outputConclusionsWithGrouping(pkgRef, writer)
	} else {
		// standard verbose output: each test's events together in go-test order
		for _, testRef := range h.result.Children(pkgRef) {
			h.outputTestEvents(testRef, writer)
		}
	}

	// output package-level conclusions (FAIL line, etc.)
	for _, e := range h.result.ReferenceEvents(pkgRef) {
		if output.HasAny(output.HasPackagePassMarking, output.HasPackageCoverageMarking, output.HasShuffleSeedMarking)(e.Output) {
			// the shuffle-seed line is framework noise: go echoes it once per package, all identical.
			continue
		}
		h.writeEvent(e, writer)
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
			// single test or empty - output without grouping
			for _, ref := range groupBuffer {
				h.outputTestPhase(ref, writer, conclusionPhase)
			}
		} else {
			// multiple consecutive groupable tests - wrap in a collapsible group
			statusLabel := h.groupConfig.GroupedStatusLabel()
			title := fmt.Sprintf("%d %s tests", len(groupBuffer), statusLabel)
			groupWriter := group.NewWriter(writer, title, h.groupConfig.Formatter)
			for _, ref := range groupBuffer {
				h.outputTestPhase(ref, groupWriter, conclusionPhase)
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
			flushGrouped()
			h.outputTestPhase(testRef, writer, conclusionPhase)
		}
	}

	flushGrouped()
}

// outputTestPhase writes events for a test and its children for the specified phase.
// The phase determines which events are included and whether indentation is applied.
func (h *verboseHandler) outputTestPhase(testRef gotest.Reference, writer io.Writer, phase eventPhase) {
	children := h.result.Children(testRef)

	// AcrossCases grouping only applies to conclusion phase with children
	shouldApplyCaseGrouping := h.groupConfig.AcrossCases &&
		h.groupConfig.Formatter != nil &&
		len(children) > 0 &&
		phase == conclusionPhase &&
		!isNestedInGroup(writer)

	if shouldApplyCaseGrouping {
		h.outputOwnEventsForPhase(testRef, writer, phase)
		h.outputChildrenWithCaseGrouping(testRef, writer, phase)
	} else {
		h.outputEventsRecursive(testRef, writer, phase)
	}
}

// outputOwnEventsForPhase outputs only the test's own events (not children) for the given phase.
func (h *verboseHandler) outputOwnEventsForPhase(testRef gotest.Reference, writer io.Writer, phase eventPhase) {
	for _, e := range h.result.ReferenceEvents(testRef) {
		if !phase.shouldInclude(e) {
			continue
		}
		w := writer
		if phase.shouldIndent() {
			w = internal.NewIndentWriterForReference(writer, e.Reference)
		}
		h.writeEvent(e, w)
	}
}

// outputEventsRecursive outputs events for a test and all its children for the given phase.
func (h *verboseHandler) outputEventsRecursive(testRef gotest.Reference, writer io.Writer, phase eventPhase) {
	for _, e := range h.result.ReferenceEvents(testRef) {
		if !phase.shouldInclude(e) {
			continue
		}
		w := writer
		if phase.shouldIndent() {
			w = internal.NewIndentWriterForReference(writer, e.Reference)
		}
		h.writeEvent(e, w)
	}

	for _, childRef := range h.result.Children(testRef) {
		h.outputEventsRecursive(childRef, writer, phase)
	}
}

// outputChildrenWithCaseGrouping outputs children of a test, grouping consecutive
// children that match the grouping config (for subtests within a parent test).
func (h *verboseHandler) outputChildrenWithCaseGrouping(testRef gotest.Reference, writer io.Writer, phase eventPhase) {
	children := h.result.Children(testRef)

	var groupBuffer []gotest.Reference

	flushGrouped := func() {
		if len(groupBuffer) == 0 {
			return
		}
		statusLabel := h.groupConfig.GroupedStatusLabel()
		title := statusLabel + " cases"
		groupWriter := group.NewWriter(writer, title, h.groupConfig.Formatter)
		for _, ref := range groupBuffer {
			h.outputTestPhase(ref, groupWriter, phase)
		}
		_, _ = groupWriter.Flush()
		groupBuffer = nil
	}

	for _, childRef := range children {
		action := h.result.ReferenceConclusiveAction(childRef)

		if h.groupConfig.ShouldGroup(action) {
			groupBuffer = append(groupBuffer, childRef)
		} else {
			flushGrouped()
			h.outputTestPhase(childRef, writer, phase)
		}
	}

	flushGrouped()
}

// String returns any remaining buffered output and closes any open streaming group.
func (h *verboseHandler) String() string {
	h.grouper.Close()
	return ""
}
