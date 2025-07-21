package selector

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestNewItems(t *testing.T) {
	tests := []struct {
		name     string
		filter   bool
		refs     []gotest.Reference
		expected []expectedItem
		wantErr  require.ErrorAssertionFunc
	}{
		{
			name:     "empty references",
			filter:   false,
			refs:     []gotest.Reference{},
			expected: []expectedItem{},
		},
		{
			name:   "single package reference",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg", pkg: "github.com/example/pkg", funcName: "", tRunCount: 0},
			},
		},
		{
			name:   "single test function",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 0},
			},
		},
		{
			name:   "test function with single t.Run - t.Run should be excluded from items",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 1},
			},
		},
		{
			name:   "test function with multiple t.Run cases - all t.Run should be excluded",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest3"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 3},
			},
		},
		{
			name:   "multiple test functions with t.Run cases",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2", TRunName: "subtest3"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc1", pkg: "github.com/example/pkg", funcName: "TestFunc1", tRunCount: 2},
				{title: "github.com/example/pkg/TestFunc2", pkg: "github.com/example/pkg", funcName: "TestFunc2", tRunCount: 1},
			},
		},
		{
			name:   "different packages with t.Run cases",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc", TRunName: "subtest1"},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: "subtest2"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1/TestFunc", pkg: "github.com/example/pkg1", funcName: "TestFunc", tRunCount: 1},
				{title: "github.com/example/pkg2/TestFunc", pkg: "github.com/example/pkg2", funcName: "TestFunc", tRunCount: 1},
			},
		},
		{
			name:   "mixed references - packages, functions, and t.Run cases",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc2", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1", pkg: "github.com/example/pkg1", funcName: "", tRunCount: 0},
				{title: "github.com/example/pkg1/TestFunc1", pkg: "github.com/example/pkg1", funcName: "TestFunc1", tRunCount: 1},
				{title: "github.com/example/pkg2/TestFunc2", pkg: "github.com/example/pkg2", funcName: "TestFunc2", tRunCount: 0},
			},
		},
		{
			name:   "edge case: t.Run without preceding function (should be skipped)",
			filter: false,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"}, // starts with t.Run
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 0},
			},
		},
		{
			name:   "all tests special case",
			filter: false,
			refs: []gotest.Reference{
				{Package: "*", FuncName: "", TRunName: ""},
			},
			expected: []expectedItem{
				{title: allTestsTitle, pkg: "*", funcName: "", tRunCount: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			result := newItems(tt.filter, tt.refs...)

			if err := validateResult(result, tt.expected); err != nil {
				t.Fatalf("validation failed: %v", err)
			}

			// verify result structure
			require.Len(t, result, len(tt.expected), "unexpected number of items")

			for i, expected := range tt.expected {
				actualItem, ok := result[i].(item)
				require.True(t, ok, "item at index %d is not of type item", i)

				require.Equal(t, expected.title, actualItem.Title(), "item %d title mismatch", i)
				require.Equal(t, expected.pkg, actualItem.ref.Package, "item %d package mismatch", i)
				require.Equal(t, expected.funcName, actualItem.ref.FuncName, "item %d function name mismatch", i)
				require.Len(t, actualItem.tRuns, expected.tRunCount, "item %d t.Run count mismatch", i)
			}
		})
	}
}

type expectedItem struct {
	title     string
	pkg       string
	funcName  string
	tRunCount int
}

// validateResult performs additional validation to catch edge cases and bugs
func validateResult(result []list.Item, expected []expectedItem) error {
	// verify no t.Run items are included in the result
	for i, listItem := range result {
		actualItem, ok := listItem.(item)
		if !ok {
			continue
		}
		if actualItem.ref.TRunName != "" {
			return fmt.Errorf("found t.Run item at index %d: %s (TRunName: %s) - t.Run cases should be excluded from items",
				i, actualItem.Title(), actualItem.ref.TRunName)
		}
	}

	return nil
}