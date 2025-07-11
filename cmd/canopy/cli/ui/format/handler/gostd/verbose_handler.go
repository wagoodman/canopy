package gostd

import (
	"fmt"
	"io"
	"strings"

	"github.com/lindell/go-ordered-set/orderedset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal"
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
	_ handler.Handler  = (*verboseHandler)(nil)
	_ partybus.Handler = (*verboseHandler)(nil)
)

type PackageConfig struct {
	Color                       bool
	PackageNameWidth            int
	IDE                         ide.Context
	HidePackagesWithNoTestFiles bool // TODO: not used??
}

type verboseHandler struct {
	writer    io.Writer
	result    *gotest.Result
	packages  *orderedset.OrderedSet[gotest.Reference]
	panic     map[gotest.Reference]bool
	formatter func(gotest.Event, bool) fmt.Stringer
}

func NewVerboseHandler(writer io.Writer, config PackageConfig) handler.Handler {
	return &verboseHandler{
		writer:   writer,
		result:   gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages: orderedset.New[gotest.Reference](),
		panic:    make(map[gotest.Reference]bool),
		formatter: presenter.NewGoPPVerboseEventFactory(
			style.NewGo(config.Color),
			config.IDE,
			false,
			config.PackageNameWidth,
		).NewEvent,
	}
}

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

func (h *verboseHandler) OnGoTestEvent(e gotest.Event) error {
	h.result.Update(e)
	if e.Reference.IsPackage() {
		h.packages.Add(e.Reference)
	}

	if output.HasPanicMarking(e.Output) {
		h.panic[e.Reference] = true
	}

	switch e.Action {
	// TODO: realtime output of test output... finally output the test conclusions

	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		switch {
		case e.Reference.IsPackage():
			// print final "FAIL" or "PASS" line for the package
			switch e.Action {
			case gotest.PassAction:
				e.Output = "PASS"
			case gotest.FailAction:
				e.Output = "FAIL"
			case gotest.SkipAction:
				e.Output = "SKIP"
			}
			e.Action = gotest.OutputAction
			fmtr := h.formatter(e, h.panic[e.Reference])
			fmt.Fprint(h.writer, fmtr.String())

		case !e.Reference.IsSubTest():
			h.outputTest(
				e.Reference,
				true,
				func(e gotest.Event) bool {
					return output.HasConclusionMarking(e.Output)
				},
			)
		}
	case gotest.OutputAction:
		if !output.HasConclusionMarking(e.Output) {
			fmtr := h.formatter(e, h.panic[e.Reference])
			if strings.TrimSpace(e.Output) != "" {
				fmt.Fprint(h.writer, fmtr.String())
			}
		}
	}

	return nil
}

func (h *verboseHandler) outputTest(testRef gotest.Reference, indent bool, include func(gotest.Event) bool) {
	outputEvents := h.getEvents(testRef, include)

	for _, e := range outputEvents {
		writer := h.writer
		if indent {
			writer = internal.NewIndentWriter(writer, e.Reference)
		}
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(writer, fmtr.String())
		}
	}
}

func (h *verboseHandler) getEvents(testRef gotest.Reference, include func(gotest.Event) bool) []gotest.Event {
	outputEvents := filterEvents(h.result.ReferenceEvents(testRef), include)

	for _, childRef := range h.result.Children(testRef) {
		outputEvents = append(outputEvents, h.getEvents(childRef, include)...)
	}

	return outputEvents
}

func filterEvents(events []gotest.Event, include func(gotest.Event) bool) []gotest.Event {
	var filtered []gotest.Event
	for _, e := range events {
		if include(e) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (h *verboseHandler) String() string {
	return ""
}
