package selector

import (
	"sort"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// referenceState manages the visibility and display format of test references in the selector.
// it tracks which references are currently visible and how they should be rendered.
type referenceState struct {
	// state provides access to the underlying test definitions.
	state state.DefinitionViewer
	// children maps each reference to its child references (e.g., package -> tests, test -> t.Run cases).
	children map[gotest.Reference][]gotest.Reference
	// stateCount is the total number of references in the current state.
	stateCount int
	// visible is the list of references currently shown in the UI.
	visible []gotest.Reference
	// preferLongForm controls whether to show full package paths or short names.
	preferLongForm bool
	// hideTests controls whether to show only packages or include test functions.
	hideTests bool
}

// newReferenceState creates a new reference state from the given definition viewer.
// it preserves preferLongForm and hideTests settings from previous states if provided.
func newReferenceState(state state.DefinitionViewer, others ...referenceState) referenceState {
	var preferLongForm, hideTests bool
	if len(others) > 0 {
		// if we have other states, we will use the first one to determine the preferLongForm setting
		preferLongForm = others[0].preferLongForm
		hideTests = others[0].hideTests
	}
	refs := state.References()
	return referenceState{
		state:          state,
		children:       mapAllChildren(refs),
		stateCount:     len(refs),
		visible:        []gotest.Reference{},
		preferLongForm: preferLongForm,
		hideTests:      hideTests,
	}
}

// mapAllChildren builds a map of parent-child relationships for all references.
// each reference is mapped to its list of direct children.
func mapAllChildren(refs []gotest.Reference) map[gotest.Reference][]gotest.Reference {
	children := make(map[gotest.Reference][]gotest.Reference)
	for _, ref := range refs {
		parent := ref.ParentRef()
		if parent != nil {
			children[*parent] = append(children[*parent], ref)
		}
	}
	return children
}

// update refreshes the list model with the current reference state.
// aboutToFilter controls whether references should be displayed in long form for filtering.
func (d *referenceState) update(m *list.Model, aboutToFilters ...bool) tea.Cmd {
	if d.state == nil {
		return nil
	}

	var aboutToFilter bool
	if len(aboutToFilters) > 0 {
		aboutToFilter = aboutToFilters[0]
	}

	return d.setReferences(m, aboutToFilter, false, d.state.References()...)
}

// func (d *referenceState) finish(m *list.Model, refs []gotest.Reference) tea.Cmd {
//	if d.state == nil {
//		return nil
//	}
//
//	return d.setReferences(m, false, true, refs...)
//}

// setReferences updates the list model with a new set of references.
// it determines the appropriate display format based on filtering and completion state.
func (d *referenceState) setReferences(m *list.Model, aboutToFilter, finished bool, refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	d.visible = filterToVisibleRefs(finished, refs)

	return tea.Batch(
		m.SetItems(
			newItems(
				finished || aboutToFilter || d.preferLongForm || m.IsFiltered() || m.SettingFilter(),
				d.hideTests,
				d.visible...,
			),
		),
	)
}

// filterToVisibleRefs determines which references should be visible in the UI.
// it adds a special "all tests" entry when not finished.
func filterToVisibleRefs(finished bool, original []gotest.Reference) []gotest.Reference {
	var refs []gotest.Reference
	if !finished {
		refs = append(refs, gotest.Reference{Package: "*"})
	}
	refs = append(refs, original...)

	return refs
}
