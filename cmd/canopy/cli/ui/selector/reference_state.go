package selector

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"sort"
)

type referenceState struct {
	state       state.DefinitionViewer
	visibleRefs []gotest.Reference
}

func (d *referenceState) update(m *list.Model) tea.Cmd {
	if d.state == nil {
		return nil
	}

	return d.setReferences(m, d.state.References()...)
}

func (d *referenceState) setReferences(m *list.Model, refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	d.visibleRefs = filterToVisibleRefs(refs)

	return tea.Batch(
		m.SetItems(newItems(m.IsFiltered() || m.FilterState() == list.Filtering, d.visibleRefs...)),
	)
}

func filterToVisibleRefs(original []gotest.Reference) []gotest.Reference {
	var refs []gotest.Reference
	refs = append(refs, gotest.Reference{Package: "*"})
	refs = append(refs, original...)

	return refs
}
