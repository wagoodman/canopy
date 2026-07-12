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
	// hoverBorder is the border style used for items under the cursor scope.
	hoverBorder = lipgloss.Border{Left: "░"}
	// userSelectedBorder is the border style used for items explicitly selected by the user.
	userSelectedBorder = lipgloss.Border{Left: "█"}
	// emptyBorder is the border style used for items with no selection state.
	emptyBorder = lipgloss.Border{Left: " "}

	// matchColor highlights active filter matches and selection accents.
	matchColor = lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"}
	// selectionColor accents borders and text for user-selected items.
	selectionColor = lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}
)

// listItemDelegate is the custom item renderer for the test selector list.
// it manages item rendering, selection state, and user interactions with test references.
type listItemDelegate struct {
	list.DefaultDelegate

	// keyMap holds the key bindings for delegate actions.
	keyMap keyMap
	// styles contains the lipgloss styles for different item states.
	styles delegateStyles
	// finished indicates whether the selection process has completed.
	finished bool

	// refState manages the visibility and display format of test references.
	refState referenceState
	// current holds the test reference at the current cursor position.
	current *gotest.Reference
	// cursorScope contains all test references under the current cursor (parent and children).
	cursorScope map[gotest.Reference]struct{}
	// cursorScopeSize is the number of items in the cursor scope.
	cursorScopeSize int
	// userSelect contains all test references explicitly selected by the user.
	userSelect map[gotest.Reference]struct{}
}

// delegateStyles holds lipgloss styles for rendering list items in various states.
type delegateStyles struct {
	// hoverLine is the style for items within cursor scope but not selected.
	hoverLine lipgloss.Style
	// hoverBullet is the style for the cursor indicator bullet.
	hoverBullet lipgloss.Style
	// cursorLine is the style for the item directly under the cursor.
	cursorLine lipgloss.Style
	// userSelectedLine is the style for items explicitly selected by the user.
	userSelectedLine lipgloss.Style
	// allTestsLine is the style for the special "all tests" item.
	allTestsLine lipgloss.Style
	// normalStyle is the default style for unselected items.
	normalStyle lipgloss.Style
}

// newItemDelegate creates a new list item delegate with the given key bindings.
// it initializes selection tracking and configures rendering styles.
func newItemDelegate(keyMap keyMap) *listItemDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)
	d.Styles.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(matchColor)

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
				BorderForeground(selectionColor),

			hoverBullet: lipgloss.NewStyle().
				Foreground(selectionColor),

			cursorLine: baseStyle.
				BorderLeft(true).
				BorderForeground(selectionColor).
				Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}),

			userSelectedLine: baseStyle.
				BorderLeft(true).
				BorderForeground(selectionColor),

			allTestsLine: lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 0, 0, 2),
		},
	}
}

// Update handles incoming messages and updates the delegate state accordingly.
// it processes refresh events, state switches, mouse interactions, and keyboard input.
func (d *listItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, d.DefaultDelegate.Update(msg, m))

	switch msg := msg.(type) {
	case uievent.RefreshReferences:
		cmds = append(cmds, d.handleRefreshReferences(msg, m))

	case uievent.SwitchState:
		cmds = append(cmds, d.handleSwitchState(msg, m))

	case tea.MouseMsg:
		cmds = append(cmds, d.handleMouseEvent(msg, m))

	case tea.KeyMsg:
		cmds = append(cmds, d.handleKeyEvent(msg, m))
	}

	return tea.Batch(cmds...)
}

// handleRefreshReferences processes refresh reference events and updates display format.
func (d *listItemDelegate) handleRefreshReferences(msg uievent.RefreshReferences, m *list.Model) tea.Cmd {
	return d.refState.update(m, msg.AboutToFilter)
}

// handleSwitchState processes state switch events and sets up test definitions.
// it restores cursor position, initializes test definitions, and applies user selections.
func (d *listItemDelegate) handleSwitchState(msg uievent.SwitchState, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd

	// restore the last selected item position
	d.restoreLastSelection(m)

	// initialize test definitions
	d.initializeTestDefinitions(msg.Definitions)
	cmds = append(cmds, d.refState.update(m))

	// apply user selections from -run statements
	firstSelectedIdx := d.applyUserSelections(msg.Selected, m)

	if firstSelectedIdx >= 0 {
		m.Select(firstSelectedIdx) // move the cursor to the first selected item
	}
	cmds = append(cmds, d.onNavigate(m))

	return tea.Batch(cmds...)
}

// restoreLastSelection restores cursor to the last selected item if available.
func (d *listItemDelegate) restoreLastSelection(m *list.Model) {
	if d.current != nil {
		for idx, i := range m.Items() {
			it := i.(item)
			if it.ref == *d.current {
				m.Select(idx)
				break
			}
		}
	}
}

