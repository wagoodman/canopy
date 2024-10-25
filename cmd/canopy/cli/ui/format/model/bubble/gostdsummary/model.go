package gostdsummary

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
)

var (
	_ bubbly.EventHandler = (*Factory)(nil)
	_ tea.Model           = (*Model)(nil)
)

type Factory struct {
	config presenter.GoStdTestResultSummaryConfig
	seen   map[uuid.UUID]struct{}
	common state.Common
}

func NewFactory(cfg presenter.GoStdTestResultSummaryConfig, common state.Common) *Factory {
	return &Factory{
		config: cfg,
		seen:   make(map[uuid.UUID]struct{}),
		common: common,
	}
}

func (j Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType, event.GoTestRunType}
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

	if _, ok := j.seen[gt.RunID]; ok {
		return nil, nil
	}

	j.seen[gt.RunID] = struct{}{}
	return []tea.Model{NewModel(j.config, j.common, gt.RunID)}, nil
}

type Model struct {
	config  presenter.GoStdTestResultSummaryConfig
	started bool
	run     gotest.Run
	common  state.Common
}

func NewModel(config presenter.GoStdTestResultSummaryConfig, common state.Common, runID uuid.UUID) *Model {
	run := gotest.NewRun(gotest.RunnerConfig{}) // we only need the cumulative state, not the run config
	run.Result = *gotest.NewResult(gotest.ResultConfig{
		TrackFailingOutput: true,
		TrackOtherOutput:   false,
	})
	run.ID = runID
	return &Model{
		config: config,
		run:    *run,
		common: common,
	}
}

func (j Model) Init() tea.Cmd {
	return nil
}

func (j Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	j.common.OnMessage(msg)

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

			if j.run.ID != testEvent.RunID {
				break
			}

			if !j.started {
				j.started = true
			}

			j.run.Result.Update(testEvent)

		case event.GoTestRunType:
			runEvent, err := parser.ParseGoTestRunType(msg)
			if err != nil {
				log.WithFields("error", err).Error("unable to parse go test event")
				panic("TODO")
			}

			if j.run.ID != runEvent.ID {
				break
			}

			j.run = *runEvent
		}
	}

	return j, cmd
}

func (j Model) View() string {
	sb := strings.Builder{}
	j.config.RunningState = j.common.Spinner.View
	err := j.config.New(j.run).Present(&sb, &sb)
	if err != nil {
		// TODO
		panic(err)
	}
	return sb.String()
}
