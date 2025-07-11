package model

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

var _ Element = (*Dispatch)(nil)

type Dispatch struct {
	selected                  int
	order                     []string
	fragments                 map[string]Element
	defaultKeyMap             xhelp.KeyMap
	viewFn                    func(Dispatch) string
	filterAlphaNumericsOnlyTo string
}

func NewDispatch(keyMap xhelp.KeyMap) Dispatch {
	return Dispatch{
		fragments:     make(map[string]Element),
		defaultKeyMap: keyMap,
	}
}

func (m *Dispatch) SetViewer(fn func(Dispatch) string) {
	m.viewFn = fn
}

func (m Dispatch) Get(key string) Element {
	return m.fragments[key]
}

func (m *Dispatch) Add(key string, fragment Element) {
	if !slices.Contains(m.order, key) {
		m.order = append(m.order, key)
	}
	m.fragments[key] = fragment
}

func (m *Dispatch) next() {
	if m.selected < len(m.fragments)-1 {
		m.selected++
	} else {
		m.selected = 0
	}
}

func (m Dispatch) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, fragment := range m.fragments {
		cmds = append(cmds, fragment.Init())
	}
	return tea.Batch(cmds...)
}

func (m Dispatch) View() string {
	return m.viewFn(m)
}

func (m Dispatch) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint: gocognit
	// TODO: augment the WS on the parent to affect the children
	var ws *tea.WindowSizeMsg
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			// TODO: emit selected and unselected events to children? or call function?
			m.next()
			return m, nil
		}
	case tea.WindowSizeMsg:
		ws = &msg
		// case uievent.SelectedTestReferences:
		// debug.SetLine(fmt.Sprintf("(dispatch) on select: %d (%d)", len(msg.Refs), rand.Int()))
	case uievent.FilteringInput:
		if msg.Completed {
			m.filterAlphaNumericsOnlyTo = ""
		} else {
			m.filterAlphaNumericsOnlyTo = msg.Name
		}
	}

	var updateKeys []string
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		updateKeys = []string{m.order[m.selected]}
	default:
		updateKeys = append(updateKeys, m.order...)
	}

	remainingWidth := -1
	if ws != nil {
		remainingWidth = ws.Width
	}

	var cmds []tea.Cmd
	for _, key := range updateKeys {
		fragment := m.fragments[key]
		if remainingWidth > 0 {
			// respond to neighboring fragments that are sizers
			if shaper, ok := fragment.(Shaper); ok {
				shaper.SetWidth(remainingWidth)
				remainingWidth = -1
				m.fragments[key] = shaper.(Element)
			}
		}

		// debug.SetLine(fmt.Sprintf("onlyPane: %v", m.filterAlphaNumericsOnlyTo))

		if m.filterAlphaNumericsOnlyTo != "" {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				if IsAlphaNumeric(keyMsg) && key != m.filterAlphaNumericsOnlyTo {
					continue
				}
			}
		}

		f, cmd := fragment.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.fragments[key] = f.(Element)

		if remainingWidth > 0 {
			// adjust the size of the fragment based on the latest update
			if sizer, ok := fragment.(Sizer); ok {
				remainingWidth -= sizer.Width()
				m.fragments[key] = sizer.(Element)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Dispatch) KeyMap() xhelp.KeyMap {
	return xhelp.JoinKeyMaps(m.defaultKeyMap, m.fragments[m.order[m.selected]])
}

func (m Dispatch) ShortHelp() []xhelp.Item {
	return m.KeyMap().ShortHelp()
}

func (m Dispatch) FullHelp() [][]xhelp.Item {
	km := m.KeyMap()
	if fkm, ok := km.(xhelp.FullKeyMap); ok {
		return fkm.FullHelp()
	}
	return nil
}

func IsAlphaNumeric(msg tea.KeyMsg) bool {
	// return true if is a-z, A-Z or 0-9
	if len(msg.Runes) != 1 {
		return false
	}
	r := msg.Runes[0]
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