// initializeTestDefinitions sets up the reference state with new test definitions.
func (d *listItemDelegate) initializeTestDefinitions(definitions gotest.Definitions) {
	d.refState = newReferenceState(state.NewDefinitionViewer(definitions), d.refState)
}

// applyUserSelections marks items selected by user with -run statements and returns first selected index.
// it also marks all children of selected items.
func (d *listItemDelegate) applyUserSelections(selected []gotest.Reference, m *list.Model) int {
	selectedSet := make(map[gotest.Reference]struct{})
	for _, ref := range selected {
		d.userSelect[ref] = struct{}{}
		selectedSet[ref] = struct{}{}
	}

	firstSelectedIdx := -1
	for candidateIdx, candidate := range m.Items() {
		it := candidate.(item)
		if _, ok := selectedSet[it.ref]; ok {
			markChildren(it, candidateIdx, d.visibleItems(m), d.userSelect, false, d.refState.children)
			if firstSelectedIdx == -1 {
				firstSelectedIdx = candidateIdx // remember the first selected item
			}
		}
	}

	return firstSelectedIdx
}

// handleMouseEvent processes mouse interactions like scrolling and clicking.
func (d *listItemDelegate) handleMouseEvent(msg tea.MouseMsg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd

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

	return tea.Batch(cmds...)
}

// handleKeyEvent processes keyboard interactions including selection, navigation, and filtering.
func (d *listItemDelegate) handleKeyEvent(msg tea.KeyMsg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd

	// handle main key actions
	cmds = append(cmds, d.handleSelectionKeys(msg, m)...)
	cmds = append(cmds, d.handleNavigationKeys(msg, m)...)
	cmds = append(cmds, d.handleToggleKeys(msg, m)...)

	// handle filter keys if not actively filtering
	if !m.SettingFilter() {
		cmds = append(cmds, d.handleFilterKeys(msg, m)...)
	}

	return tea.Batch(cmds...)
}

// handleSelectionKeys processes test selection and finish actions.
func (d *listItemDelegate) handleSelectionKeys(msg tea.KeyMsg, m *list.Model) []tea.Cmd {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, d.keyMap.SelectTest):
		cmds = append(cmds, d.onToggleMultiselect(m))
		cmds = append(cmds, d.onNavigate(m))

	case key.Matches(msg, d.keyMap.SelectAllTests):
		isAllAlreadySelected := len(d.userSelect) == len(m.Items())
		cmds = append(cmds, d.onSelectAll(m, isAllAlreadySelected))

	case key.Matches(msg, d.keyMap.Finish):
		d.finished = true
		cmds = append(cmds, d.selectedTestReferences(true))
	}

	return cmds
}

// handleNavigationKeys processes cursor movement and package navigation.
func (d *listItemDelegate) handleNavigationKeys(msg tea.KeyMsg, m *list.Model) []tea.Cmd {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, d.keyMap.NextPackage):
		cmds = append(cmds, d.nextPkg(m))
		cmds = append(cmds, d.onNavigate(m))

	case key.Matches(msg, d.keyMap.PrevPackage):
		cmds = append(cmds, d.prevPkg(m))
		cmds = append(cmds, d.onNavigate(m))

	case key.Matches(msg, m.KeyMap.CursorDown, m.KeyMap.CursorUp, m.KeyMap.PrevPage, m.KeyMap.NextPage):
		cmds = append(cmds, d.onNavigate(m))
	}

	return cmds
}

// handleToggleKeys processes view toggle actions for reference format and test visibility.
func (d *listItemDelegate) handleToggleKeys(msg tea.KeyMsg, m *list.Model) []tea.Cmd {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, d.keyMap.ToggleReferenceLongForm):
		d.refState.preferLongForm = !d.refState.preferLongForm
		cmds = append(cmds, d.refState.update(m))

	case key.Matches(msg, d.keyMap.ToggleTests):
		d.refState.hideTests = !d.refState.hideTests
		cmds = append(cmds, d.refState.update(m))
	}

	return cmds
}

// handleFilterKeys processes text filtering input for letter characters.
func (d *listItemDelegate) handleFilterKeys(msg tea.KeyMsg, m *list.Model) []tea.Cmd {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, filterKeyBindings...):
		// if matches a-z, A-Z then we set the filter state to filtering. We should account for the missing input
		// character that the user just entered (as well as anything else that already may be applied in the filter text)
		m.SetFilterText(m.FilterInput.Value() + msg.String())
		m.SetFilterState(list.Filtering)
		cmds = append(cmds, d.refState.update(m))
	}

	return cmds
}

