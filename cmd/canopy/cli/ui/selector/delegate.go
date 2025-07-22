package selector

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/internal"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"io"
	"sort"
)

type listItemDelegate struct {
	list.DefaultDelegate

	keyMap keyMap

	highlightedStyle lipgloss.Style
	multiSelectStyle lipgloss.Style
	allTestsStyle    lipgloss.Style
	filterMatchStyle lipgloss.Style
	normalStyle      lipgloss.Style

	wasFilterViewActive bool // used to determine if we were filtering before the last update

	refState    referenceState
	current     *gotest.Reference
	cursorScope map[gotest.Reference]struct{}
	multiSelect map[gotest.Reference]struct{}
}

func newItemDelegate(keyMap keyMap) *listItemDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false

	filterMatchStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})

	d.SetHeight(1)
	d.SetSpacing(0)

	// d.Styles.SelectedTitle = lipgloss.NewStyle().
	//	Border(lipgloss.HiddenBorder(), false, false, false, true).
	//	BorderBackground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
	//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
	//	Padding(0, 0, 0, 1)

	cursorBrd := lipgloss.NormalBorder()
	cursorBrd.Left = "░" // ❯❱ ●• » ▚ [ "\U0001FB6A", this is a powerline glyph, but it doesn't work in all terminals, so we use a normal character instead )

	highlightPadding := lipgloss.NewStyle().Padding(0, 0, 0, 1)

	d.Styles.SelectedTitle = highlightPadding.
		Border(cursorBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		// BorderForeground(lipgloss.AdaptiveColor{Light: "#AD58B4", Dark: "#EEEEEE"}).
		// BorderBackground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})

	scopeBrd := lipgloss.NormalBorder()
	scopeBrd.Left = "░" // █ ░ ▎ ▚ │

	highlightStyle := highlightPadding.
		Border(scopeBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	// Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})

	multiSelectBrd := lipgloss.NormalBorder()
	multiSelectBrd.Left = "█" // ✔

	multiSelectStyle := highlightStyle.
		Border(multiSelectBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})

	return &listItemDelegate{
		DefaultDelegate: d,
		keyMap:          keyMap,
		multiSelect:     make(map[gotest.Reference]struct{}),
		cursorScope:     make(map[gotest.Reference]struct{}),
		//highlightedStyle: lipgloss.NewStyle().
		//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
		//	Padding(0, 0, 0, 2),
		filterMatchStyle: filterMatchStyle,
		normalStyle:      lipgloss.NewStyle().Padding(0, 0, 0, 1),
		highlightedStyle: highlightStyle,
		multiSelectStyle: multiSelectStyle,
		allTestsStyle:    lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888")).Padding(0, 0, 0, 2),
		refState:         referenceState{},
	}
}

func (d *listItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	isFilterViewActive := m.FilterState() == list.Filtering

	if isFilterViewActive != d.wasFilterViewActive {
		// if we just switched to filtering, we need to update the items
		// to reflect the current filter state.
		d.refState.update(m)
	}

	var cmds []tea.Cmd
	cmds = append(cmds, d.DefaultDelegate.Update(msg, m))

	switch msg := msg.(type) {

	case uievent.SwitchState:
		// select the last item that was selected in the list
		if d.current != nil {
			for idx, i := range m.Items() {
				it := i.(item)
				if it.ref == *d.current {
					m.Select(idx)
					break
				}
			}
		}

		d.refState.state = state.NewDefinitionViewer(msg.Definitions)
		d.refState.update(m)
		d.onNavigate(m)

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.CursorUp()
			d.onNavigate(m)
		case tea.MouseButtonWheelDown:
			m.CursorDown()
			d.onNavigate(m)
		default:
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				d.onNavigate(m)
			}
		}

	case tea.KeyMsg:
		// note: delegates do not receive key messages regarding filtering
		switch {
		case key.Matches(msg, d.keyMap.SelectTest):
			d.onToggleMultiselect(m)
		case key.Matches(msg, d.keyMap.NextPackage):
			cmds = append(cmds, d.nextPkg(m))
			d.onNavigate(m)
		case key.Matches(msg, d.keyMap.PrevPackage):
			cmds = append(cmds, d.prevPkg(m))
			d.onNavigate(m)
		case key.Matches(msg, m.KeyMap.CursorDown, m.KeyMap.CursorUp, m.KeyMap.PrevPage, m.KeyMap.NextPage):
			d.onNavigate(m)
		}

		// don't match any of the keys below if we're actively filtering.
		if m.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, filterKeyBindings...):
			// if matches a-z, A-Z then we set the filter state to filtering. We should account for the missing input
			// character that the user just entered (as well as anything else that already may be applied in the filter text)
			m.SetFilterText(m.FilterInput.Value() + msg.String())
			m.SetFilterState(list.Filtering)
		}
	}

	d.wasFilterViewActive = isFilterViewActive

	return tea.Batch(cmds...)
}

