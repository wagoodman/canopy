package jestsummary

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
)

var _ bubbly.EventHandler = (*Factory)(nil)

type Factory struct {
	config presenter.JestTestResultSummaryConfig
	seen   map[uuid.UUID]struct{}
}

func NewFactory(cfg presenter.JestTestResultSummaryConfig) *Factory {
	return &Factory{
		config: cfg,
		seen:   make(map[uuid.UUID]struct{}),
	}
}

func (f Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType, event.GoTestRunType}
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

	if _, ok := f.seen[gt.RunID]; ok {
		return nil, nil
	}

	f.seen[gt.RunID] = struct{}{}
	return []tea.Model{NewModel(f.config, gt.RunID)}, nil
}
