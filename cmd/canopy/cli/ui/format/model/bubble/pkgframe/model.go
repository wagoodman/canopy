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

type TestRowFactory func(gotest.Reference, tea.WindowSizeMsg) tea.Model

type Factory struct {
	testRowFactory TestRowFactory
	seen           map[gotest.Reference]struct{}
	ws             tea.WindowSizeMsg
}

func NewFactory(testRowFactory TestRowFactory) *Factory {
	return &Factory{
		testRowFactory: testRowFactory,
		seen:           make(map[gotest.Reference]struct{}),
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
	}

	j.seen[gt.Reference] = struct{}{}
	return []tea.Model{NewModel(gt.Reference, j.ws, j.testRowFactory)}, nil
}

type Model struct {
	pkgRef         gotest.Reference
	action         gotest.Action
	frame          frame.Frame
	testsSeen      map[gotest.Reference]struct{}
	testRowFactory TestRowFactory
	ws             tea.WindowSizeMsg
}

func NewModel(ref gotest.Reference, ws tea.WindowSizeMsg, testRowFactory TestRowFactory) *Model {
	return &Model{
		pkgRef:         ref,
		frame:          *frame.New(),
		testsSeen:      make(map[gotest.Reference]struct{}),
		testRowFactory: testRowFactory,
		ws:             ws,
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
	j.update(msg)

	newFrame, cmd := j.frame.Update(msg)
	j.frame = newFrame.(frame.Frame)

	return j, cmd
}

func (j *Model) update(msg tea.Msg) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		j.ws = msg
	}

	e, ok := msg.(partybus.Event)
	if !ok {
		return
	}

	if e.Type != event.GoTestType {
		return
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return
	}

	if gt.Reference.Package != j.pkgRef.Package {
		return
	}

	if _, ok := j.testsSeen[gt.Reference]; !ok {
		j.testsSeen[gt.Reference] = struct{}{}

		testRowModel := j.testRowFactory(gt.Reference, j.ws)

		j.frame.AppendModel(testRowModel)
	}

	if gt.Reference == j.pkgRef {
		switch gt.Action {
		case gotest.RunAction, gotest.PassAction, gotest.FailAction, gotest.SkipAction, gotest.StartAction:
			j.action = gt.Action
		}
	}
}

func (j Model) View() string {
	return j.frame.View()
}
