package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
	"github.com/anchore/bubbly/bubbles/frame"
	"github.com/anchore/clio"
)

var _ tea.Model = (*frameWithFooter)(nil)

var _ interface {
	tea.Model
	clio.UI
} = (*UI)(nil)

type frameWithFooter struct {
	body   frame.Frame
	footer frame.Frame
}

func newFrameWithFooter() frameWithFooter {
	return frameWithFooter{
		body:   *frame.New(),
		footer: *frame.New(),
	}
}

func (f frameWithFooter) Init() tea.Cmd {
	return tea.Batch(f.body.Init(), f.footer.Init())
}

func (f frameWithFooter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	bodyModel, bodyCmd := f.body.Update(msg)
	cmds = append(cmds, bodyCmd)

	footerModel, footerCmd := f.footer.Update(msg)
	cmds = append(cmds, footerCmd)

	f.body = bodyModel.(frame.Frame)
	f.footer = footerModel.(frame.Frame)

	return f, tea.Batch(cmds...)
}

func (f frameWithFooter) View() string {
	b := f.body.View()

	sb := &strings.Builder{}
	if b != "" {
		sb.WriteString(b)
		if !strings.HasSuffix(b, "\n") {
			sb.WriteString("\n")
		}
	}
	sb.WriteString(f.footer.View())
	return sb.String()
}

type UI struct {
	config         TeaUIConfig
	program        *tea.Program
	running        *sync.WaitGroup
	subscription   partybus.Unsubscribable
	teardownCalled bool
}

type TeaUIConfig struct {
	handler       *bubbly.HandlerCollection
	footerHandler *bubbly.HandlerCollection
	simpleUI      *simpleUI
	printReaders  []io.Reader
	spinner       *syncspinner.Model

	frame frameWithFooter
}

func NewTeaUIConfig(handlers ...bubbly.EventHandler) *TeaUIConfig {
	fc := &TeaUIConfig{
		handler:  bubbly.NewHandlerCollection(handlers...),
		simpleUI: nil,
		frame:    newFrameWithFooter(),
	}

	return fc
}

func (c *TeaUIConfig) WithSimpleUI(u *simpleUI) *TeaUIConfig {
	c.simpleUI = u
	return c
}

func (c *TeaUIConfig) WithPrintReader(readers ...io.Reader) *TeaUIConfig {
	c.printReaders = append(c.printReaders, readers...)
	return c
}

func (c *TeaUIConfig) WithFooter(handlers ...bubbly.EventHandler) *TeaUIConfig {
	if c.footerHandler != nil {
		panic("footer handler already set")
	}

	c.footerHandler = bubbly.NewHandlerCollection(handlers...)
	return c
}

func (c *TeaUIConfig) WithSyncSpinner(s syncspinner.Model) *TeaUIConfig {
	if c.spinner != nil {
		panic("spinner already set")
	}
	c.spinner = &s
	return c
}

func NewTeaUI(c *TeaUIConfig) *UI {
	return &UI{
		config:  *c,
		running: &sync.WaitGroup{},
	}
}

func (m *UI) Setup(subscription partybus.Unsubscribable) error {
	if m == nil {
		return nil
	}
	m.subscription = subscription
	m.program = tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithInput(os.Stdin), tea.WithoutSignalHandler())
	// m.config.frame.withPrinter(m.program)

	m.running.Add(1)

	go func() {
		// defer m.running.Done()
		if _, err := m.program.Run(); err != nil {
			log.Errorf("unable to start UI: %+v", err)
			bus.ExitWithInterrupt()
		}

		m.running.Done()
	}()

	for i := range m.config.printReaders {
		m.running.Add(1)
		go func(reader io.Reader) {
			// scan for every line in the reader and print it just behind the UI
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				m.program.Println(scanner.Text())
			}

			m.running.Done()
		}(m.config.printReaders[i])
	}

	return m.config.simpleUI.Setup(subscription)
}

func (m *UI) Handle(e partybus.Event) error {
	if m == nil {
		return nil
	}
	// if m.teardownCalled {
	//	return nil
	//}
	if m.program != nil {
		m.program.Send(e)
	}
	return m.config.simpleUI.Handle(e)
}

