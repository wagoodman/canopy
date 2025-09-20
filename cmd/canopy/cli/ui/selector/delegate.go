package selector

import (
	"io"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/internal"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var (
	hoverBorder        = lipgloss.Border{Left: "░"}
	userSelectedBorder = lipgloss.Border{Left: "█"}
	emptyBorder        = lipgloss.Border{Left: " "}
)

type listItemDelegate struct {
	list.DefaultDelegate

	keyMap   keyMap
	styles   delegateStyles
	finished bool

	refState        referenceState
	current         *gotest.Reference
	cursorScope     map[gotest.Reference]struct{}
	cursorScopeSize int
	userSelect      map[gotest.Reference]struct{}
}

type delegateStyles struct {
	hoverLine        lipgloss.Style
	hoverBullet      lipgloss.Style
	cursorLine       lipgloss.Style
	userSelectedLine lipgloss.Style
	allTestsLine     lipgloss.Style
	normalStyle      lipgloss.Style
}

func newItemDelegate(keyMap keyMap) *listItemDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)
	d.Styles.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})

	baseStyle := lipgloss.NewStyle().Padding(0, 0, 0, 1)

	return &listItemDelegate{
		DefaultDelegate: d,
		userSelect:      make(map[gotest.Reference]struct{}),
		cursorScope:     make(map[gotest.Reference]struct{}),
		refState:        referenceState{},
		keyMap:          keyMap,

		styles: delegateStyles{
			normalStyle: lipgloss.NewStyle().
				Padding(0, 0, 0, 2),

			hoverLine: baseStyle.
				BorderLeft(true).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}),

			hoverBullet: lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}),

			cursorLine: baseStyle.
				BorderLeft(true).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}),

			userSelectedLine: baseStyle.
				BorderLeft(true).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}),

			allTestsLine: lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 0, 0, 2),
		},
	}
}

func (d *listItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, d.DefaultDelegate.Update(msg, m))

	switch msg := msg.(type) {
	case uievent.RefreshReferences:
		cmds = append(cmds, d.refState.update(m, msg.AboutToFilter))

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

		// set the initial test definitions
		d.refState = newReferenceState(state.NewDefinitionViewer(msg.Definitions), d.refState)
		cmds = append(cmds, d.refState.update(m))

		// select any items that were selected by the user with -run statements...
		selectedSet := make(map[gotest.Reference]struct{})
		for _, ref := range msg.Selected {
			d.userSelect[ref] = struct{}{}
			selectedSet[ref] = struct{}{}
		}

		firstSelectedIdx := -1
		for candidateIdx, candidate := range m.Items() {
			it := candidate.(item)
			if _, ok := selectedSet[it.ref]; ok {
				markChildren(it, candidateIdx, d.visibleItems(m), d.userSelect, false, d.refState.children)
				firstSelectedIdx = candidateIdx // remember the first selected item
			}
		}

		if firstSelectedIdx >= 0 {
			m.Select(firstSelectedIdx) // move the cursor to the last selected item
		}
		cmds = append(cmds, d.onNavigate(m))

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.CursorUp()
			cmds = append(cmds, d.onNavigate(m))
		case tea.MouseButtonWheelDown:
			m.CursorDown()
			cmds = append(cmds, d.onNavigate(m))
		default:
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				cmds = append(cmds, d.onNavigate(m))
			}
		}

	case tea.KeyMsg:
		// note: delegates do not receive key messages regarding filtering
		switch {
		case key.Matches(msg, d.keyMap.SelectTest):
			cmds = append(cmds, d.onToggleMultiselect(m))
			cmds = append(cmds, d.onNavigate(m))

		case key.Matches(msg, d.keyMap.SelectAllTests):
			isAllAlreadySelected := len(d.userSelect) == len(m.Items())
			cmds = append(cmds, d.onSelectAll(m, isAllAlreadySelected))

		case key.Matches(msg, d.keyMap.NextPackage):
			cmds = append(cmds, d.nextPkg(m))
			cmds = append(cmds, d.onNavigate(m))

		case key.Matches(msg, d.keyMap.PrevPackage):
			cmds = append(cmds, d.prevPkg(m))
			cmds = append(cmds, d.onNavigate(m))

		case key.Matches(msg, m.KeyMap.CursorDown, m.KeyMap.CursorUp, m.KeyMap.PrevPage, m.KeyMap.NextPage):
			cmds = append(cmds, d.onNavigate(m))

		case key.Matches(msg, d.keyMap.ToggleReferenceLongForm):
			// toggle the long form view
			d.refState.preferLongForm = !d.refState.preferLongForm
			cmds = append(cmds, d.refState.update(m))

		case key.Matches(msg, d.keyMap.ToggleTests):
			// toggle the long form view
			d.refState.hideTests = !d.refState.hideTests
			cmds = append(cmds, d.refState.update(m))

		case key.Matches(msg, d.keyMap.Finish):
			d.finished = true
			cmds = append(cmds, d.selectedTestReferences(m, true))
		}

		// don't match any of the keys below if we're actively filtering.
		if m.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, filterKeyBindings...):
			// if matches a-z, A-Z then we set the filter state to filtering. We should account for the missing input
			// character that the user just entered (as well as anything else that already may be applied in the filter text)
			m.SetFilterText(m.FilterInput.Value() + msg.String())
			m.SetFilterState(list.Filtering)
			cmds = append(cmds, d.refState.update(m))
		}
	}

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
	for i := currentIdx; i < len(d.refState.visible); i++ {
		if d.refState.visible[i].Package != curPkg {
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
			if d.refState.visible[i].Package != curPkg {
				targetPkg = d.refState.visible[i].Package
				continue
			}
		} else {
			// head to the top of the package
			if d.refState.visible[i].Package != targetPkg {
				// select the previous reference...
				m.Select(i + 1)
				break
			}
		}
	}

	return d.refState.update(m)
}

