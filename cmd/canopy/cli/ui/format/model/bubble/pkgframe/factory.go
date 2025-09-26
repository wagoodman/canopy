package pkgframe

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
)

var (
	_ bubbly.EventHandler    = (*Factory)(nil)
	_ bubbly.MessageListener = (*Factory)(nil)
)

type ModelFactory func(gotest.Event, state.Common) tea.Model

type FactoryConfig struct {
	ShowPackagesMissingTests bool
	Common                   state.Common
}

type Factory struct {
	config          FactoryConfig
	pkgModelFactory ModelFactory
	seenPkg         map[string]struct{}
	startPkgEvent   map[string]gotest.Event
}

func NewFactory(pkgModelFactory ModelFactory, cfg FactoryConfig) *Factory {
	return &Factory{
		config:          cfg,
		pkgModelFactory: pkgModelFactory,
		seenPkg:         make(map[string]struct{}),
		startPkgEvent:   make(map[string]gotest.Event),
	}
}

func (f *Factory) OnMessage(msg tea.Msg) {
	f.config.Common.OnMessage(msg)
}

func (f Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType}
}

func (f Factory) Handle(e partybus.Event) ([]tea.Model, tea.Cmd) {
	if e.Type != event.GoTestType {
		return nil, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return nil, nil
	}

	if f.config.ShowPackagesMissingTests {
		return f.handlePackageEvent(gt)
	}
	return f.handleAnyTestEvent(gt)
}

func (f *Factory) handleAnyTestEvent(gt gotest.Event) ([]tea.Model, tea.Cmd) {
	hasSeenPkg := f.hasSeenPackage(gt.Reference)
	isPkg := gt.Reference.IsPackage()
	startPkgEvent, hasSeenStartPkgEvent := f.startPkgEvent[gt.Reference.Package]

	if isPkg {
		if hasSeenPkg {
			return nil, nil
		}

		if hasSeenStartPkgEvent {
			// we've seen the startPkgEvent package event already, and this is the second event!
			// make a new model, pass all events... only if the second event is NOT a no-test indication
			delete(f.startPkgEvent, gt.Reference.Package)
			if gt.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) {
				return nil, nil
			}

			return f.newModel(startPkgEvent)
		}

		switch gt.Action {
		case gotest.StartAction:
			// we'll check the second event to see if it is a no-test indication from a package ref...
			// but a pure package ref is not guaranteed either.
			f.startPkgEvent[gt.Reference.Package] = gt
			return nil, nil
		case gotest.SkipAction:
			// we're never going to show this package
			f.markPackageAsSeen(gt.Reference)
			return nil, nil
		}
		// this isn't a start event! this is odd (maybe a panic in testing?) let's handle it.
		return f.newModel(gt)
	}

	if hasSeenStartPkgEvent {
		// we've seen the startPkgEvent package event already, and this is the second event!
		// make a new model, pass all events... only if the second event is NOT a no-test indication
		delete(f.startPkgEvent, gt.Reference.Package)
		if gt.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) {
			return nil, nil
		}

		return f.newModel(startPkgEvent)
	}

	return nil, nil
}

func (f *Factory) handlePackageEvent(gt gotest.Event) ([]tea.Model, tea.Cmd) {
	if !gt.Reference.IsPackage() {
		return nil, nil
	}

	if f.hasSeenPackage(gt.Reference) {
		return nil, nil
	}

	return f.newModel(gt)
}

func (f Factory) hasSeenPackage(ref gotest.Reference) bool {
	_, ok := f.seenPkg[ref.Package]
	return ok
}

func (f *Factory) newModel(gt gotest.Event) ([]tea.Model, tea.Cmd) {
	f.markPackageAsSeen(gt.Reference)

	pkgMod := f.pkgModelFactory(gt, f.config.Common)

	if pkgMod == nil {
		return nil, nil
	}

	return []tea.Model{pkgMod}, nil
}

func (f *Factory) markPackageAsSeen(ref gotest.Reference) {
	f.seenPkg[ref.Package] = struct{}{}
}
