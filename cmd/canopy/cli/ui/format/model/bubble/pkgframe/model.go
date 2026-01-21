package pkgframe

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
)

type Model struct {
	pkgRef     gotest.Reference
	action     gotest.Action
	frame      *frame.Frame
	testsSeen  map[gotest.Reference]struct{}
	rowFactory ModelFactory
	common     state.Common
}

func NewPackageModel(ref gotest.Reference, common state.Common, rowFactory ModelFactory) *Model {
	return &Model{
		pkgRef:     ref.PackageRef(),
		frame:      frame.New(),
		testsSeen:  make(map[gotest.Reference]struct{}),
		rowFactory: rowFactory,
		common:     common,
	}
}

func (m Model) Init() tea.Cmd {
	return m.frame.Init()
}

func (m Model) ShouldImprint() bool {
	switch m.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.common.OnMessage(msg)

	m.update(msg)

	newFrame, cmd := m.frame.Update(msg)
	m.frame = newFrame.(*frame.Frame)

	return m, cmd
}

func (m *Model) update(msg tea.Msg) bool {
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

	if gt.Reference.Package != m.pkgRef.Package {
		return false
	}

	actionTaken := false
	if _, ok := m.testsSeen[gt.Reference]; !ok {
		m.testsSeen[gt.Reference] = struct{}{}

		testRowModel := m.rowFactory(gt, m.common)

		if testRowModel != nil {
			m.frame.AppendModel(testRowModel)
			actionTaken = true
		}
	}

	if gt.Reference == m.pkgRef {
		switch gt.Action {
		case gotest.RunAction, gotest.PassAction, gotest.FailAction, gotest.SkipAction, gotest.StartAction:
			m.action = gt.Action
			actionTaken = true
		}
	}
	return actionTaken
}

func (m Model) View() string {
	return m.frame.View()
}
