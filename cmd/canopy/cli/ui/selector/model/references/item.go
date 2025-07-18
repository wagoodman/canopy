package references

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	zone "github.com/lrstanley/bubblezone"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

const allTestsTitle = "(all available tests)"

type item struct {
	title     string
	ref       gotest.Reference
	tRuns     []string
	filtering bool
}

//func (i item) Title() string       { return i.title }
//func (i item) Description() string { return "" }
//func (i item) FilterValue() string { return i.title }

func (i item) Title() string {
	return zone.Mark(i.title, i.display())
}

func (i item) display() string {
	if i.ref.Package == "*" {
		return allTestsTitle
	}

	if i.filtering {
		return i.filterTitle()
	}

	return i.treeTitle()
}

func (i item) treeTitle() string {
	var tRuns string
	if len(i.tRuns) > 0 {
		tRuns = fmt.Sprintf(" (%d cases)", len(i.tRuns))
	}

	if i.ref.FuncName != "" {
		return fmt.Sprintf(" • %s%s", i.ref.FuncName, tRuns)
	}

	return i.ref.Package
}

func (i item) filterTitle() string {
	return i.ref.String(true)
}

func (i item) Description() string { return "" }
func (i item) FilterValue() string {
	if i.ref.Package == "*" {
		return allTestsTitle
	}
	return i.display()
}

func newItems(filter bool, refs ...gotest.Reference) []list.Item {
	var items []list.Item
	var lastRef *gotest.Reference
	var offset int
	for i := 0; i+offset < len(refs); i++ {
		ref := refs[i+offset]

		var title string
		if ref.Package == "*" {
			title = allTestsTitle
		} else {
			title = ref.String(true)
		}

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
			it := item{title: title, ref: ref, tRuns: tRuns, filtering: filter}
			items = append(items, it)
			lastRef = &it.ref
		}
	}
	return items
}

func samePkg(a gotest.Reference, b gotest.Reference) bool {
	return a.Package == b.Package
}

func sameFunc(a gotest.Reference, b gotest.Reference) bool {
	return a.FuncName == b.FuncName
}
