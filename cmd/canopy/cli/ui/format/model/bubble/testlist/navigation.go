package testlist

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) nextPkg() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	for i := currentIdx; i < len(m.visibleRefs); i++ {
		if m.visibleRefs[i].Package != curPkg {
			m.list.Select(i)
			break
		}
	}

	return m.refreshRun()
}

func (m *Model) prevPkg() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	targetPkg := ""
	for i := currentIdx; i >= 0; i-- {
		if targetPkg == "" {
			if m.visibleRefs[i].Package != curPkg {
				targetPkg = m.visibleRefs[i].Package
				continue
			}
		} else {
			// head to the top of the package
			if m.visibleRefs[i].Package != targetPkg {
				// select the previous reference...
				m.list.Select(i + 1)
				break
			}
		}
	}

	return m.refreshRun()
}

func (m *Model) nextTestFn() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	curTestFn := currentItem.ref.FuncName
	for i := currentIdx; i < len(m.visibleRefs); i++ {
		if m.visibleRefs[i].FuncName == "" {
			continue
		}
		if m.visibleRefs[i].Package != curPkg || m.visibleRefs[i].FuncName != curTestFn {
			m.list.Select(i)
			break
		}
	}
	return m.refreshRun()
}

func (m *Model) prevTestFn() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	curTestFn := currentItem.ref.FuncName
	targetTestFn := ""
	for i := currentIdx; i >= 0; i-- {
		if targetTestFn == "" {
			if m.visibleRefs[i].FuncName == "" {
				continue
			}

			if m.visibleRefs[i].Package != curPkg || m.visibleRefs[i].FuncName != curTestFn {
				targetTestFn = m.visibleRefs[i].FuncName
				continue
			}
		} else if m.visibleRefs[i].FuncName != targetTestFn {
			m.list.Select(i + 1)
			break
		}
	}
	return m.refreshRun()
}
