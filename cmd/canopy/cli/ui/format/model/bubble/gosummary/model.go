package gosummary

import (
	"strings"
	"time"

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
	config   presenter.GoSummaryConfig
	started  bool
	canceled bool
	runs     []gotest.Run
	ids      mapset.Set[uuid.UUID]
	// pending holds the requested runs that have not published a run-end event yet
	pending mapset.Set[uuid.UUID]
	common  state.Common

	// wall clock marks for the footer timer (see the timing model in presenter/go_summary.go). startedAt stands in
	// for launch: the run request is published right after canopy starts the go test process, and this model is
	// created when that request arrives. endedAt is when the last pending run-end event arrived.
	startedAt time.Time
	endedAt   time.Time
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
		config:    config,
		runs:      []gotest.Run{*run},
		ids:       mapset.NewSet[uuid.UUID](runID),
		pending:   mapset.NewSet[uuid.UUID](runID),
		common:    common,
		startedAt: time.Now(),
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
			m.handleGoTestEvent(msg)
		case event.GoTestRunType:
			m.handleGoTestRunEvent(msg)
		case event.GoTestRunRequestType:
			m.handleGoTestRunRequestEvent(msg)
		}
	}

	return m, cmd
}

// handleGoTestEvent processes GoTestType events, updating test results for matching runs
func (m *Model) handleGoTestEvent(msg partybus.Event) {
	testEvent, err := parser.ParseGoTestType(msg)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		panic("TODO")
	}

	if !m.shouldProcessTestEvent(testEvent) {
		return
	}

	if !m.started {
		m.started = true
	}

	m.updateRunResult(testEvent)
}

// handleGoTestRunEvent processes GoTestRunType events, adding new test runs
func (m *Model) handleGoTestRunEvent(msg partybus.Event) {
	runEvent, err := parser.ParseGoTestRunType(msg)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		panic("TODO")
	}

	if !m.shouldProcessRunEvent(runEvent) {
		return
	}

	// the run-end event is the only reliable signal that a run has concluded
	m.pending.Remove(runEvent.ID)
	if m.pending.Cardinality() == 0 && m.endedAt.IsZero() {
		m.endedAt = time.Now()
	}

	// the run-end event carries the final run state, including whether it was interrupted
	if runEvent.Canceled {
		m.canceled = true
	}

	if !m.ids.Contains(runEvent.ID) {
		m.runs = append(m.runs, *runEvent)
		m.ids.Add(runEvent.ID)
		return
	}

	// a run this model already tracks was built up from its test events, which don't carry coverage. That is only
	// calculated once the run is over, so take it from the run-end event.
	if cov, ok := runEvent.Result.Coverage(); ok {
		for i := range m.runs {
			if m.runs[i].ID == runEvent.ID {
				m.runs[i].Result.SetCoverage(&cov)
				break
			}
		}
	}
}

// handleGoTestRunRequestEvent tracks runs requested after this model was created, so a combined summary
// keeps running until every run has concluded (not just the first one)
func (m *Model) handleGoTestRunRequestEvent(msg partybus.Event) {
	if !m.config.CombineMultipleRuns {
		return
	}

	_, id, err := parser.ParseGoTestRunRequestType(msg)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test run request event")
		return
	}

	m.pending.Add(*id)
}

// shouldProcessTestEvent determines if a test event should be processed based on configuration
func (m *Model) shouldProcessTestEvent(testEvent gotest.Event) bool {
	return m.config.CombineMultipleRuns || m.ids.Contains(testEvent.RunID)
}

// shouldProcessRunEvent determines if a run event should be processed based on configuration
func (m *Model) shouldProcessRunEvent(runEvent *gotest.Run) bool {
	if runEvent == nil {
		return false
	}
	return m.config.CombineMultipleRuns || m.ids.Contains(runEvent.ID)
}

// updateRunResult finds the matching run and updates its result with the test event
func (m *Model) updateRunResult(testEvent gotest.Event) {
	for i := range m.runs {
		if m.runs[i].ID == testEvent.RunID {
			m.runs[i].Result.Update(testEvent)
			break
		}
	}
}

func (m Model) View() string {
	sb := strings.Builder{}
	m.config.RunningState = m.common.Spinner.View
	m.config.Window = m.common.Window
	// an interrupt keypress (tracked on common) or a canceled run-end event both mean the results are incomplete
	m.config.Canceled = m.canceled || m.common.Canceled
	m.config.Running = m.pending.Cardinality() > 0
	m.config.StartedAt = m.startedAt
	m.config.EndedAt = m.endedAt
	err := m.config.New(m.runs...).Present(&sb, &sb)
	if err != nil {
		// TODO
		panic(err)
	}
	return sb.String()
}
