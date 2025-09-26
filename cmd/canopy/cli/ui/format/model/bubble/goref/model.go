package goref

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
	_ frame.TerminalElement    = (*Model)(nil)
)

type Reactor interface {
	handler.TestEventHandler
	fmt.Stringer
}

type Viewer func(ref gotest.Reference, common state.Common, completed map[gotest.Reference]struct{}, elapsed time.Duration) string

type Model struct {
	ref    gotest.Reference
	action gotest.Action

	// event driven state
	common    state.Common
	start     *time.Time
	running   map[gotest.Reference]struct{}
	completed map[gotest.Reference]struct{}

	fragment Reactor
	viewer   Viewer
	buffer   *bytes.Buffer
}

func NewModel(ref gotest.Reference, common state.Common, viewer Viewer, rowFactory func(io.Writer, gotest.Reference) Reactor) *Model {
	var buffer bytes.Buffer

	return &Model{
		ref:       ref,
		viewer:    viewer,
		fragment:  rowFactory(&buffer, ref),
		buffer:    &buffer,
		common:    common,
		running:   make(map[gotest.Reference]struct{}),
		completed: make(map[gotest.Reference]struct{}),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) ShouldImprint() bool {
	return m.isExpired(true)
}

func (m Model) isExpired(enabled bool) bool {
	if !enabled {
		return false
	}
	switch m.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (m Model) IsAlive() bool {
	return true
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.common.OnMessage(msg)
	switch msg := msg.(type) {
	case partybus.Event:
		return m.handlePartybusEvent(msg)
	}
	return m, nil
}

func (m Model) handlePartybusEvent(e partybus.Event) (tea.Model, tea.Cmd) {
	if e.Type != event.GoTestType {
		return m, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return m, nil
	}

	if gt.Reference.Package != m.ref.Package {
		return m, nil
	}

	m.trackRunningTests(gt)

	if m.start == nil {
		m.start = &gt.Time
	}

	if gt.Reference == m.ref {
		m.action = gt.Action
	}

	err = m.fragment.OnGoTestEvent(gt)
	switch {
	case err == nil:
		break
	case errors.Is(err, handler.ErrPackageComplete):
		return m, nil
	default:
		panic("TODO: gostdref error: " + err.Error())
	}

	return m, nil
}

func (m *Model) trackRunningTests(e gotest.Event) {
	if e.Reference.IsPackage() {
		return
	}
	switch e.Action {
	case gotest.RunAction:
		m.running[e.Reference] = struct{}{}
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		delete(m.running, e.Reference)
		m.completed[e.Reference] = struct{}{}
	}
}

func (m Model) View() string {
	if buffer := m.buffer.String(); buffer != "" {
		return strings.TrimSpace(buffer)
	}

	render := strings.TrimSpace(m.fragment.String())
	if render != "" {
		return render
	}

	var elapsed time.Duration
	if m.start != nil {
		elapsed = time.Since(*m.start).Truncate(time.Millisecond)
	}

	return m.viewer(m.ref, m.common, m.completed, elapsed)
}
