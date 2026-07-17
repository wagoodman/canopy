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

const tabWidth = 4 // number of spaces per tab character

var _ tea.Model = (*frameWithFooter)(nil)

var _ interface {
	tea.Model
	clio.UI
} = (*UI)(nil)

type frameWithFooter struct {
	body   *frame.Frame
	footer *frame.Frame
}

func newFrameWithFooter() frameWithFooter {
	return frameWithFooter{
		body:   frame.New(),
		footer: frame.New(),
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

	f.body = bodyModel.(*frame.Frame)
	f.footer = footerModel.(*frame.Frame)

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
	coreUI        *coreUI
	printReaders  []io.Reader
	spinner       *syncspinner.Model

	frame frameWithFooter
}

func NewTeaUIConfig(handlers ...bubbly.EventHandler) *TeaUIConfig {
	fc := &TeaUIConfig{
		handler: bubbly.NewHandlerCollection(handlers...),
		coreUI:  nil,
		frame:   newFrameWithFooter(),
	}

	return fc
}

func (c *TeaUIConfig) WithCoreUI(u *coreUI) *TeaUIConfig {
	c.coreUI = u
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
	m.program = tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithoutSignalHandler())

	m.running.Add(1)

	go func() {
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
				m.program.Println(expandTabs(scanner.Text()))
			}

			m.running.Done()
		}(m.config.printReaders[i])
	}

	return m.config.coreUI.Setup(subscription)
}

// expandTabs replaces tab characters in a string with spaces, expanding each tab to a fixed width. Tabs in the terminal
// really advance the cursor to the next tab stop, which is typically every 4 spaces. However, whatever characters
// that are between the tab and the next tab stop are not overwritten with spaces. In a bubbletea UI, this can cause
// issues since we may be continually overwriting the same line with new content, leaving artifacts behind where tab
// characters were present (like small 4 character windows in the previous screen buffer). For this reason, we simulate
// tab characters for any possible input, replacing with spaces to overwrite all characters between the tab and
// next tab stop.
// Why go this route instead of changing the output to not use tabs? Mainly because the output from `go test` already
// uses tabs, so we would need to preserve the semantics in each case anyway. For this reason, it's generally easier
// to do this once just-in-time before rendering the output to the terminal, rather than in all places where we are
// reasoning about the output of `go test` (which would still need to preserve ansi control characters too).
func expandTabs(s string) string {
	var result strings.Builder
	column := 0

	runes := []rune(s)
	i := 0

	for i < len(runes) {
		r := runes[i]

		switch r {
		case '\t':
			// calculate how many spaces needed to reach next tab stop
			spacesToAdd := tabWidth - (column % tabWidth)
			result.WriteString(strings.Repeat(" ", spacesToAdd))
			column += spacesToAdd
			i++
		case '\n', '\r':
			// reset column position on newline
			result.WriteRune(r)
			column = 0
			i++
		case '\x1b': // ESC character - start of ANSI sequence
			// find the end of the ANSI escape sequence and copy it without affecting column
			start := i
			i++ // skip ESC

			// handle CSI sequences (most common: ESC[...m)
			if i < len(runes) && runes[i] == '[' {
				i++ // skip '['
				// find the end of the CSI sequence (ends with a letter)
				for i < len(runes) && !isCSITerminator(runes[i]) {
					i++
				}
				if i < len(runes) {
					i++ // include the terminator
				}
			} else {
				// handle other escape sequences (like ESC(, ESC), etc.)
				// most are 2 characters, but some can be longer
				for i < len(runes) && i < start+10 { // reasonable limit
					if isEscapeTerminator(runes[i]) {
						i++
						break
					}
					i++
				}
			}

			// copy the entire ANSI sequence to result without affecting column
			for j := start; j < i; j++ {
				result.WriteRune(runes[j])
			}
		default:
			// regular character - affects column position
			result.WriteRune(r)
			column++
			i++
		}
	}

	return result.String()
}

// isCSITerminator checks if a rune terminates a CSI sequence
func isCSITerminator(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

// isEscapeTerminator checks if a rune terminates other escape sequences
func isEscapeTerminator(r rune) bool {
	// common terminators for various escape sequences
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		r == '(' || r == ')' || r == '#' || r == '%'
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
	return m.config.coreUI.Handle(e)
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
	err := m.config.coreUI.Teardown(force)

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
			// let the models observe the interrupt before we quit, so the final rendered frame can
			// reflect the cancellation (e.g. the summary footer shows CANCELED instead of a stale spinner/PASS)
			frameModel, _ := m.config.frame.Update(msg)
			m.config.frame = frameModel.(frameWithFooter)
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
	return expandTabs(m.config.frame.View())
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
