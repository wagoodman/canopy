package gosummary

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var _ tea.Model = (*Model)(nil)

type Factory struct {
	config presenter.GoSummaryConfig
	seen   map[uuid.UUID]struct{}
	common state.Common
}

func NewFactory(cfg presenter.GoSummaryConfig, common state.Common) *Factory {
	return &Factory{
		config: cfg,
		seen:   make(map[uuid.UUID]struct{}),
		common: common,
	}
}

func (f Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType, event.GoTestRunType, event.GoTestRunRequestType}
}

func (f Factory) Handle(e partybus.Event) ([]tea.Model, tea.Cmd) {
	if e.Type != event.GoTestRunRequestType {
		return nil, nil
	}

	cfg, id, err := parser.ParseGoTestRunRequestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return nil, nil
	}

	idVal := *id

	if _, ok := f.seen[idVal]; ok {
		return nil, nil
	}

	if f.config.CombineMultipleRuns && len(f.seen) > 0 {
		// if we are combining multiple runs, we only want to show the first run
		// so we skip this one if we've already seen a run
		return nil, nil
	}

	f.seen[idVal] = struct{}{}
	return []tea.Model{NewModel(f.config, f.common, idVal, *cfg)}, nil
}
