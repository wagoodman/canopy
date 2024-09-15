package xhelp

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// this is a copy of the help.go file from the bubbles package in bubbletea
// this is a basis for extending help options to also be toggles, showing their state

type Item struct {
	key.Binding
	toggle *Toggle
}

func NewKeyBinding(keyBinding key.Binding) Item {
	return Item{Binding: keyBinding}
}

func (i Item) WithToggle(state bool, engagedDesc, disengagedDesc string) Item {
	i.toggle = &Toggle{
		engaged:        state,
		engagedDesc:    engagedDesc,
		disengagedDesc: disengagedDesc,
	}

	i.setHelp()

	return i
}

type Toggle struct {
	engaged        bool
	engagedDesc    string
	disengagedDesc string

	EngagedStyle    *lipgloss.Style
	DisengagedStyle *lipgloss.Style
}

func (i Item) Engaged() bool {
	if i.toggle == nil {
		return false
	}
	return i.toggle.engaged
}

func (i *Item) Press() {
	if i.toggle == nil {
		return
	}
	i.toggle.engaged = !i.toggle.engaged
	i.setHelp()
}

func (i *Item) setHelp() {
	k := i.Binding.Help().Key
	if k == "" {
		keys := i.Binding.Keys()
		if len(keys) > 0 {
			k = keys[0]
		}
	}
	if i.toggle.engaged {
		if i.toggle.engagedDesc != "" {
			i.Binding.SetHelp(k, i.toggle.engagedDesc)
		}
	} else {
		if i.toggle.disengagedDesc != "" {
			i.Binding.SetHelp(k, i.toggle.disengagedDesc)
		}
	}
}

// KeyMap is a map of keybindings used to generate help. Since it's an
// interface it can be any type, though struct or a map[string][]key.Binding
// are likely candidates.
//
// Note that if a key is disabled (via key.Binding.SetEnabled) it will not be
// rendered in the help view, so in theory generated help should self-manage.
type KeyMap interface {
	// ShortHelp returns a slice of bindings to be displayed in the short
	// version of the help. The help bubble will render help in the order in
	// which the help items are returned here.
	ShortHelp() []Item
}

type FullKeyMap interface {
	// FullHelp returns an extended group of help items, grouped by columns.
	// The help bubble will render the help in the order in which the help
	// items are returned here.
	FullHelp() [][]Item
}

type jointKeyMap struct {
	keyMaps []KeyMap
}

func JoinKeyMaps(keyMaps ...KeyMap) KeyMap {
	return jointKeyMap{keyMaps: keyMaps}
}

func (m jointKeyMap) ShortHelp() []Item {
	var h []Item
	for _, km := range m.keyMaps {
		h = append(h, km.ShortHelp()...)
	}
	return h
}

func (m jointKeyMap) FullHelp() [][]Item {
	var h [][]Item
	for _, km := range m.keyMaps {
		if fkm, ok := km.(FullKeyMap); ok {
			h = append(h, fkm.FullHelp()...)
		}
	}
	return h
}

// Styles is a set of available style definitions for the Help bubble.
type Styles struct {
	Ellipsis lipgloss.Style

	// Styling for the short help
	ShortKey       lipgloss.Style
	ShortDesc      lipgloss.Style
	ShortSeparator lipgloss.Style

	// Styling for the full help
	FullKey       lipgloss.Style
	FullDesc      lipgloss.Style
	FullSeparator lipgloss.Style

	// Styling for toggles
	Engaged    lipgloss.Style
	Disengaged lipgloss.Style

	IDStyle lipgloss.Style
}

// Model contains the state of the help view.
type Model struct {
	Width   int
	ShowAll bool // if true, render the "full" help menu

	ShortSeparator string
	FullSeparator  string

	// The symbol we use in the short help when help items have been truncated
	// due to width. Periods of ellipsis by default.
	Ellipsis string

	Styles Styles
}

// New creates a new help view with some useful defaults.
func New() Model {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	})

	descStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#B2B2B2",
		Dark:  "#4A4A4A",
	})

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#DDDADA",
		Dark:  "#3C3C3C",
	})

	engagedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D16DFF"))

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#B2B2B2",
		Dark:  "#4A4A4A",
	}).Padding(0, 1, 0, 1)

	return Model{
		ShortSeparator: " • ",
		FullSeparator:  "    ",
		Ellipsis:       "…",
		Styles: Styles{
			ShortKey:       keyStyle,
			ShortDesc:      descStyle,
			ShortSeparator: sepStyle,
			Ellipsis:       sepStyle,
			FullKey:        keyStyle,
			FullDesc:       descStyle,
			FullSeparator:  sepStyle,
			Engaged:        engagedStyle,
			Disengaged:     keyStyle,
			IDStyle:        idStyle,
		},
	}
}

