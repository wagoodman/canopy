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

// item represents a single test reference in the selector list.
// it implements the list.Item interface for use with Bubble Tea's list component.
type item struct {
	// title is the display text shown in the list.
	title string
	// filter is the full reference text used for filtering operations.
	filter string
	// ref is the test reference this item represents.
	ref gotest.Reference
	// tRuns contains the names of all t.Run sub-tests for this test function.
	tRuns []string
}

// Title returns the display title for this item.
func (i item) Title() string {
	return i.title
	// zone.Mark(i.title, i.title) }
}

// Description returns the package path as the item description.
func (i item) Description() string {
	return i.ref.Package
}

// FilterValue returns the text used for filtering this item.
// it uses the full reference format to enable filtering by package and function name.
func (i item) FilterValue() string {
	return i.filter
	// zone.Mark(i.title, i.title) } // TODO: breaks filtering (when using in the filter value and not in the filter value... either messes with the lengths to select matched characters, or breaks rendering of patially matching ansi characters)
}

// newItems creates a list of items from the given test references.
// showFullReference controls whether to display full package paths or tree-style names.
// pkgsOnly filters the list to only include package-level references.
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

// display returns the formatted display text for a test reference.
// it chooses between full reference format and tree format based on showFullReference.
func display(ref gotest.Reference, tRuns []string, showFullReference bool) string {
	if ref.Package == "*" {
		return allTestsTitle
	}

	if showFullReference {
		return fullReferenceTitle(ref, tRuns)
	}

	return treeTitle(ref, tRuns)
}

// fullReferenceTitle returns the complete reference path including package and function.
// it includes a count of t.Run sub-tests if any exist.
func fullReferenceTitle(ref gotest.Reference, tRuns []string) string {
	var tRunsStr string
	if len(tRuns) > 0 {
		tRunsStr = auxCasesStyle.Render(fmt.Sprintf(" (%d cases)", len(tRuns)))
		// tRunsStr = fmt.Sprintf(" (%d cases)", len(tRuns))
	}
	return ref.String(true) + tRunsStr
}

// treeTitle returns a tree-style display format with indentation for hierarchy.
// packages are shown without indentation, functions are shown with a bullet point.
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

// samePkg returns true if both references belong to the same package.
func samePkg(a gotest.Reference, b gotest.Reference) bool {
	return a.Package == b.Package
}

// sameFunc returns true if both references refer to the same test function.
func sameFunc(a gotest.Reference, b gotest.Reference) bool {
	return a.FuncName == b.FuncName
}
