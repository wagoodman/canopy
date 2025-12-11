package handler

import (
	"bytes"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

// multiPackageHandler manages handlers for multiple packages, creating a new handler
// for each package as events arrive. Maintains package ordering for consistent output.
type multiPackageHandler struct {
	// order tracks the sequence in which packages were first seen.
	order []string

	// packages maps package names to their handlers.
	packages map[string]Handler

	// factory creates new handlers for each package.
	factory PackageHandlerFactory

	// writer is the buffer where all package output is written.
	writer *bytes.Buffer
}

// NewMultiPackageHandler creates a handler that manages multiple package handlers,
// creating them on-demand as test events arrive for different packages.
func NewMultiPackageHandler(factory PackageHandlerFactory) Handler {
	return &multiPackageHandler{
		packages: make(map[string]Handler),
		factory:  factory,
		writer:   &bytes.Buffer{},
	}
}

// Handle processes partybus events, routing them to the appropriate package handler.
func (m *multiPackageHandler) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		return m.OnGoTestEvent(goTestEvent)
	}
	return nil
}

// OnGoTestEvent processes a test event, creating a new package handler if needed
// and forwarding the event to the appropriate handler.
func (m *multiPackageHandler) OnGoTestEvent(event gotest.Event) error {
	p := event.Reference.Package
	if _, ok := m.packages[p]; !ok {
		m.packages[p] = m.factory(event.Reference, m.writer)
		m.order = append(m.order, p)
	}

	return m.packages[p].OnGoTestEvent(event)
}

// String returns all buffered output from all package handlers in the order
// packages were first seen.
func (m multiPackageHandler) String() string {
	sb := strings.Builder{}
	for _, pkg := range m.order {
		sb.WriteString(m.packages[pkg].String())
	}
	return sb.String()
}
