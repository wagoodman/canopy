package referencespane

import (
	"fmt"
	"io"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// TODO: this is a bad global, please delete me
var isFiltering bool

type item struct {
	id         string
	ref        gotest.Reference
	tRunBranch string
}

type listItemDelegate struct {
	list.DefaultDelegate

	navigateBindings []key.Binding

	highlightedStyle       lipgloss.Style
	highlightedFailedStyle lipgloss.Style
	highlightedRunStyle    lipgloss.Style
	highlightedSkipStyle   lipgloss.Style
	highlightedPassStyle   lipgloss.Style

	multiSelectStyle       lipgloss.Style
	multiSelectFailedStyle lipgloss.Style
	multiSelectRunStyle    lipgloss.Style
	multiSelectSkipStyle   lipgloss.Style
	multiSelectPassStyle   lipgloss.Style

	passStyle     lipgloss.Style
	failStyle     lipgloss.Style
	runStyle      lipgloss.Style
	skipStyle     lipgloss.Style
	allTestsStyle lipgloss.Style

	current     *gotest.Reference
	cursorScope map[gotest.Reference]struct{}
	multiSelect map[gotest.Reference]struct{}

	state state.RunViewer
}

func listKeyMap() list.KeyMap {
	return list.KeyMap{
		// Browsing.
		CursorUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑/k", "up"),
		),
		CursorDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓/j", "down"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "prev page"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "next page"),
		),
		GoToStart: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "go to start"),
		),
		GoToEnd: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "go to end"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear filter"),
		),

		// Filtering.
		CancelWhileFiltering: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		AcceptWhileFiltering: key.NewBinding(
			key.WithKeys("enter", "tab", "shift+tab", "ctrl+k", "up", "ctrl+j", "down"),
			key.WithHelp("enter", "apply filter"),
		),

		//// Toggle help.
		//ShowFullHelp: key.NewBinding(
		//	key.WithKeys("?"),
		//	key.WithHelp("?", "more"),
		// ),
		//CloseFullHelp: key.NewBinding(
		//	key.WithKeys("?"),
		//	key.WithHelp("?", "close help"),
		// ),
		//
		//// Quitting.
		//Quit: key.NewBinding(
		//	key.WithKeys("q", "esc"),
		//	key.WithHelp("q", "quit"),
		// ),
		//ForceQuit: key.NewBinding(key.WithKeys("ctrl+c")),
	}
}

func newList(navigateBindings ...key.Binding) list.Model {
	l := list.New(newItems(gotest.Reference{Package: "*"}), newListItemDelegate(navigateBindings...), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowFilter(false)
	l.KeyMap = listKeyMap()
	return l
}

func newListItemDelegate(navigateBindings ...key.Binding) *listItemDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)

	// d.Styles.SelectedTitle = lipgloss.NewStyle().
	//	Border(lipgloss.HiddenBorder(), false, false, false, true).
	//	BorderBackground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
	//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
	//	Padding(0, 0, 0, 1)

	cursorBrd := lipgloss.NormalBorder()
	cursorBrd.Left = "\U0001FB6A" // ❯❱ ●• » ▚

	highlightPadding := lipgloss.NewStyle().Padding(0, 0, 0, 1)
	notHighlightedPadding := lipgloss.NewStyle().Padding(0, 0, 0, 2)

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

	failedStyle := notHighlightedPadding.Foreground(lipgloss.Color("9"))
	runStyle := notHighlightedPadding.Foreground(lipgloss.Color("11"))
	skipStyle := notHighlightedPadding.Faint(true)
	// passStyle := notHighlightedPadding.Foreground(lipgloss.Color("10"))
	passStyle := notHighlightedPadding

	return &listItemDelegate{
		navigateBindings: navigateBindings,
		DefaultDelegate:  d,
		multiSelect:      make(map[gotest.Reference]struct{}),
		cursorScope:      make(map[gotest.Reference]struct{}),
		//highlightedStyle: lipgloss.NewStyle().
		//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
		//	Padding(0, 0, 0, 2),
		highlightedStyle:       highlightStyle,
		highlightedFailedStyle: highlightStyle.Foreground(failedStyle.GetForeground()),
		highlightedRunStyle:    highlightStyle.Foreground(runStyle.GetForeground()),
		highlightedSkipStyle:   highlightStyle.Foreground(skipStyle.GetForeground()),
		highlightedPassStyle:   highlightStyle.Foreground(passStyle.GetForeground()),
		multiSelectStyle:       multiSelectStyle,
		multiSelectFailedStyle: multiSelectStyle.Foreground(failedStyle.GetForeground()),
		multiSelectRunStyle:    multiSelectStyle.Foreground(runStyle.GetForeground()),
		multiSelectSkipStyle:   multiSelectStyle.Foreground(skipStyle.GetForeground()),
		multiSelectPassStyle:   multiSelectStyle.Foreground(passStyle.GetForeground()),
		failStyle:              failedStyle,
		runStyle:               runStyle,
		skipStyle:              skipStyle,
		passStyle:              passStyle,
		allTestsStyle:          lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888")).Padding(0, 0, 0, 2),
	}
}

const allTestsTitle = "(all available tests)"

func (i item) Title() string {
	return zone.Mark(i.id, i.title())
}

func (i item) title() string {
	if i.ref.Package == "*" {
		return allTestsTitle
	}

	if isFiltering {
		return i.filterTitle()
	}

	return i.treeTitle()
}