func (d *listItemDelegate) nextPkg(m *list.Model) tea.Cmd {
	currentIdx := m.Index()
	currentElement := m.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	for i := currentIdx; i < len(d.refState.visibleRefs); i++ {
		if d.refState.visibleRefs[i].Package != curPkg {
			m.Select(i)
			break
		}
	}

	return d.refState.update(m)
}

func (d *listItemDelegate) prevPkg(m *list.Model) tea.Cmd {
	currentIdx := m.Index()
	currentElement := m.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	targetPkg := ""
	for i := currentIdx; i >= 0; i-- {
		if targetPkg == "" {
			if d.refState.visibleRefs[i].Package != curPkg {
				targetPkg = d.refState.visibleRefs[i].Package
				continue
			}
		} else {
			// head to the top of the package
			if d.refState.visibleRefs[i].Package != targetPkg {
				// select the previous reference...
				m.Select(i + 1)
				break
			}
		}
	}

	return d.refState.update(m)
}

func (d *listItemDelegate) onNavigate(m *list.Model) {
	currentItem := m.SelectedItem()
	if currentItem == nil {
		// we have changed the view in a way that invalidates the selection (we're outside of the bounds)
		// let's select the last item in the list to keep the cursor in a valid position but as close as
		// possible to the last selected item
		m.Select(len(m.Items()) - 1)
	} else {
		// keep track of the last selected item
		currentRef := currentItem.(item).ref
		d.current = &currentRef
	}

	// TODO: maybe only on OnArrow or other keys?
	d.cursorScope = make(map[gotest.Reference]struct{})
	selectedIdx, selected := d.selectedItem(m)
	markChildren(selected, selectedIdx, d.visibleItems(m), d.cursorScope, false)
}

func (d *listItemDelegate) onToggleMultiselect(m *list.Model) {
	d.cursorScope = make(map[gotest.Reference]struct{}) // reset!

	selectedIdx, selected := d.selectedItem(m)
	var invert bool
	if _, ok := d.multiSelect[selected.ref]; ok {
		delete(d.multiSelect, selected.ref)
		invert = true
	} else {
		d.multiSelect[selected.ref] = struct{}{}
	}

	markChildren(selected, selectedIdx, d.visibleItems(m), d.multiSelect, invert)
}

func (d listItemDelegate) visibleItems(m *list.Model) []item {
	var refs []item
	for _, it := range m.VisibleItems() {
		refs = append(refs, it.(item))
	}
	return refs
}

func (d listItemDelegate) selectedItem(m *list.Model) (int, item) {
	return m.Index(), m.SelectedItem().(item)
}

func (d listItemDelegate) selectedTestReferencesCmd(m *list.Model) tea.Cmd {
	var refs []gotest.Reference
	for ref := range d.cursorScope {
		refs = append(refs, ref)
	}
	for ref := range d.multiSelect {
		refs = append(refs, ref)
	}
	sort.Sort(gotest.References(refs))
	return func() tea.Msg {
		return uievent.SelectedTestReferences{
			All:  m.SelectedItem().(item).ref.Package == "*",
			Refs: refs,
		}
	}
}

func isChild(ref, other *gotest.Reference) bool {
	if other == nil || ref == nil {
		return false
	}
	if ref.Package != other.Package {
		return false
	}

	if ref.FuncName == "" {
		// all items are children of the package
		return true
	}

	if ref.FuncName != other.FuncName {
		return false
	}
	if ref.TRunName == "" {
		return true
	}
	if other.TRunName == "" {
		return false
	}
	return ref.TRunName == other.TRunName
}

