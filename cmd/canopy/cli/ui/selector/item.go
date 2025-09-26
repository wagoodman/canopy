package selector

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

const allTestsTitle = "(all available tests)"

// TODO: no global please...
var auxCasesStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

type item struct {
	title  string
	filter string
	ref    gotest.Reference
	tRuns  []string
}

func (i item) Title() string {
	return i.title
	// zone.Mark(i.title, i.title) }
}

func (i item) Description() string {
	return i.ref.Package
}

func (i item) FilterValue() string {
	return i.filter
	// zone.Mark(i.title, i.title) } // TODO: breaks filtering (when using in the filter value and not in the filter value... either messes with the lengths to select matched characters, or breaks rendering of patially matching ansi characters)
}

func newItems(showFullReference bool, pkgsOnly bool, refs ...gotest.Reference) []list.Item {
	var items []list.Item

	for i := 0; i < len(refs); {
		ref := refs[i]

		if ref.TRunName != "" {
			// skip t.Run cases that don't have a preceding function reference
			i++
			continue
		}

		if pkgsOnly && !ref.IsPackage() {
			// if we are only showing packages, skip any function references
			i++
			continue
		}

		var tRuns []string
		// look ahead to collect any t.Run cases for this function
		j := i + 1
		for j < len(refs) {
			nextRef := refs[j]
			if samePkg(ref, nextRef) && sameFunc(ref, nextRef) && nextRef.TRunName != "" {
				tRuns = append(tRuns, nextRef.TRunName)
				j++
			} else {
				// we have reached the end of the test runs for this function
				break
			}
		}

		// create the item for the function (with collected t.Run names)
		it := item{title: display(ref, tRuns, showFullReference), filter: fullReferenceTitle(ref, tRuns), ref: ref, tRuns: tRuns}
		items = append(items, it)

		// skip past the t.Run cases we just processed
		i = j
	}
	return items
}

func display(ref gotest.Reference, tRuns []string, showFullReference bool) string {
	if ref.Package == "*" {
		return allTestsTitle
	}

	if showFullReference {
		return fullReferenceTitle(ref, tRuns)
	}

	return treeTitle(ref, tRuns)
}

func fullReferenceTitle(ref gotest.Reference, tRuns []string) string {
	var tRunsStr string
	if len(tRuns) > 0 {
		tRunsStr = auxCasesStyle.Render(fmt.Sprintf(" (%d cases)", len(tRuns)))
		// tRunsStr = fmt.Sprintf(" (%d cases)", len(tRuns))
	}
	return ref.String(true) + tRunsStr
}

func treeTitle(ref gotest.Reference, tRuns []string) string {
	var tRunsStr string
	if len(tRuns) > 0 {
		tRunsStr = auxCasesStyle.Render(fmt.Sprintf(" (%d cases)", len(tRuns)))
		// tRunsStr = fmt.Sprintf(" (%d cases)", len(tRuns))
	}

	if ref.FuncName != "" {
		return fmt.Sprintf(" • %s%s", ref.FuncName, tRunsStr)
	}

	return ref.Package
}

func samePkg(a gotest.Reference, b gotest.Reference) bool {
	return a.Package == b.Package
}

func sameFunc(a gotest.Reference, b gotest.Reference) bool {
	return a.FuncName == b.FuncName
}
