// Package syncspinner provides a synchronized spinner component for Bubble Tea
// applications, ensuring consistent animation timing across multiple spinner instances.
package syncspinner

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ fmt.Stringer = (*Model)(nil)

// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var (
	lastID int
	idMtx  sync.Mutex
)

// nextID returns the next unique ID for a spinner model.
func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

// New creates a spinner model with default values and optional configuration.
func New(opts ...spinner.Option) Model {
	m := spinner.Model{
		Spinner: spinner.MiniDot,
	}

	for _, opt := range opts {
		opt(&m)
	}

	return Model{
		Spinner: m.Spinner,
		Style:   m.Style,
		id:      nextID(),
	}
}

// Model is a synchronized spinner that uses ID-based message filtering to
// prevent cross-talk between multiple spinner instances.
type Model struct {
	// Spinner is the spinner configuration (frames, FPS).
	Spinner spinner.Spinner

	// Style is the lipgloss style applied to the spinner.
	Style lipgloss.Style

	// frame is the current animation frame index.
	frame int

	// tag is used to filter duplicate messages.
	tag int

	// id is the unique identifier for this spinner instance.
	id int
}

// TickMsg is sent on each animation frame to update the spinner.
type TickMsg struct {
	// Time is when the tick occurred.
	Time time.Time

	// Tag is used to filter stale messages.
	Tag int

	// ID is the spinner instance ID.
	ID int

	// View is the current rendered spinner frame.
	View string
}

// ID returns the spinner's unique ID.
func (m Model) ID() int {
	return m.id
}

// Update processes Bubble Tea messages and advances the spinner animation.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		// If an ID is set, and the ID doesn't belong to this spinner, reject
		// the message.
		if msg.ID > 0 && msg.ID != m.id {
			return m, nil
		}

		// If a Tag is set, and it's not the one we expect, reject the message.
		// This prevents the spinner from receiving too many messages and
		// thus spinning too fast.
		if msg.Tag > 0 && msg.Tag != m.tag {
			return m, nil
		}

		m.frame++
		if m.frame >= len(m.Spinner.Frames) {
			m.frame = 0
		}

		m.tag++

		return m, m.tick()
	default:
		return m, nil
	}
}

// String returns the current spinner frame with styling applied.
func (m Model) String() string {
	if m.frame >= len(m.Spinner.Frames) {
		return "(error)"
	}

	return m.Style.Render(m.Spinner.Frames[m.frame])
}

// CurrentTick returns a tick message for the current spinner state.
func (m Model) CurrentTick() TickMsg {
	return TickMsg{
		Tag:  m.tag,
		ID:   m.id,
		View: m.String(),
	}
}

// Tick generates an immediate tick message for starting the spinner.
func (m Model) Tick() tea.Msg {
	t := m.CurrentTick()
	t.Time = time.Now()
	return t
}

// tick generates a command to advance the spinner after a delay based on FPS.
func (m Model) tick() tea.Cmd {
	return tea.Tick(m.Spinner.FPS, func(t time.Time) tea.Msg {
		s := m.CurrentTick()
		s.Time = t
		return s
	})
}
