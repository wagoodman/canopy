package gopp

import (
	"fmt"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*QuietPackage)(nil)
	_ partybus.Handler = (*QuietPackage)(nil)
	_ fmt.Stringer     = (*QuietPackage)(nil)
)

type QuietPackageConfig struct {
	Color                       bool
	PackageNameWidth            int
	IDE                         ide.Context
	HidePackagesWithNoTestFiles bool
}

func NewQuietHandler(writer io.Writer, config QuietPackageConfig) handler.Handler {
	return handler.NewPackageHandler(
		func(ref gotest.Reference, writer io.Writer) handler.Handler {
			return NewQuietPackage(writer, config, ref)
		}, writer)
}

type QuietPackage struct {
	writer          io.Writer
	config          QuietPackageConfig
	pkg             string
	events          []gotest.Event
	failedRefs      map[gotest.Reference]struct{}
	resultEvent     map[gotest.Reference]gotest.Event
	packageCoverage map[gotest.Reference]string
	panic           map[gotest.Reference]bool

	formatter func(gotest.Event, bool) fmt.Stringer
}

func NewQuietPackage(writer io.Writer, config QuietPackageConfig, ref gotest.Reference) *QuietPackage {
	return &QuietPackage{
		writer:          writer,
		config:          config,
		pkg:             ref.Package,
		failedRefs:      make(map[gotest.Reference]struct{}),
		resultEvent:     make(map[gotest.Reference]gotest.Event),
		packageCoverage: make(map[gotest.Reference]string),
		panic:           make(map[gotest.Reference]bool),
		formatter: presenter.NewGoPPQuietEventFactory(
			style.NewGo(config.Color),
			config.IDE,
			config.PackageNameWidth,
		).NewEvent,
	}
}

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

func (h *QuietPackage) String() string {
	sb := strings.Builder{}
	h.render(&sb)
	return sb.String()
}

func (h *QuietPackage) render(writer io.Writer) { //nolint: gocognit
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

			fmt.Fprint(writer, h.formatter(resultEvent, false).String())
		default:
			if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && h.config.HidePackagesWithNoTestFiles {
				continue
			}
			if output.HasRunMarking(e.Output) || output.HasPassMarking(e.Output) {
				// skip the run line
				continue
			}
			if output.HasPackageCoverageMarking(e.Output) {
				// skip "coverage:" lines
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

func (h *QuietPackage) isFailedReference(ref gotest.Reference) bool {
	_, ok := h.failedRefs[ref]
	return ok
}
