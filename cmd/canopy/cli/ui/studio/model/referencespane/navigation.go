package referencespane

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// visibleRefsFromList returns the refs of the list's currently visible items,
// honoring an active text filter. m.list.Index() is an index into this set,
// not into m.visibleRefs (which is only status-filtered), so navigation must
// walk these items to map the cursor back to a reference.
func (m *Model) visibleRefsFromList() []gotest.Reference {
	visible := m.list.VisibleItems()
	refs := make([]gotest.Reference, 0, len(visible))
	for _, it := range visible {
		refs = append(refs, it.(item).ref)
	}
	return refs
}

// nextPkg moves the selection to the first reference of the next package in the list.
func (m *Model) nextPkg() tea.Cmd {
	refs := m.visibleRefsFromList()
	currentIdx := m.list.Index()
	if currentIdx < 0 || currentIdx >= len(refs) {
		return nil
	}
	curPkg := refs[currentIdx].Package
	for i := currentIdx; i < len(refs); i++ {
		if refs[i].Package != curPkg {
			m.list.Select(i)
			break
		}
	}

	return m.refreshRun()
}

// prevPkg moves the selection to the first reference of the previous package in the list.
func (m *Model) prevPkg() tea.Cmd {
	refs := m.visibleRefsFromList()
	currentIdx := m.list.Index()
	if currentIdx < 0 || currentIdx >= len(refs) {
		return nil
	}
	curPkg := refs[currentIdx].Package
	targetPkg := ""
	for i := currentIdx; i >= 0; i-- {
		if targetPkg == "" {
			if refs[i].Package != curPkg {
				targetPkg = refs[i].Package
				continue
			}
		} else {
			// head to the top of the package
			if refs[i].Package != targetPkg {
				// select the previous reference...
				m.list.Select(i + 1)
				break
			}
		}
	}

	return m.refreshRun()
}

// nextTestFn moves the selection to the next test function in the list.
func (m *Model) nextTestFn() tea.Cmd {
	refs := m.visibleRefsFromList()
	currentIdx := m.list.Index()
	if currentIdx < 0 || currentIdx >= len(refs) {
		return nil
	}
	curPkg := refs[currentIdx].Package
	curTestFn := refs[currentIdx].FuncName
	for i := currentIdx; i < len(refs); i++ {
		if refs[i].FuncName == "" {
			continue
		}
		if refs[i].Package != curPkg || refs[i].FuncName != curTestFn {
			m.list.Select(i)
			break
		}
	}
	return m.refreshRun()
}

// prevTestFn moves the selection to the previous test function in the list.
func (m *Model) prevTestFn() tea.Cmd {
	refs := m.visibleRefsFromList()
	currentIdx := m.list.Index()
	if currentIdx < 0 || currentIdx >= len(refs) {
		return nil
	}
	curPkg := refs[currentIdx].Package
	curTestFn := refs[currentIdx].FuncName
	targetTestFn := ""
	for i := currentIdx; i >= 0; i-- {
		if targetTestFn == "" {
			if refs[i].FuncName == "" {
				continue
			}

			if refs[i].Package != curPkg || refs[i].FuncName != curTestFn {
				targetTestFn = refs[i].FuncName
				continue
			}
		} else if refs[i].FuncName != targetTestFn {
			m.list.Select(i + 1)
			break
		}
	}
	return m.refreshRun()
}
