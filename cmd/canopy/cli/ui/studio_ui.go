package ui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*StudioUI)(nil)

type StudioUI struct {
	config       studio.Config
	program      *tea.Program
	running      *sync.WaitGroup
	subscription partybus.Unsubscribable
}

func NewStudioUI(cfg studio.Config) *StudioUI {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &StudioUI{
		config:  cfg,
		running: wg,
	}
}

func (s *StudioUI) Setup(subscription partybus.Unsubscribable) error {
	if s == nil {
		return nil
	}
	s.subscription = subscription
	s.program = tea.NewProgram(
		studio.New(s.config, s.running),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
		tea.WithoutSignalHandler(),
	)

	go func() {
		defer s.running.Done()
		if _, err := s.program.Run(); err != nil {
			log.Errorf("unable to start UI: %+v", err)
			bus.ExitWithInterrupt()
		}
	}()

	return nil
}

func (s StudioUI) Wait() {
	s.running.Wait()
}

func (s *StudioUI) Handle(e partybus.Event) error {
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

func (s *StudioUI) Teardown(force bool) error {
	if s == nil {
		return nil
	}
	s.program.Quit()
	if !force {
		s.running.Wait()
	}
	return nil
}