func (m *UI) Teardown(force bool) error {
	if m == nil {
		return nil
	}
	if m.teardownCalled {
		return nil
	}
	m.teardownCalled = true

	// we need to make certain that any writers are closed before attempting to wait for them to complete
	err := m.config.simpleUI.Teardown(force)

	if !force {
		// just wow... tea commands are racy
		time.Sleep(100 * time.Millisecond)

		m.config.handler.Wait()
		m.program.Quit()
		// typically in all cases we would want to wait for the UI to finish. However there are still error cases
		// that are not accounted for, resulting in hangs. For now, we'll just wait for the UI to finish in the
		// happy path only. There will always be an indication of the problem to the user via reporting the error
		// string from the worker (outside of the UI after teardown).
		m.running.Wait()
	} else {
		_ = runWithTimeout(250*time.Millisecond, func() error {
			m.config.handler.Wait()
			// if m.config.footerHandler != nil {
			//	m.config.footerHandler.Wait()
			//}
			return nil
		})

		// it may be tempting to use Kill() however it has been found that this can cause the terminal to be left in
		// a bad state (where Ctrl+C and other control characters no longer works for future processes in that terminal).
		m.program.Quit()

		_ = runWithTimeout(250*time.Millisecond, func() error {
			m.running.Wait()
			// if m.config.footerHandler != nil {
			//	m.config.footerHandler.Wait()
			//}
			return nil
		})
	}

	// TODO: allow for writing out the full log output to the screen (only a partial log is shown currently)
	// this needs coordination to know what the last frame event is to change the state accordingly (which isn't possible now)
	return err
}

// bubbletea.Model functions

func (m UI) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.config.frame.Init(),
	}

	if m.config.spinner != nil {
		cmds = append(cmds, m.config.spinner.Tick)
	}

	return tea.Batch(cmds...)
}

func (m UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// if m.teardownCalled {
	//	return nil, nil
	//}
	// note: we need a pointer receiver such that the same instance of UI used in Teardown is referenced (to keep finalize events)

	var cmds []tea.Cmd

	// allow for non-partybus UI updates (such as window size events). Note: these must not affect existing models,
	// that is the responsibility of the frame object on this UI object. The handler is a factory of models
	// which the frame is responsible for the lifecycle of. This update allows for injecting the initial state
	// of the world when creating those models.
	m.config.handler.OnMessage(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// today we treat esc and ctrl+c the same, but in the future when the worker has a graceful way to
		// cancel in-flight work via a context, we can wire up esc to this path with bus.Exit()
		case "esc", "ctrl+c":
			bus.ExitWithInterrupt()
			return m, tea.Quit
		}

	case syncspinner.TickMsg:
		spinnerModel, cmd := m.config.spinner.Update(msg)
		m.config.spinner = &spinnerModel
		cmds = append(cmds, cmd)

	case partybus.Event:
		log.WithFields("component", "ui").Tracef("event: %q", msg.Type)

		// handle main body events
		models, cmd := m.config.handler.Handle(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		for _, newModel := range models {
			if newModel == nil {
				continue
			}
			cmds = append(cmds, newModel.Init())
			m.config.frame.body.AppendModel(newModel)
		}

		// handle footer events
		if m.config.footerHandler != nil {
			footerModels, cmd := m.config.footerHandler.Handle(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			for _, newModel := range footerModels {
				if newModel == nil {
					continue
				}
				cmds = append(cmds, newModel.Init())
				m.config.frame.footer.AppendModel(newModel)
			}
		}

		// intentionally fallthrough to update the frame model
	}

	frameModel, cmd := m.config.frame.Update(msg)
	m.config.frame = frameModel.(frameWithFooter)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m UI) View() string {
	// if m.teardownCalled {
	//	return ""
	//}
	return m.config.frame.View()
}

func runWithTimeout(timeout time.Duration, fn func() error) (err error) {
	c := make(chan struct{}, 1)
	go func() {
		err = fn()
		c <- struct{}{}
	}()
	select {
	case <-c:
	case <-time.After(timeout):
		return fmt.Errorf("timed out after %v", timeout)
	}
	return err
}
