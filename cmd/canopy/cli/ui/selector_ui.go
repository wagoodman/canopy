package ui

import (
	"github.com/anchore/clio"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/wagoodman/go-partybus"
	"sync"
)

var _ clio.UI = (*SelectorUI)(nil)

type SelectorUI struct {
	config       selector.Config
	program      *tea.Program
	running      *sync.WaitGroup
	subscription partybus.Unsubscribable
	testDefs     gotest.Definitions
}

func NewSelectorUI(cfg selector.Config, testDefs []gotest.Definition) *SelectorUI {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &SelectorUI{
		config:   cfg,
		running:  wg,
		testDefs: testDefs,
	}
}

func (s *SelectorUI) Setup(subscription partybus.Unsubscribable) error {
	if s == nil {
		return nil
	}
	s.subscription = subscription
	s.program = tea.NewProgram(
		selector.New(s.config),
		//tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
		tea.WithoutSignalHandler(),
	)

	// setup initial state
	go func() {
		s.program.Send(uievent.SwitchState{
			Definitions: s.testDefs,
		})
	}()

	// run the application
	go func() {
		defer s.running.Done()
		if _, err := s.program.Run(); err != nil {
			log.Errorf("unable to start UI: %+v", err)
			bus.ExitWithInterrupt()
		}
	}()

	return nil
}

func (s SelectorUI) Wait() {
	s.running.Wait()
}

func (s *SelectorUI) Handle(e partybus.Event) error {
	if s == nil {
		return nil
	}
	if s.program != nil {
		if e.Type == event.GoTestType {
			// this is **really** expensive, as it's going to incur some update to the UI on each event (which is a lot)
			return nil
		}
		s.program.Send(e)
	}
	return nil
}

func (s *SelectorUI) Teardown(force bool) error {
	if s == nil {
		return nil
	}
	s.program.Quit()
	if !force {
		s.running.Wait()
	}
	return nil
}