func (d *listItemDelegate) onNavigate(m *list.Model) tea.Cmd {
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
	d.cursorScopeSize = markChildren(selected, selectedIdx, d.visibleItems(m), d.cursorScope, false, d.refState.children)

	if len(d.cursorScope) > 0 && len(d.userSelect) == 0 {
		// special case: if the user is hovering over an item, but has not selected any items, then these are implicitly selected
		// in case the user hits "enter".
		return d.selectedTestReferences(m, false)
	}
	return nil
}

func (d *listItemDelegate) onSelectAll(m *list.Model, isAllAlreadySelected bool) tea.Cmd {
	// select all items that can be seen in the list (on all pages)
	// or if they are already all selected, then unselect them all

	markAll(d.visibleItems(m), d.userSelect, isAllAlreadySelected)

	return d.selectedTestReferences(m, false)
}

func (d *listItemDelegate) onToggleMultiselect(m *list.Model) tea.Cmd {
	d.cursorScope = make(map[gotest.Reference]struct{}) // reset!

	selectedIdx, selected := d.selectedItem(m)
	var invert bool
	if _, ok := d.userSelect[selected.ref]; ok {
		delete(d.userSelect, selected.ref)
		invert = true
	} else {
		d.userSelect[selected.ref] = struct{}{}
	}

	markChildren(selected, selectedIdx, d.visibleItems(m), d.userSelect, invert, d.refState.children)

	return d.selectedTestReferences(m, false)
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

func (d listItemDelegate) selectedTestReferences(m *list.Model, finished bool) tea.Cmd {
	var cmds []tea.Cmd
	refs := d.selectedReferences()
	// if finished {
	//	cmds = append(cmds, d.refState.finish(m, refs))
	//}
	cmds = append(cmds, func() tea.Msg {
		return uievent.SelectedTestReferences{
			Finished: finished,
			Refs:     refs,
		}
	})

	return tea.Batch(cmds...)
}

func (d listItemDelegate) selectedReferences() []gotest.Reference {
	var refs []gotest.Reference

	for ref := range d.userSelect {
		if ref.Package == "*" {
			continue // don't include the all tests package
		}
		refs = append(refs, ref)
	}

	if len(refs) == 0 {
		// the user hasn't selected anything, but is hovering over something... we'll use that
		for ref := range d.cursorScope {
			if ref.Package == "*" {
				continue // don't include the all tests package
			}
			refs = append(refs, ref)
		}
	}

	sort.Sort(gotest.References(refs))

	return refs
}

func markChildren(selected item, start int, items []item, marker map[gotest.Reference]struct{}, invert bool, children map[gotest.Reference][]gotest.Reference) int {
	if selected.ref.Package == "*" {
		return markAll(items, marker, invert)
	}
	count := 0
	for i := start; i < len(items); i++ {
		it := items[i]

		if it.ref.IsPackage() {
			if isChild(&selected.ref, &it.ref) {
				if invert {
					delete(marker, it.ref)
				} else {
					marker[it.ref] = struct{}{}
					count++
				}
				// mark by what is defined within the package (not by what is visible)
				for _, child := range children[it.ref] {
					if invert {
						delete(marker, child)
					} else {
						marker[child] = struct{}{}
						count++
					}
				}
			}
		} else {
			// mark by what is visible, since this is a test function
			if isChild(&selected.ref, &it.ref) {
				if invert {
					delete(marker, it.ref)
				} else {
					marker[it.ref] = struct{}{}
					count++
				}
			} else {
				break
			}
		}
	}
	return count
}

func isChild(ref, other *gotest.Reference) bool {
	if other == nil || ref == nil {
		return false
	}
	if ref.Package != other.Package {
		return false
	}

	if ref.IsPackage() {
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

func markAll(items []item, marker map[gotest.Reference]struct{}, invert bool) int {
	count := 0
	for i := 0; i < len(items); i++ {
		it := items[i]

		if invert {
			delete(marker, it.ref)
		} else {
			marker[it.ref] = struct{}{}
			count++
		}
	}
	return count
}

func (d listItemDelegate) Render(w io.Writer, m list.Model, idx int, i list.Item) {
	it := i.(item)

	_, isHovering := d.cursorScope[it.ref]
	isHovering = isHovering && d.cursorScopeSize > 1
	_, isUserSelected := d.userSelect[it.ref]

	if m.Index() == idx && !d.finished {
		// cursor is hovering on this exact item (the cursor line)
		w = internal.NewIndentWriter(w, d.styles.hoverBullet.Render(" ❯"))
		d.Styles.SelectedTitle = d.styles.cursorLine
		switch {
		case isUserSelected:
			d.Styles.SelectedTitle = d.Styles.SelectedTitle.BorderStyle(userSelectedBorder)
		case isHovering:
			d.Styles.SelectedTitle = d.Styles.SelectedTitle.BorderStyle(hoverBorder)
		default:
			d.Styles.SelectedTitle = d.Styles.SelectedTitle.BorderStyle(emptyBorder)
		}
	} else {
		w = internal.NewIndentWriter(w, "  ")
	}

	if it.ref.Package == "*" {
		d.Styles.NormalTitle = d.styles.allTestsLine
	}

	if d.finished {
		d.Styles.NormalTitle = d.styles.normalStyle
		d.Styles.SelectedTitle = d.styles.normalStyle
	} else {
		if isUserSelected {
			d.Styles.NormalTitle = d.styles.userSelectedLine.BorderStyle(userSelectedBorder)
		} else if isHovering {
			// cursor is hovering over this item or a parent of this item
			d.Styles.NormalTitle = d.styles.hoverLine.BorderStyle(hoverBorder)
		}
	}

	// don't show matched characters when filtering is not occurring (including when the filter has been applied)
	// if m.FilterState() == list.Filtering {
	//	d.DefaultDelegate.Styles.FilterMatch = d.filterMatchStyle
	// } else {
	//	d.DefaultDelegate.Styles.FilterMatch = d.normalStyle
	//}

	d.DefaultDelegate.Render(
		w,
		m, idx, i,
	)
}
