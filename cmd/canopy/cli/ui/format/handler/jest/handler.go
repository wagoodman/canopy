// Package jest provides handlers that format test output in Jest-style,
// with checkmarks and X marks for test results. It includes CI grouping
// support for collapsible output in CI environments.
package jest

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*jestHandler)(nil)
	_ partybus.Handler = (*jestHandler)(nil)
)

// Config holds configuration for the jest-style handler.
type Config struct {
	// Color enables colored output.
	Color bool

	// Grouping configures collapsible output groups for CI environments.
	Grouping group.Config

	// HidePackagesWithNoTestFiles hides packages that have no test files.
	HidePackagesWithNoTestFiles bool

	// Verbose shows all test output, not just failures.
	Verbose bool
}

// jestHandler formats test output in Jest style with checkmarks and X marks.
type jestHandler struct {
	config Config
	writer io.Writer
	result *gotest.Result
	style  style.Jest

	// packages tracks package references in order seen.
	packages *orderedset.OrderedSet[gotest.Reference]

	// groupConfig configures collapsible output groups.
	groupConfig group.Config
}

// NewHandler creates a jest-style handler.
func NewHandler(writer io.Writer, config Config) handler.Handler {
	return &jestHandler{
		config:      config,
		writer:      writer,
		result:      gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages:    orderedset.New[gotest.Reference](),
		style:       style.NewJest(config.Color),
		groupConfig: config.Grouping,
	}
}

// Handle processes partybus events, routing test events to the handler.
func (h *jestHandler) Handle(e partybus.Event) error {
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
func (h *jestHandler) OnGoTestEvent(e gotest.Event) error {
	h.result.Update(e)
	if e.Reference.IsPackage() {
		h.packages.Add(e.Reference)
	}

	switch e.Action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		if e.Reference.IsPackage() {
			h.render()
		}
	}

	return nil
}

// render outputs completed packages in alphabetical order.
func (h *jestHandler) render() {
	pkgs := h.packages.Values()
	sort.Sort(gotest.References(pkgs))

	// check if across-packages grouping is enabled
	if h.groupConfig.AcrossPackages && h.groupConfig.Formatter != nil {
		h.renderWithPackageGrouping(pkgs)
		return
	}

	for len(pkgs) > 0 {
		pkgRef := pkgs[0]
		action := h.result.ReferenceConclusiveAction(pkgRef)

		if !action.Completed() {
			return
		}

		// determine if package passed for grouping decision
		passed := action == gotest.PassAction

		// select the writer - use a group writer if grouping is enabled for this package status
		writer := h.writer
		var groupWriter *group.Writer
		if h.groupConfig.ShouldGroup(passed) {
			groupWriter = group.NewWriter(h.writer, pkgRef.Package, h.groupConfig.Formatter)
			writer = groupWriter
		}

		h.outputPackage(pkgRef, writer, action)

		// flush the group writer to emit group markers
		if groupWriter != nil {
			_, _ = groupWriter.Flush()
		}

		h.packages.Delete(pkgRef)
		pkgs = h.packages.Values()
	}
}

