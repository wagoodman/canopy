package selector

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	zone "github.com/lrstanley/bubblezone"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

const allTestsTitle = "(all available tests)"

type item struct {
	title string
	ref   gotest.Reference
	tRuns []string
}

func (i item) Title() string       { return zone.Mark(i.title, i.title) }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return zone.Mark(i.title, i.title) }

func newItems(filter bool, refs ...gotest.Reference) []list.Item {
	var items []list.Item
	var lastRef *gotest.Reference
	var offset int
	for i := 0; i+offset < len(refs); i++ {
		ref := refs[i+offset]

		var tRuns []string
		// we don't include t.Run cases, instead they are pruned and the t.Run name is added to the func test item
		if ref.TRunName != "" && lastRef != nil {
			for j := i + offset; j < len(refs); j++ {
				if samePkg(ref, *lastRef) && sameFunc(ref, *lastRef) && refs[j].TRunName != "" {
					tRuns = append(tRuns, refs[j].TRunName)
				} else {
					// we have reached the end of the test runs for this function
					break
				}
				offset++
			}
		} else {
			it := item{title: display(ref, tRuns, filter), ref: ref, tRuns: tRuns}
			items = append(items, it)
			lastRef = &it.ref
		}
	}
	return items
}

func display(ref gotest.Reference, tRuns []string, filtering bool) string {
	if ref.Package == "*" {
		return allTestsTitle
	}

	if filtering {
		return filterTitle(ref)
	}

	return treeTitle(ref, tRuns)
}

func filterTitle(ref gotest.Reference) string {
	return ref.String(true)
}

func treeTitle(ref gotest.Reference, tRuns []string) string {
	var tRunsStr string
	if len(tRuns) > 0 {
		tRunsStr = fmt.Sprintf(" (%d cases)", len(tRuns))
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
