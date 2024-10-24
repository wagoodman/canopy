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
	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ bubbly.EventHandler      = (*Factory)(nil)
	_ bubbly.MessageListener   = (*Factory)(nil)
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
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

func (j *Factory) OnMessage(msg tea.Msg) {
	j.config.Common.OnMessage(msg)
}

func (j Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType}
}

func (j Factory) Handle(e partybus.Event) ([]tea.Model, tea.Cmd) {
	if e.Type != event.GoTestType {
		return nil, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return nil, nil
	}

	if j.config.ShowPackagesMissingTests {
		return j.handlePackageEvent(gt)
	}
	return j.handleAnyTestEvent(gt, e)
}

func (j *Factory) handleAnyTestEvent(gt gotest.Event, e partybus.Event) ([]tea.Model, tea.Cmd) {
	hasSeenPkg := j.hasSeenPackage(gt.Reference)
	isPkg := gt.Reference.IsPackage()
	startPkgEvent, hasSeenStartPkgEvent := j.startPkgEvent[gt.Reference.Package]

	if isPkg {
		if hasSeenPkg {
			return nil, nil
		}

		if hasSeenStartPkgEvent {
			// we've seen the startPkgEvent package event already, and this is the second event!
			// make a new model, pass all events... only if the second event is NOT a no-test indication
			delete(j.startPkgEvent, gt.Reference.Package)
			if gt.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) {
				return nil, nil
			}

			return j.newModel(startPkgEvent, e)
		}

		switch gt.Action {
		case gotest.StartAction:
			// we'll check the second event to see if it is a no-test indication from a package ref...
			// but a pure package ref is not guaranteed either.
			j.startPkgEvent[gt.Reference.Package] = gt
			return nil, nil
		case gotest.SkipAction:
			// we're never going to show this package
			j.markPackageAsSeen(gt.Reference)
			return nil, nil
		}
		// this isn't a start event! this is odd (maybe a panic in testing?) let's handle it.
		return j.newModel(gt, e)
	}

	if hasSeenStartPkgEvent {
		// we've seen the startPkgEvent package event already, and this is the second event!
		// make a new model, pass all events... only if the second event is NOT a no-test indication
		delete(j.startPkgEvent, gt.Reference.Package)
		if gt.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) {
			return nil, nil
		}

		return j.newModel(startPkgEvent, e)
	}

	return nil, nil
}

func (j *Factory) handlePackageEvent(gt gotest.Event) ([]tea.Model, tea.Cmd) {
	if !gt.Reference.IsPackage() {
		return nil, nil
	}

	if j.hasSeenPackage(gt.Reference) {
		return nil, nil
	}

	return j.newModel(gt)
}

func (j Factory) hasSeenPackage(ref gotest.Reference) bool {
	_, ok := j.seenPkg[ref.Package]
	return ok
}

func (j *Factory) newModel(gt gotest.Event, es ...partybus.Event) ([]tea.Model, tea.Cmd) {
	j.markPackageAsSeen(gt.Reference)

	pkgMod := j.pkgModelFactory(gt, j.config.Common)

	if pkgMod == nil {
		return nil, nil
	}

	var cmd tea.Cmd
	for _, e := range es {
		newPkgMod, newCmd := pkgMod.Update(e)
		cmd = tea.Batch(cmd, newCmd)
		pkgMod = newPkgMod
	}

	return []tea.Model{pkgMod}, cmd
}

func (j *Factory) markPackageAsSeen(ref gotest.Reference) {
	j.seenPkg[ref.Package] = struct{}{}
}

type Model struct {
	pkgRef     gotest.Reference
	action     gotest.Action
	frame      frame.Frame
	testsSeen  map[gotest.Reference]struct{}
	rowFactory ModelFactory
	common     state.Common
}

func NewPackageModel(ref gotest.Reference, common state.Common, rowFactory ModelFactory) *Model {
	return &Model{
		pkgRef:     ref.PackageRef(),
		frame:      *frame.New(),
		testsSeen:  make(map[gotest.Reference]struct{}),
		rowFactory: rowFactory,
		common:     common,
	}
}

func (j Model) Init() tea.Cmd {
	return j.frame.Init()
}

func (j Model) ShouldImprint() bool {
	switch j.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (j Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	j.common.OnMessage(msg)

	j.update(msg)

	newFrame, cmd := j.frame.Update(msg)
	j.frame = newFrame.(frame.Frame)

	return j, cmd
}

func (j *Model) update(msg tea.Msg) bool {
	e, ok := msg.(partybus.Event)
	if !ok {
		return false
	}

	if e.Type != event.GoTestType {
		return false
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return false
	}

	if gt.Reference.Package != j.pkgRef.Package {
		return false
	}

	actionTaken := false
	if _, ok := j.testsSeen[gt.Reference]; !ok {
		j.testsSeen[gt.Reference] = struct{}{}

		testRowModel := j.rowFactory(gt, j.common)

		if testRowModel != nil {
			j.frame.AppendModel(testRowModel)
			actionTaken = true
		}
	}

	if gt.Reference == j.pkgRef {
		switch gt.Action {
		case gotest.RunAction, gotest.PassAction, gotest.FailAction, gotest.SkipAction, gotest.StartAction:
			j.action = gt.Action
			actionTaken = true
		}
	}
	return actionTaken
}

func (j Model) View() string {
	return j.frame.View()
}