// nextPkg moves the cursor to the first item of the next package.
func (d *listItemDelegate) nextPkg(m *list.Model) tea.Cmd {
	// iterate the list's visible items (honors an active filter) rather than the
	// unfiltered refState.visible, since m.Index() is a filtered index
	items := d.visibleItems(m)
	currentIdx := m.Index()
	if currentIdx < 0 || currentIdx >= len(items) {
		return nil
	}
	curPkg := items[currentIdx].ref.Package
	for i := currentIdx; i < len(items); i++ {
		if items[i].ref.Package != curPkg {
			m.Select(i)
			break
		}
	}

	return d.refState.update(m)
}

// prevPkg moves the cursor to the first item of the previous package.
func (d *listItemDelegate) prevPkg(m *list.Model) tea.Cmd {
	items := d.visibleItems(m)
	currentIdx := m.Index()
	if currentIdx < 0 || currentIdx >= len(items) {
		return nil
	}
	curPkg := items[currentIdx].ref.Package
	targetPkg := ""
	for i := currentIdx; i >= 0; i-- {
		if targetPkg == "" {
			if items[i].ref.Package != curPkg {
				targetPkg = items[i].ref.Package
				continue
			}
		} else {
			// head to the top of the package
			if items[i].ref.Package != targetPkg {
				// select the previous reference...
				m.Select(i + 1)
				break
			}
		}
	}

	return d.refState.update(m)
}

// onNavigate updates cursor scope when the cursor moves to a new item.
// it marks all children of the current item as being within cursor scope for potential hover effects.
func (d *listItemDelegate) onNavigate(m *list.Model) tea.Cmd {
	currentItem := m.SelectedItem()
	if currentItem == nil {
		if len(m.Items()) == 0 {
			// nothing to navigate to (no references at all)
			return nil
		}
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
	selectedIdx, selected, ok := d.selectedItem(m)
	if !ok {
		return nil
	}
	d.cursorScopeSize = markChildren(selected, selectedIdx, d.visibleItems(m), d.cursorScope, false, d.refState.children)

	if len(d.cursorScope) > 0 && len(d.userSelect) == 0 {
		// special case: if the user is hovering over an item, but has not selected any items, then these are implicitly selected
		// in case the user hits "enter".
		return d.selectedTestReferences(false)
	}
	return nil
}

// onSelectAll selects or deselects all visible items in the list.
func (d *listItemDelegate) onSelectAll(m *list.Model, isAllAlreadySelected bool) tea.Cmd {
	// select all items that can be seen in the list (on all pages)
	// or if they are already all selected, then unselect them all

	markAll(d.visibleItems(m), d.userSelect, isAllAlreadySelected)

	return d.selectedTestReferences(false)
}

// onToggleMultiselect toggles selection state for the current item and its children.
func (d *listItemDelegate) onToggleMultiselect(m *list.Model) tea.Cmd {
	d.cursorScope = make(map[gotest.Reference]struct{}) // reset!

	selectedIdx, selected, ok := d.selectedItem(m)
	if !ok {
		return nil
	}
	var invert bool
	if _, ok := d.userSelect[selected.ref]; ok {
		delete(d.userSelect, selected.ref)
		invert = true
	} else {
		d.userSelect[selected.ref] = struct{}{}
	}

	markChildren(selected, selectedIdx, d.visibleItems(m), d.userSelect, invert, d.refState.children)

	return d.selectedTestReferences(false)
}

// visibleItems returns all currently visible items as a slice.
func (d listItemDelegate) visibleItems(m *list.Model) []item {
	var refs []item
	for _, it := range m.VisibleItems() {
		refs = append(refs, it.(item))
	}
	return refs
}

// selectedItem returns the index and item at the current cursor position.
func (d listItemDelegate) selectedItem(m *list.Model) (int, item, bool) {
	// guard the type assertion: SelectedItem() is nil when the visible list is empty
	it, ok := m.SelectedItem().(item)
	return m.Index(), it, ok
}

// selectedTestReferences returns a command that emits the currently selected test references.
// if finished is true, it signals that the user has confirmed their selection.
func (d listItemDelegate) selectedTestReferences(finished bool) tea.Cmd {
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

// selectedReferences builds the final list of selected test references.
// it uses explicitly selected items if any exist, otherwise falls back to cursor scope.
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

// markChildren marks an item and all of its children in the given marker map.
// if invert is true, it removes items from the marker instead of adding them.
// returns the count of items marked.
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

// isChild determines if other is a child of ref based on package and function hierarchy.
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

// markAll marks all items in the list.
// if invert is true, it removes all items from the marker instead.
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

// Render draws a single list item with appropriate styling based on selection state.
// it applies different styles for cursor hover, user selection, and filter matches.
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
