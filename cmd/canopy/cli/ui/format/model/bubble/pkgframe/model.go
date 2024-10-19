package pkgframe

import (
	tea "github.com/charmbracelet/bubbletea"
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

type ModelFactory func(gotest.Event, tea.WindowSizeMsg) tea.Model

type Factory struct {
	pkgModelFactory ModelFactory
	seen            map[gotest.Reference]struct{}
	ws              tea.WindowSizeMsg
}

func NewFactory(pkgModelFactory ModelFactory) *Factory {
	return &Factory{
		pkgModelFactory: pkgModelFactory,
		seen:            make(map[gotest.Reference]struct{}),
	}
}

func (j *Factory) OnMessage(msg tea.Msg) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		j.ws = msg
	}
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

	if !gt.Reference.IsPackage() {
		return nil, nil
	}

	if _, ok := j.seen[gt.Reference]; ok {
		return nil, nil
	} else {
		// we only want to register that we have seen the package, not any indication of testing that will start
		// this is because we will only see no-test indications on the second event at the earliest
		if gt.Action == gotest.StartAction {
			return nil, nil
		}
		// this isn't a simple start action! this is odd (maybe a panic in testing?) let's handle it.
	}

	j.seen[gt.Reference] = struct{}{}

	pkgMod := j.pkgModelFactory(gt, j.ws)
	if pkgMod != nil {
		return []tea.Model{pkgMod}, nil
	}
	return nil, nil
}

type Model struct {
	pkgRef     gotest.Reference
	action     gotest.Action
	frame      frame.Frame
	testsSeen  map[gotest.Reference]struct{}
	rowFactory ModelFactory
	ws         tea.WindowSizeMsg
}

func NewPackageModel(ref gotest.Reference, ws tea.WindowSizeMsg, rowFactory ModelFactory) *Model {
	return &Model{
		pkgRef:     ref.PackageRef(),
		frame:      *frame.New(),
		testsSeen:  make(map[gotest.Reference]struct{}),
		rowFactory: rowFactory,
		ws:         ws,
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
	//if !j.update(msg) {
	//	return j, nil
	//}
	j.update(msg)

	newFrame, cmd := j.frame.Update(msg)
	j.frame = newFrame.(frame.Frame)

	return j, cmd
}

func (j *Model) update(msg tea.Msg) bool {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		j.ws = msg
	}

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

		testRowModel := j.rowFactory(gt, j.ws)

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