func (i item) treeTitle() string {
	if i.ref.TRunName != "" {
		branch := i.tRunBranch
		if branch == "" {
			branch = "   "
		}
		return fmt.Sprintf("  %s%s", branch, i.ref.TRunName)
	}

	if i.ref.FuncName != "" {
		return fmt.Sprintf(" • %s", i.ref.FuncName)
	}

	return i.ref.Package
}

func (i item) filterTitle() string {
	name := i.ref.Package
	decl := "[pkg] "

	if i.ref.TRunName != "" {
		decl = "[case]"
		name = i.ref.TRunName
	} else if i.ref.FuncName != "" {
		decl = "[func]"
		name = i.ref.FuncName
	}

	return fmt.Sprintf("%s %s", decl, name)
}

func (i item) Description() string { return "" }
func (i item) FilterValue() string {
	if i.ref.Package == "*" {
		return allTestsTitle
	}
	return zone.Mark(i.id, i.id)
}

func newItems(refs ...gotest.Reference) []list.Item {
	items := make([]list.Item, len(refs))
	var next *gotest.Reference
	for i, ref := range refs {
		if i+1 < len(refs) {
			next = &refs[i+1]
		} else {
			next = nil
		}

		var id string
		if ref.Package == "*" {
			id = allTestsTitle
		} else {
			id = ref.String(true)
		}
		it := item{id: id, ref: ref}

		if ref.TRunName != "" {
			if samePkg(ref, next) && sameFunc(ref, next) && !sameTRun(ref, next) {
				it.tRunBranch = "  ├── "
			} else {
				it.tRunBranch = "  └── "
			}
		}

		items[i] = it
	}
	return items
}

func samePkg(a gotest.Reference, b *gotest.Reference) bool {
	if b == nil {
		return false
	}
	return a.Package == b.Package
}

func sameFunc(a gotest.Reference, b *gotest.Reference) bool {
	if b == nil {
		return false
	}
	return a.FuncName == b.FuncName
}

func sameTRun(a gotest.Reference, b *gotest.Reference) bool {
	if b == nil {
		return false
	}
	return a.TRunName == b.TRunName
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

func (d listItemDelegate) Render(w io.Writer, m list.Model, idx int, i list.Item) {
	it := i.(item)

	if it.ref.Package == "*" {
		d.Styles.NormalTitle = d.allTestsStyle
	}

	var act gotest.Action
	if d.state != nil {
		act = d.state.ReferenceConclusion(it.ref)
	}

	if _, ok := d.multiSelect[it.ref]; ok {
		// multi selected...
		switch act {
		case gotest.FailAction:
			d.Styles.NormalTitle = d.multiSelectFailedStyle
		case gotest.RunAction:
			d.Styles.NormalTitle = d.multiSelectRunStyle
		case gotest.SkipAction:
			d.Styles.NormalTitle = d.multiSelectSkipStyle
		default:
			d.Styles.NormalTitle = d.multiSelectStyle
		}
	} else if _, ok := d.cursorScope[it.ref]; ok {
		// highlighted...
		switch act {
		case gotest.FailAction:
			d.Styles.NormalTitle = d.highlightedFailedStyle
		case gotest.RunAction:
			d.Styles.NormalTitle = d.highlightedRunStyle
		case gotest.SkipAction:
			d.Styles.NormalTitle = d.highlightedSkipStyle
		default:
			d.Styles.NormalTitle = d.highlightedStyle
		}
	} else {
		// not highlighted...
		switch act {
		case gotest.FailAction:
			d.Styles.NormalTitle = d.failStyle
		case gotest.RunAction:
			d.Styles.NormalTitle = d.runStyle
		case gotest.SkipAction:
			d.Styles.NormalTitle = d.skipStyle
		case gotest.PassAction:
			d.Styles.NormalTitle = d.passStyle
		}
	}

	d.DefaultDelegate.Render(w, m, idx, i)
}

func (d *listItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, d.DefaultDelegate.Update(msg, m))

	switch msg := msg.(type) {
	case gotest.Event:
		cmds = append(cmds, d.onNavigate(m))

	case uievent.SwitchTestRun:
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

		d.state = state.NewRunViewer(msg.TestRun)
		cmds = append(cmds, d.onNavigate(m))

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
			cmds = append(cmds, d.onNavigate(m))
		default:
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				cmds = append(cmds, d.onNavigate(m))
			}
		}

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeySpace:
			cmds = append(cmds, d.onToggleMultiselect(m))
		case key.Matches(msg, m.KeyMap.CursorDown, m.KeyMap.CursorUp, m.KeyMap.PrevPage, m.KeyMap.NextPage, m.KeyMap.AcceptWhileFiltering, m.KeyMap.CancelWhileFiltering, m.KeyMap.Filter):
			cmds = append(cmds, d.onNavigate(m))
		case key.Matches(msg, d.navigateBindings...):
			cmds = append(cmds, d.onNavigate(m))
		}
	}

	return tea.Batch(cmds...)
}

func (d *listItemDelegate) onToggleMultiselect(m *list.Model) tea.Cmd {
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

	if len(d.multiSelect) == 0 {
		// we've deselected the last item in the multi-select... default to cursor scope
		return d.onNavigate(m)
	}

	return d.selectedTestReferencesCmd(m)
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
	// ogScope := d.cursorScope
	// changed := false
	if len(d.multiSelect) == 0 {
		d.cursorScope = make(map[gotest.Reference]struct{})
		selectedIdx, selected := d.selectedItem(m)
		markChildren(selected, selectedIdx, d.visibleItems(m), d.cursorScope, false)
		return d.selectedTestReferencesCmd(m)
	}
	return nil
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