func markChildren(selected item, start int, visibleItems []item, marker map[gotest.Reference]struct{}, invert bool) {
	ref := selected.ref
	for i := start; i < len(visibleItems); i++ {
		it := visibleItems[i]
		other := it.ref

		if ref.Package == "*" {
			if invert {
				delete(marker, other)
			} else {
				marker[other] = struct{}{}
			}

			continue
		}

		if isChild(&ref, &other) {
			if invert {
				delete(marker, other)
			} else {
				marker[other] = struct{}{}
			}
		} else {
			break
		}
	}
}

func (d listItemDelegate) Render(w io.Writer, m list.Model, idx int, i list.Item) {
	it := i.(item)

	if m.Index() == idx {
		// selected
		w = internal.NewIndentWriter(w, " ❯ ")
	} else {
		w = internal.NewIndentWriter(w, "   ")
	}

	if it.ref.Package == "*" {
		d.Styles.NormalTitle = d.allTestsStyle
	}

	if _, ok := d.multiSelect[it.ref]; ok {
		// multi selected: the user has selected at least this item
		d.Styles.NormalTitle = d.multiSelectStyle

	} else if _, ok := d.cursorScope[it.ref]; ok {
		// highlighted: the cursor is on this item
		d.Styles.NormalTitle = d.highlightedStyle
	}

	// don't show matched characters when filtering is not occurring (including when the filter has been applied)
	//if m.FilterState() == list.Filtering {
	//	d.DefaultDelegate.Styles.FilterMatch = d.filterMatchStyle
	//} else {
	//	d.DefaultDelegate.Styles.FilterMatch = d.normalStyle
	//}

	d.DefaultDelegate.Render(
		w,
		m, idx, i,
	)
}

//func (d listItemDelegate) render(w io.Writer, m Model, index int, item list.Item) {
//	var (
//		title, desc  string
//		matchedRunes []int
//		s            = &d.Styles
//	)
//
//	if i, ok := item.(list.DefaultItem); ok {
//		title = i.Title()
//		desc = i.Description()
//	} else {
//		return
//	}
//
//	if m.width <= 0 {
//		// short-circuit
//		return
//	}
//
//	// Prevent text from exceeding list width
//	textwidth := m.width - s.NormalTitle.GetPaddingLeft() - s.NormalTitle.GetPaddingRight()
//	title = ansi.Truncate(title, textwidth, ellipsis)
//	if d.ShowDescription {
//		var lines []string
//		for i, line := range strings.Split(desc, "\n") {
//			if i >= d.height-1 {
//				break
//			}
//			lines = append(lines, ansi.Truncate(line, textwidth, ellipsis))
//		}
//		desc = strings.Join(lines, "\n")
//	}
//
//	// Conditions
//	var (
//		isSelected  = index == m.Index()
//		emptyFilter = m.FilterState() == Filtering && m.FilterValue() == ""
//		isFiltered  = m.FilterState() == Filtering || m.FilterState() == FilterApplied
//	)
//
//	if isFiltered && index < len(m.filteredItems) {
//		// Get indices of matched characters
//		matchedRunes = m.MatchesForItem(index)
//	}
//
//	if emptyFilter {
//		title = s.DimmedTitle.Render(title)
//		desc = s.DimmedDesc.Render(desc)
//	} else if isSelected && m.FilterState() != Filtering {
//		if isFiltered {
//			// Highlight matches
//			unmatched := s.SelectedTitle.Inline(true)
//			matched := unmatched.Inherit(s.FilterMatch)
//			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
//		}
//		title = s.SelectedTitle.Render(title)
//		desc = s.SelectedDesc.Render(desc)
//	} else {
//		if isFiltered {
//			// Highlight matches
//			unmatched := s.NormalTitle.Inline(true)
//			matched := unmatched.Inherit(s.FilterMatch)
//			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
//		}
//		title = s.NormalTitle.Render(title)
//		desc = s.NormalDesc.Render(desc)
//	}
//
//	if d.ShowDescription {
//		fmt.Fprintf(w, "%s\n%s", title, desc) //nolint: errcheck
//		return
//	}
//	fmt.Fprintf(w, "%s", title) //nolint: errcheck
//}
