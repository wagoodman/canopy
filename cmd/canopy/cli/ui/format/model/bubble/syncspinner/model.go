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

// Return the next ID we should use on the Model.
func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

// New returns a model with default values.
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

// Model contains the state for the spinner. Use New to create new models
// rather than using Model as a struct literal.
type Model struct {
	// Spinner settings to use. See type Spinner.
	Spinner spinner.Spinner

	// Style sets the styling for the spinner. Most of the time you'll just
	// want foreground and background coloring, and potentially some padding.
	Style lipgloss.Style

	frame int
	tag   int
	id    int
}

// TickMsg indicates that the timer has ticked and we should render a frame.
type TickMsg struct {
	Time time.Time
	Tag  int
	ID   int
	View string
}

// ID returns the spinner's unique ID.
func (m Model) ID() int {
	return m.id
}

// Update is the Tea update function.
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

// String renders the model's String.
func (m Model) String() string {
	if m.frame >= len(m.Spinner.Frames) {
		return "(error)"
	}

	return m.Style.Render(m.Spinner.Frames[m.frame])
}

func (m Model) CurrentTick() TickMsg {
	return TickMsg{
		Tag:  m.tag,
		ID:   m.id,
		View: m.String(),
	}
}

// Tick is the command used to advance the spinner one frame. Use this command
// to effectively start the spinner.
func (m Model) Tick() tea.Msg {
	t := m.CurrentTick()
	t.Time = time.Now()
	return t
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.Spinner.FPS, func(t time.Time) tea.Msg {
		s := m.CurrentTick()
		s.Time = t
		return s
	})
}