// renderWithPackageGrouping renders packages, grouping consecutive passing packages together.
// This helps reduce noise when there are many passing packages before a failure by collapsing
// consecutive passing packages into a single collapsible group.
func (h *jestHandler) renderWithPackageGrouping(pkgs []gotest.Reference) {
	var passingBuffer []gotest.Reference

	flushPassing := func() {
		if len(passingBuffer) == 0 {
			return
		}
		if len(passingBuffer) == 1 {
			// single passing package - output with individual grouping
			pkgRef := passingBuffer[0]
			action := h.result.ReferenceConclusiveAction(pkgRef)
			writer := h.writer
			var groupWriter *group.Writer
			if h.groupConfig.ShouldGroup(true) {
				groupWriter = group.NewWriter(h.writer, pkgRef.Package, h.groupConfig.Formatter)
				writer = groupWriter
			}
			h.outputPackage(pkgRef, writer, action)
			if groupWriter != nil {
				_, _ = groupWriter.Flush()
			}
		} else {
			// multiple consecutive passing packages - group them together
			title := fmt.Sprintf("%d passing packages", len(passingBuffer))
			groupWriter := group.NewWriter(h.writer, title, h.groupConfig.Formatter)
			for _, pkgRef := range passingBuffer {
				action := h.result.ReferenceConclusiveAction(pkgRef)
				h.outputPackage(pkgRef, groupWriter, action)
			}
			_, _ = groupWriter.Flush()
		}
		passingBuffer = nil
	}

	for len(pkgs) > 0 {
		pkgRef := pkgs[0]
		action := h.result.ReferenceConclusiveAction(pkgRef)

		if !action.Completed() {
			// flush accumulated passing packages before blocking
			flushPassing()
			return
		}

		passed := action == gotest.PassAction

		if passed && h.groupConfig.GroupPassed {
			passingBuffer = append(passingBuffer, pkgRef)
		} else {
			// flush any accumulated passing packages
			flushPassing()
			// output this non-passing package directly (not grouped at package level)
			h.outputPackage(pkgRef, h.writer, action)
		}

		h.packages.Delete(pkgRef)
		pkgs = h.packages.Values()
	}

	// flush remaining passing packages
	flushPassing()
}

// outputPackage writes jest-style output for a completed package.
func (h *jestHandler) outputPackage(pkgRef gotest.Reference, writer io.Writer, pkgAction gotest.Action) {
	// output package header
	title := h.packageTitle(pkgAction)
	fmt.Fprintf(writer, "%s %s\n", title, pkgRef.Package)

	// output tests
	children := h.result.Children(pkgRef)
	for _, testRef := range children {
		h.outputTest(testRef, writer, "  ")
	}
}

// outputTest writes jest-style output for a test and its children.
func (h *jestHandler) outputTest(testRef gotest.Reference, writer io.Writer, indent string) {
	action := h.result.ReferenceConclusiveAction(testRef)

	// skip running tests (not completed)
	if !action.Completed() {
		return
	}

	// determine if we should show this test
	showTest := h.config.Verbose || action == gotest.FailAction

	if showTest {
		title := h.testTitle(action)
		testName := testRef.TestName(false)
		elapsed := h.result.ReferenceElapsed(testRef, false)
		fmt.Fprintf(writer, "%s%s %s (%s)\n", indent, title, testName, formatDuration(elapsed))

		// output failure details
		if action == gotest.FailAction {
			h.outputFailureDetails(testRef, writer, indent+"  ")
		}
	}

	// output children recursively
	children := h.result.Children(testRef)
	for _, child := range children {
		h.outputTest(child, writer, indent+"  ")
	}
}

// outputFailureDetails writes failure output for a test.
func (h *jestHandler) outputFailureDetails(testRef gotest.Reference, writer io.Writer, indent string) {
	events := h.result.ReferenceEvents(testRef)
	for _, e := range events {
		if e.Action != gotest.OutputAction {
			continue
		}
		// skip run/conclusion markers
		if output.HasRunMarking(e.Output) || output.HasConclusionMarking(e.Output) {
			continue
		}
		trimmed := strings.TrimSpace(e.Output)
		if trimmed != "" {
			fmt.Fprintf(writer, "%s%s\n", indent, trimmed)
		}
	}
}

// packageTitle returns the styled title for a package result.
func (h *jestHandler) packageTitle(action gotest.Action) string {
	switch action {
	case gotest.PassAction:
		return h.style.SuccessTitle.Render(" PASS ")
	case gotest.FailAction:
		return h.style.FailureTitle.Render(" FAIL ")
	case gotest.SkipAction:
		return h.style.SkipTitle.Render(" SKIP ")
	default:
		return h.style.RunningTitle.Render(" RUNS ")
	}
}

// testTitle returns the styled title for a test result.
func (h *jestHandler) testTitle(action gotest.Action) string {
	switch action {
	case gotest.PassAction:
		return h.style.CheckTitle.Render("✔")
	case gotest.FailAction:
		return h.style.XTitle.Render("✕")
	case gotest.SkipAction:
		return h.style.Aux.Render("►►")
	default:
		return h.style.Aux.Render("…")
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// String returns any remaining buffered output (none in this implementation).
func (h *jestHandler) String() string {
	return ""
}
