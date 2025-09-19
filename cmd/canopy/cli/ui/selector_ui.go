package ui

import (
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*SelectorUI)(nil)

type SelectorUI struct {
	program          *tea.Program
	running          *sync.WaitGroup
	subscription     partybus.Unsubscribable
	testDefinitions  gotest.Definitions // what is displayed as an option in the UI when it starts
	initialSelection gotest.References  // the initial selection of tests to run, if any
	model            selector.Model     // the current state of the UI model

	references []gotest.References
}

func NewSelectorUI(cfg selector.Config, testDefs gotest.Definitions, selected gotest.References) *SelectorUI {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &SelectorUI{
		running:          wg,
		testDefinitions:  testDefs,
		initialSelection: selected,
		model:            selector.New(cfg),
	}
}

func (s *SelectorUI) Setup(subscription partybus.Unsubscribable) error {
	if s == nil {
		return nil
	}
	s.subscription = subscription
	s.program = tea.NewProgram(
		s.model,
		// disabling zone support since it does not work well with bubbletea table filtering
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
		tea.WithoutSignalHandler(),
	)

	// setup initial state
	go func() {
		s.program.Send(uievent.SwitchState{
			Definitions: s.testDefinitions,
			Selected:    s.initialSelection,
		})
	}()

	// run the application
	go func() {
		defer s.running.Done()
		finalModel, err := s.program.Run()
		if err != nil {
			log.Errorf("unable to start UI: %+v", err)
			bus.ExitWithInterrupt()
		}

		if m, ok := finalModel.(selector.Model); ok {
			s.model = m
		} else {
			log.Errorf("unexpected final model type: %T", finalModel)
		}
	}()

	return nil
}

func (s *SelectorUI) Prompt() gotest.References {
	s.running.Wait()
	// TODO: is there a better way to do this? should this go to stderr?
	fmt.Println(s.model.View())
	return s.model.Selected()
}

func (s *SelectorUI) Handle(e partybus.Event) error {
	if s == nil {
		return nil
	}
	if s.program != nil {
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
