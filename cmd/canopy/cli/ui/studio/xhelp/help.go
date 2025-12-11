package xhelp

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// Item wraps a key.Binding with optional toggle state support, allowing
// keybindings to show whether a feature is currently enabled or disabled.
type Item struct {
	key.Binding

	// toggle holds state for keybindings that toggle a feature.
	toggle *Toggle
}

// NewKeyBinding creates an Item from a key.Binding without toggle functionality.
func NewKeyBinding(keyBinding key.Binding) Item {
	return Item{Binding: keyBinding}
}

// WithToggle adds toggle support to this Item. The state parameter indicates
// the initial toggle state, and the engaged/disengaged descriptions are shown
// in the help text based on the current state.
func (i Item) WithToggle(state bool, engagedDesc, disengagedDesc string) Item {
	i.toggle = &Toggle{
		engaged:        state,
		engagedDesc:    engagedDesc,
		disengagedDesc: disengagedDesc,
	}

	i.setHelp()

	return i
}

// Toggle represents the state of a toggleable keybinding.
type Toggle struct {
	// engaged indicates whether the toggle is currently active.
	engaged bool

	// engagedDesc is the help text shown when engaged.
	engagedDesc string

	// disengagedDesc is the help text shown when disengaged.
	disengagedDesc string

	// EngagedStyle is applied when the toggle is active.
	EngagedStyle *lipgloss.Style

	// DisengagedStyle is applied when the toggle is inactive.
	DisengagedStyle *lipgloss.Style
}

// Engaged returns whether this toggle is currently active.
func (i Item) Engaged() bool {
	if i.toggle == nil {
		return false
	}
	return i.toggle.engaged
}

// Press toggles the Item's state and updates its help text accordingly.
func (i *Item) Press() {
	if i.toggle == nil {
		return
	}
	i.toggle.engaged = !i.toggle.engaged
	i.setHelp()
}

// setHelp updates the Item's help text based on its current toggle state.
func (i *Item) setHelp() {
	k := i.Binding.Help().Key
	if k == "" {
		keys := i.Keys()
		if len(keys) > 0 {
			k = keys[0]
		}
	}
	if i.toggle.engaged {
		if i.toggle.engagedDesc != "" {
			i.SetHelp(k, i.toggle.engagedDesc)
		}
	} else {
		if i.toggle.disengagedDesc != "" {
			i.SetHelp(k, i.toggle.disengagedDesc)
		}
	}
}

// KeyMap defines the interface for retrieving keybindings for help display.
// Disabled keybindings are automatically excluded from the help view.
type KeyMap interface {
	// ShortHelp returns a slice of bindings to be displayed in the short
	// version of the help. The help bubble will render help in the order in
	// which the help items are returned here.
	ShortHelp() []Item
}

// FullKeyMap extends KeyMap with support for multi-column full help display.
type FullKeyMap interface {
	// FullHelp returns an extended group of help items, grouped by columns.
	// The help bubble will render the help in the order in which the help
	// items are returned here.
	FullHelp() [][]Item
}

// jointKeyMap combines multiple KeyMaps into a single KeyMap for display.
type jointKeyMap struct {
	keyMaps []KeyMap
}

// JoinKeyMaps merges multiple KeyMaps into one, concatenating their help items.
func JoinKeyMaps(keyMaps ...KeyMap) KeyMap {
	return jointKeyMap{keyMaps: keyMaps}
}

// ShortHelp implements KeyMap by concatenating all short help items.
func (m jointKeyMap) ShortHelp() []Item {
	var h []Item
	for _, km := range m.keyMaps {
		h = append(h, km.ShortHelp()...)
	}
	return h
}

// FullHelp implements FullKeyMap by concatenating all full help items.
func (m jointKeyMap) FullHelp() [][]Item {
	var h [][]Item
	for _, km := range m.keyMaps {
		if fkm, ok := km.(FullKeyMap); ok {
			h = append(h, fkm.FullHelp()...)
		}
	}
	return h
}

// Styles defines the visual styling for the help display.
type Styles struct {
	// Ellipsis is shown when help items are truncated due to width.
	Ellipsis lipgloss.Style

	// ShortKey styles the key portion of short help items.
	ShortKey lipgloss.Style

	// ShortDesc styles the description portion of short help items.
	ShortDesc lipgloss.Style

	// ShortSeparator styles the separator between short help items.
	ShortSeparator lipgloss.Style

	// FullKey styles the key portion of full help items.
	FullKey lipgloss.Style

	// FullDesc styles the description portion of full help items.
	FullDesc lipgloss.Style

	// FullSeparator styles the separator between full help columns.
	FullSeparator lipgloss.Style

	// Engaged styles toggle items when they are active.
	Engaged lipgloss.Style

	// Disengaged styles toggle items when they are inactive.
	Disengaged lipgloss.Style

	// IDStyle styles the session ID shown in the help footer.
	IDStyle lipgloss.Style
}

// Model manages the help display state and rendering.
type Model struct {
	// Width constrains the help display width.
	Width int

	// ShowAll toggles between short and full help display.
	ShowAll bool

	// ShortSeparator is the string used between short help items.
	ShortSeparator string

	// FullSeparator is the string used between full help columns.
	FullSeparator string

	// Ellipsis is shown when help items are truncated.
	Ellipsis string

	// Styles defines visual styling for help components.
	Styles Styles
}

// New creates a new help Model with default styling.
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

// View renders the help display with the given KeyMap and session ID, constrained
// to the specified width. Returns either short or full help based on ShowAll.
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

// Height returns the vertical space required by the help display for the given KeyMap.
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
// If the line exceeds the maximum width, it will be gracefully truncated.
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
func (m Model) FullHelpView(groups [][]Item) string {
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

// shouldRenderColumn returns true if the column contains at least one enabled keybinding.
func shouldRenderColumn(b []Item) (ok bool) {
	for _, v := range b {
		if v.Enabled() {
			return true
		}
	}
	return false
}