func (m Model) View(keyMap KeyMap, id string, width int) string {
	var view string
	if m.ShowAll {
		if f, ok := keyMap.(FullKeyMap); ok {
			fh := f.FullHelp()
			if len(fh) > 0 {
				view = m.FullHelpView(fh)
			}
		}
	} else {
		view = m.ShortHelpView(keyMap.ShortHelp())
	}

	right := m.Styles.IDStyle.Width(width - lipgloss.Width(view)).Align(lipgloss.Right).Render(id)

	return lipgloss.JoinHorizontal(lipgloss.Top, view, right)
}

func (m Model) Height(keyMap KeyMap) int {
	if m.ShowAll {
		if f, ok := keyMap.(FullKeyMap); ok {
			fh := f.FullHelp()
			if len(fh) > 0 {
				maxLen := 0
				for _, col := range fh {
					if len(col) > maxLen {
						maxLen = len(col)
					}
				}
				return maxLen
			}
		}
	}
	return 1
}

// ShortHelpView renders a single line help view from a slice of keybindings.
// If the line is longer than the maximum width it will be gracefully
// truncated, showing only as many help items as possible.
func (m Model) ShortHelpView(bindings []Item) string {
	if len(bindings) == 0 {
		return ""
	}

	var b strings.Builder
	var totalWidth int
	separator := m.Styles.ShortSeparator.Inline(true).Render(m.ShortSeparator)

	for i, kb := range bindings {
		if !kb.Enabled() {
			continue
		}

		var sep string
		if totalWidth > 0 && i < len(bindings) {
			sep = separator
		}

		var sty, desc lipgloss.Style
		if kb.toggle != nil {
			if kb.Engaged() {
				sty = m.Styles.Engaged
			} else {
				sty = m.Styles.Disengaged
			}
			desc = sty
		} else {
			sty = m.Styles.ShortKey
			desc = m.Styles.ShortDesc
		}

		str := sep +
			sty.Inline(true).Render(kb.Help().Key+" ") +
			desc.Inline(true).Render(kb.Help().Desc)

		w := lipgloss.Width(str)

		// If adding this help item would go over the available width, stop
		// drawing.
		if m.Width > 0 && totalWidth+w > m.Width {
			// Although if there's room for an ellipsis, print that.
			tail := " " + m.Styles.Ellipsis.Inline(true).Render(m.Ellipsis)
			tailWidth := lipgloss.Width(tail)

			if totalWidth+tailWidth < m.Width {
				b.WriteString(tail)
			}

			break
		}

		totalWidth += w
		b.WriteString(str)
	}

	return b.String()
}

// FullHelpView renders help columns from a slice of key binding slices. Each
// top level slice entry renders into a column.
func (m Model) FullHelpView(groups [][]Item) string { //nolint:funlen
	if len(groups) == 0 {
		return ""
	}

	// Linter note: at this time we don't think it's worth the additional
	// code complexity involved in preallocating this slice.
	//nolint:prealloc
	var (
		out []string

		totalWidth int
		sep        = m.Styles.FullSeparator.Render(m.FullSeparator)
		sepWidth   = lipgloss.Width(sep)
	)

	// Iterate over groups to build columns
	for i, group := range groups {
		if group == nil || !shouldRenderColumn(group) {
			continue
		}

		var (
			keys         []string
			descriptions []string
		)

		// Separate keys and descriptions into different slices
		for _, kb := range group {
			if !kb.Enabled() {
				continue
			}

			var sty, desc lipgloss.Style
			if kb.toggle != nil {
				if kb.Engaged() {
					sty = m.Styles.Engaged
				} else {
					sty = m.Styles.Disengaged
				}
				desc = sty
			} else {
				sty = m.Styles.ShortKey
				desc = m.Styles.ShortDesc
			}

			keys = append(keys, sty.Render(kb.Help().Key))
			descriptions = append(descriptions, desc.Render(kb.Help().Desc))
		}

		col := lipgloss.JoinHorizontal(lipgloss.Top,
			m.Styles.FullKey.Render(strings.Join(keys, "\n")),
			m.Styles.FullKey.Render(" "),
			m.Styles.FullDesc.Render(strings.Join(descriptions, "\n")),
			"    ",
		)

		// Column
		totalWidth += lipgloss.Width(col)
		if m.Width > 0 && totalWidth > m.Width {
			break
		}

		out = append(out, col)

		// Separator
		if i < len(group)-1 {
			totalWidth += sepWidth
			if m.Width > 0 && totalWidth > m.Width {
				break
			}
			out = append(out, sep)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

func shouldRenderColumn(b []Item) (ok bool) {
	for _, v := range b {
		if v.Enabled() {
			return true
		}
	}
	return false
}
