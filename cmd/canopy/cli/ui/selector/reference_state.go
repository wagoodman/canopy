package selector

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"sort"
)

type referenceState struct {
	state          state.DefinitionViewer
	children       map[gotest.Reference][]gotest.Reference
	stateCount     int
	visible        []gotest.Reference
	preferLongForm bool
	hideTests      bool
}

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

func (d *referenceState) finish(m *list.Model, refs []gotest.Reference) tea.Cmd {
	if d.state == nil {
		return nil
	}

	return d.setReferences(m, false, true, refs...)
}

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

func filterToVisibleRefs(finished bool, original []gotest.Reference) []gotest.Reference {
	var refs []gotest.Reference
	if !finished {
		refs = append(refs, gotest.Reference{Package: "*"})
	}
	refs = append(refs, original...)

	return refs
}
