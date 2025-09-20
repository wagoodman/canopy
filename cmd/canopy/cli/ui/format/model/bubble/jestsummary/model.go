package jestsummary

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var _ tea.Model = (*Model)(nil)

type Model struct {
	config  presenter.JestTestResultSummaryConfig
	started bool
	run     gotest.Run
	timer   Timer
}

func NewModel(config presenter.JestTestResultSummaryConfig, runID uuid.UUID) *Model {
	run := gotest.NewRun(gotest.RunnerConfig{}) // we only need the cumulative state, not the run config
	run.Result = *gotest.NewResult(gotest.ResultConfig{
		TrackFailingOutput: true,
		TrackOtherOutput:   false,
	})
	run.ID = runID
	return &Model{
		timer:  newTimer(100 * time.Millisecond),
		config: config,
		run:    *run,
	}
}

func (m Model) Init() tea.Cmd {
	return m.timer.tick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case timerTickMessage:
		if msg.id != m.timer.id {
			break
		}

		cmd = m.timer.tick()

	case partybus.Event:

		switch msg.Type {
		case event.GoTestType:
			testEvent, err := parser.ParseGoTestType(msg)
			if err != nil {
				log.WithFields("error", err).Error("unable to parse go test event")
				panic("TODO")
			}

			if m.run.ID != testEvent.RunID {
				break
			}

			if !m.started {
				m.started = true
			}

			m.run.Result.Update(testEvent)

		case event.GoTestRunType:
			runEvent, err := parser.ParseGoTestRunType(msg)
			if err != nil {
				log.WithFields("error", err).Error("unable to parse go test event")
				panic("TODO")
			}

			if m.run.ID != runEvent.ID {
				break
			}

			m.run = *runEvent
		}
	}

	return m, cmd
}

func (m Model) View() string {
	sb := strings.Builder{}
	err := m.config.New(m.run).Present(&sb, &sb)
	if err != nil {
		// TODO
		panic(err)
	}
	return sb.String()
}
