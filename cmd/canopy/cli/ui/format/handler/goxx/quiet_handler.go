// Package goxx provides enhanced Go test output handlers with improved formatting
// and features beyond standard Go test output.
package goxx

import (
	"fmt"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*QuietPackage)(nil)
	_ partybus.Handler = (*QuietPackage)(nil)
	_ fmt.Stringer     = (*QuietPackage)(nil)
)

// QuietPackageConfig holds configuration for quiet mode goxx output.
type QuietPackageConfig struct {
	// Color enables colored output.
	Color bool

	// PackageNameWidth sets the width for package name alignment.
	PackageNameWidth int

	// IDE is the IDE context for generating clickable links.
	IDE ide.Context

	// HidePackagesWithNoTestFiles controls visibility of packages without tests.
	HidePackagesWithNoTestFiles bool

	// ExecutionMarkers controls visibility of test state markers (=== RUN/PAUSE/CONT).
	// Valid values: "none" (hide all), "all" (show all), "parallel-only" (show only PAUSE/CONT).
	ExecutionMarkers string
}

// NewQuietHandler creates a handler that formats output in enhanced quiet mode,
// showing only failures with improved formatting.
func NewQuietHandler(writer io.Writer, config QuietPackageConfig) handler.Handler {
	return handler.NewPackageHandler(
		func(ref gotest.Reference, writer io.Writer) handler.Handler {
			return NewQuietPackage(writer, config, ref)
		}, writer)
}

// QuietPackage handles test events for a single package in quiet mode, buffering
// output and writing it when the package completes.
type QuietPackage struct {
	// writer is where formatted output is written.
	writer io.Writer

	// config holds formatting configuration.
	config QuietPackageConfig

	// pkg is the package name being handled.
	pkg string

	// events holds all test events for this package.
	events []gotest.Event

	// failedRefs tracks which test references have failed.
	failedRefs map[gotest.Reference]struct{}

	// resultEvent holds the final result event for each test.
	resultEvent map[gotest.Reference]gotest.Event

	// packageCoverage holds coverage output for package references.
	packageCoverage map[gotest.Reference]string

	// panic tracks which test references have panic output.
	panic map[gotest.Reference]bool

	// formatter converts test events to formatted output.
	formatter func(gotest.Event, bool) fmt.Stringer
}

// NewQuietPackage creates a handler for a single package in quiet mode.
func NewQuietPackage(writer io.Writer, config QuietPackageConfig, ref gotest.Reference) *QuietPackage {
	// quiet mode should never show state markers (=== RUN, === PAUSE, === CONT)
	config.ExecutionMarkers = output.ExecutionMarkersNone

	return &QuietPackage{
		writer:          writer,
		config:          config,
		pkg:             ref.Package,
		failedRefs:      make(map[gotest.Reference]struct{}),
		resultEvent:     make(map[gotest.Reference]gotest.Event),
		packageCoverage: make(map[gotest.Reference]string),
		panic:           make(map[gotest.Reference]bool),
		formatter: presenter.NewGoQuietEventFactory(
			presenter.GoEventConfig{
				Style:              style.NewGo(config.Color),
				IDE:                config.IDE,
				PackageNameWidth:   config.PackageNameWidth,
				StripPackagePrefix: "", // TODO: not wired up
			},
		).NewEvent,
	}
}

// Handle processes partybus events, routing test events to the handler.
func (h *QuietPackage) Handle(e partybus.Event) error {
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

// OnGoTestEvent processes test events for this package, tracking failures and
// rendering output when the package completes.
func (h *QuietPackage) OnGoTestEvent(e gotest.Event) error {
	if e.Reference.Package != h.pkg {
		return nil
	}

	isPkg := e.Reference.IsPackage()

	if output.HasFailedTestMarking(e.Output) {
		h.resultEvent[e.Reference] = e
	} else {
		h.events = append(h.events, e)
	}

	if output.HasPanicMarking(e.Output) {
		h.panic[e.Reference] = true
	}

	switch e.Action {
	case gotest.FailAction:
		h.failedRefs[e.Reference] = struct{}{}
		fallthrough
	case gotest.PassAction, gotest.SkipAction:
		if isPkg {
			h.render(h.writer)
			return handler.ErrPackageComplete
		}
	}
	return nil
}

// String returns buffered output for this package.
func (h *QuietPackage) String() string {
	sb := strings.Builder{}
	h.render(&sb)
	return sb.String()
}

// render writes the formatted output for this package, showing only failed tests
// and package conclusions.
func (h *QuietPackage) render(writer io.Writer) { //nolint:gocognit
	for _, e := range h.events {
		if !e.Reference.IsPackage() && !h.isFailedReference(e.Reference) {
			continue
		}

		switch e.Action {
		case gotest.RunAction:
			// replace with the eventual result
			resultEvent, ok := h.resultEvent[e.Reference]
			if !ok {
				// TODO, not great
				log.Warnf("no result found for test: %s", e.Reference)
				continue
			}

			if strings.TrimSpace(resultEvent.Output) == "" {
				continue
			}

			fmt.Fprint(writer, h.formatter(resultEvent, h.panic[e.Reference]).String())
		default:
			if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && h.config.HidePackagesWithNoTestFiles {
				continue
			}
			if output.HasFailedPackageTrailer(e.Output) {
				// skip the package FAIL line, this is redundant
				continue
			}
			if output.HasPackagePassMarking(e.Output) {
				// skip the package PASS line
				continue
			}
			if !output.ShouldShowStateMarker(e.Output, h.config.ExecutionMarkers) {
				// skip state markers based on config
				continue
			}
			if output.HasPackageCoverageMarking(e.Output) {
				// skip "coverage:" lines
				continue
			}
			if output.HasShuffleSeedMarking(e.Output) {
				// go echoes "-test.shuffle <seed>" once per package (all identical), drop it
				continue
			}
			if strings.TrimSpace(e.Output) == "" {
				continue
			}

			out := h.formatter(e, h.panic[e.Reference]).String()
			if out != "" {
				fmt.Fprint(writer, out)
			}
		}
	}
}

// isFailedReference checks if a test reference has failed.
func (h *QuietPackage) isFailedReference(ref gotest.Reference) bool {
	_, ok := h.failedRefs[ref]
	return ok
}
