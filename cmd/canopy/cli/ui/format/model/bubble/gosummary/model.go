package gosummary

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var _ tea.Model = (*Model)(nil)

type Model struct {
	config  presenter.GoSummaryConfig
	started bool
	runs    []gotest.Run
	ids     mapset.Set[uuid.UUID]
	common  state.Common
}

func NewModel(config presenter.GoSummaryConfig, common state.Common, runID uuid.UUID, runCfg gotest.RunnerConfig) *Model {
	run := gotest.NewRun(gotest.RunnerConfig{}) // we only need the cumulative state, not the run config
	run.Result = *gotest.NewResult(gotest.ResultConfig{
		TrackFailingOutput: true,
		TrackOtherOutput:   false,
	})
	run.ID = runID
	run.Config = runCfg
	return &Model{
		config: config,
		runs:   []gotest.Run{*run},
		ids:    mapset.NewSet[uuid.UUID](runID),
		common: common,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.common.OnMessage(msg)

	var cmd tea.Cmd
	switch msg := msg.(type) {
	case partybus.Event:

		switch msg.Type {
		case event.GoTestType:
			testEvent, err := parser.ParseGoTestType(msg)
			if err != nil {
				log.WithFields("error", err).Error("unable to parse go test event")
				panic("TODO")
			}

			if !m.config.CombineMultipleRuns && !m.ids.Contains(testEvent.RunID) {
				break
			}

			if !m.started {
				m.started = true
			}

			for i := range m.runs {
				if m.runs[i].ID == testEvent.RunID {
					m.runs[i].Result.Update(testEvent)
					break
				}
			}

		case event.GoTestRunType:
			runEvent, err := parser.ParseGoTestRunType(msg)
			if err != nil {
				log.WithFields("error", err).Error("unable to parse go test event")
				panic("TODO")
			}

			if (!m.config.CombineMultipleRuns && !m.ids.Contains(runEvent.ID)) || runEvent == nil {
				break
			}

			if !m.ids.Contains(runEvent.ID) {
				m.runs = append(m.runs, *runEvent)
				m.ids.Add(runEvent.ID)
			}
		}
	}

	return m, cmd
}

func (m Model) View() string {
	sb := strings.Builder{}
	m.config.RunningState = m.common.Spinner.View
	err := m.config.New(m.runs...).Present(&sb, &sb)
	if err != nil {
		// TODO
		panic(err)
	}
	return sb.String()
}
