package gostd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/lindell/go-ordered-set/orderedset"
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

type quietHandler struct {
	writer    io.Writer
	result    *gotest.Result
	packages  *orderedset.OrderedSet[gotest.Reference]
	panic     map[gotest.Reference]bool
	formatter func(gotest.Event, bool) fmt.Stringer
}

func NewQuietHandler(writer io.Writer, config PackageConfig) handler.Handler {
	return &quietHandler{
		writer:   writer,
		result:   gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true}),
		packages: orderedset.New[gotest.Reference](),
		panic:    make(map[gotest.Reference]bool),
		formatter: presenter.NewGoPPQuietEventFactory(
			style.NewGo(config.Color),
			config.IDE,
			config.PackageNameWidth,
		).NewEvent,
	}
}

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

func (h *quietHandler) OnGoTestEvent(e gotest.Event) error {
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

func (h *quietHandler) render() {
	// only render packages that are done, and render them in the order they were started
	// this is the reason why we cannot use a package handler (since order of packages is important, independent of the order of completion)
	pkgs := h.packages.Values()
	for len(pkgs) > 0 {
		pkgRef := pkgs[0]
		action := h.result.ReferenceConclusiveAction(pkgRef)

		if !action.Completed() {
			// this package isn't done yet, so we can't output anything after it
			break
		}

		h.outputPackage(
			pkgRef,
			h.hasFailure,
			func(e gotest.Event) bool {
				return !output.HasStateMarking(e.Output)
			},
		)

		h.packages.Delete(pkgRef)
		pkgs = h.packages.Values()
	}
}

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

func (h *quietHandler) outputPackage(pkgRef gotest.Reference, include func(gotest.Reference) bool, render func(gotest.Event) bool) {
	for _, testRef := range h.result.Children(pkgRef) {
		if include(testRef) {
			h.outputTest(testRef, include, render)
		}
	}
	// output package conclusions
	outputEvents := h.result.ReferenceEvents(pkgRef)
	for _, e := range outputEvents {
		fmtr := h.formatter(e, h.panic[e.Reference])
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprint(h.writer, fmtr.String())
		}
	}

	// print final "FAIL" or "PASS" line for the package
	e := h.result.ReferenceConclusion(pkgRef)
	if e != nil {
		switch e.Action {
		case gotest.PassAction:
			e.Output = "PASS"
		case gotest.FailAction:
			e.Output = "FAIL"
		case gotest.SkipAction:
			e.Output = "SKIP"
		}
		e.Action = gotest.OutputAction
		fmtr := h.formatter(*e, h.panic[e.Reference])
		fmt.Fprint(h.writer, fmtr.String())
	}
}

func (h *quietHandler) outputTest(testRef gotest.Reference, include func(gotest.Reference) bool, render func(gotest.Event) bool) {
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
			fmt.Fprint(h.writer, fmtr.String())
		}
	}
}

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

func (h *quietHandler) String() string {
	return ""
}

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
