package gostd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var ErrPackageComplete = fmt.Errorf("package output complete")

type PackageHandlerFactory func(gotest.Reference, io.Writer) handler.Handler

type packageHandler struct {
	writer      io.Writer
	runningPkgs map[string]handler.Handler
	factory     PackageHandlerFactory
}

func newPackageHandler(factory PackageHandlerFactory, writer io.Writer) handler.Handler {
	return &packageHandler{
		writer:      writer,
		runningPkgs: make(map[string]handler.Handler),
		factory:     factory,
	}
}

func (n *packageHandler) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		return n.OnGoTestEvent(goTestEvent)
	}
	return nil
}

func (n *packageHandler) OnGoTestEvent(goTestEvent gotest.Event) error {
	// buffer all package output until all package test results are in
	if n.runningPkgs[goTestEvent.Reference.Package] == nil {
		n.runningPkgs[goTestEvent.Reference.Package] = n.factory(goTestEvent.Reference, n.writer)
	}

	if err := n.runningPkgs[goTestEvent.Reference.Package].OnGoTestEvent(goTestEvent); err != nil {
		if errors.Is(err, ErrPackageComplete) {
			delete(n.runningPkgs, goTestEvent.Reference.Package)
		} else {
			return err
		}
	}
	return nil
}

func (n packageHandler) String() string {
	var pkgs []string
	for pkg := range n.runningPkgs {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	sb := strings.Builder{}
	for _, p := range pkgs {
		if n.runningPkgs[p] != nil {
			sb.WriteString(n.runningPkgs[p].String())
		}
	}
	return sb.String()
}
