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

type multiPackageHandler struct {
	order    []string
	packages map[string]Handler
	factory  PackageHandlerFactory
	writer   *bytes.Buffer
}

func NewMultiPackageHandler(factory PackageHandlerFactory) Handler {
	return &multiPackageHandler{
		packages: make(map[string]Handler),
		factory:  factory,
		writer:   &bytes.Buffer{},
	}
}

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

func (m *multiPackageHandler) OnGoTestEvent(event gotest.Event) error {
	p := event.Reference.Package
	if _, ok := m.packages[p]; !ok {
		m.packages[p] = m.factory(event.Reference, m.writer)
		m.order = append(m.order, p)
	}

	return m.packages[p].OnGoTestEvent(event)
}

func (m multiPackageHandler) String() string {
	sb := strings.Builder{}
	for _, pkg := range m.order {
		sb.WriteString(m.packages[pkg].String())
	}
	return sb.String()
}
