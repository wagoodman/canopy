package dottestrow

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
)

var (
	_ bubbly.EventHandler    = (*Factory)(nil)
	_ bubbly.MessageListener = (*Factory)(nil)
)

type Config struct {
	Color                  bool
	ShowPackages           bool
	KeepFailedTestOutput   bool
	NestNonPackages        bool
	ExpireOnCompletion     bool
	DieOnCompletion        bool
	ShowIntermediateOutput bool
	Style                  *style.Dot
}

type Factory struct {
	config Config
	seen   map[gotest.Reference]struct{}
	common state.Common
}

func NewFactory(config Config) *Factory {
	return &Factory{
		config: config,
		seen:   make(map[gotest.Reference]struct{}),
	}
}

func (f *Factory) OnMessage(msg tea.Msg) {
	f.common.OnMessage(msg)
}

func (f Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType}
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

	if !f.config.ShowPackages && gt.Reference.IsPackage() {
		return nil, nil
	}

	if _, ok := f.seen[gt.Reference]; ok {
		return nil, nil
	}

	f.seen[gt.Reference] = struct{}{}
	return []tea.Model{NewModel(gt.Reference, f.common, f.config)}, nil
}
