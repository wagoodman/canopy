package selector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type expectedItem struct {
	title     string
	pkg       string
	funcName  string
	tRunCount int
}

func TestNewItems(t *testing.T) {
	tests := []struct {
		name         string
		showFullRefs bool
		pkgsOnly     bool
		refs         []gotest.Reference
		expected     []expectedItem
		wantErr      require.ErrorAssertionFunc
	}{
		{
			name:         "empty references",
			showFullRefs: true,
			refs:         []gotest.Reference{},
			expected:     []expectedItem{},
		},
		{
			name:         "single package reference",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg", pkg: "github.com/example/pkg", funcName: "", tRunCount: 0},
			},
		},
		{
			name:         "single test function",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 0},
			},
		},
		{
			name:         "test function with single t.Run - t.Run should be excluded from items",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc (1 cases)", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 1},
			},
		},
		{
			name:         "test function with multiple t.Run cases - all t.Run should be excluded",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest3"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc (3 cases)", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 3},
			},
		},
		{
			name:         "multiple test functions with t.Run cases",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2", TRunName: "subtest3"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc1 (2 cases)", pkg: "github.com/example/pkg", funcName: "TestFunc1", tRunCount: 2},
				{title: "github.com/example/pkg/TestFunc2 (1 cases)", pkg: "github.com/example/pkg", funcName: "TestFunc2", tRunCount: 1},
			},
		},
		{
			name:         "different packages with t.Run cases",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc", TRunName: "subtest1"},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: ""},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: "subtest2"},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1/TestFunc (1 cases)", pkg: "github.com/example/pkg1", funcName: "TestFunc", tRunCount: 1},
				{title: "github.com/example/pkg2/TestFunc (1 cases)", pkg: "github.com/example/pkg2", funcName: "TestFunc", tRunCount: 1},
			},
		},
		{
			name:         "mixed references - packages, functions, and t.Run cases",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1", TRunName: ""},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg2", FuncName: "TestFunc2", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1", pkg: "github.com/example/pkg1", funcName: "", tRunCount: 0},
				{title: "github.com/example/pkg1/TestFunc1 (1 cases)", pkg: "github.com/example/pkg1", funcName: "TestFunc1", tRunCount: 1},
				{title: "github.com/example/pkg2/TestFunc2", pkg: "github.com/example/pkg2", funcName: "TestFunc2", tRunCount: 0},
			},
		},
		{
			name:         "edge case: t.Run without preceding function (should be skipped)",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: "subtest1"}, // starts with t.Run
				{Package: "github.com/example/pkg", FuncName: "TestFunc", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg/TestFunc", pkg: "github.com/example/pkg", funcName: "TestFunc", tRunCount: 0},
			},
		},
		{
			name:         "all tests special case",
			showFullRefs: true,
			refs: []gotest.Reference{
				{Package: "*", FuncName: "", TRunName: ""},
			},
			expected: []expectedItem{
				{title: allTestsTitle, pkg: "*", funcName: "", tRunCount: 0},
			},
		},
		{
			name:     "pkgsOnly: mixed references - only packages should be included",
			pkgsOnly: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "", TRunName: ""},          // package - should be included
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1", TRunName: ""}, // function - should be excluded
				{Package: "github.com/example/pkg2", FuncName: "", TRunName: ""},          // package - should be included
				{Package: "github.com/example/pkg2", FuncName: "TestFunc2", TRunName: ""}, // function - should be excluded
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1", pkg: "github.com/example/pkg1", funcName: "", tRunCount: 0},
				{title: "github.com/example/pkg2", pkg: "github.com/example/pkg2", funcName: "", tRunCount: 0},
			},
		},
		{
			name:     "pkgsOnly: only functions - should return empty",
			pkgsOnly: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: ""},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2", TRunName: ""},
			},
			expected: []expectedItem{},
		},
		{
			name:     "pkgsOnly: only packages - all should be included",
			pkgsOnly: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "", TRunName: ""},
				{Package: "github.com/example/pkg2", FuncName: "", TRunName: ""},
				{Package: "github.com/example/pkg3", FuncName: "", TRunName: ""},
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1", pkg: "github.com/example/pkg1", funcName: "", tRunCount: 0},
				{title: "github.com/example/pkg2", pkg: "github.com/example/pkg2", funcName: "", tRunCount: 0},
				{title: "github.com/example/pkg3", pkg: "github.com/example/pkg3", funcName: "", tRunCount: 0},
			},
		},
		{
			name:     "pkgsOnly: functions with t.Run cases - should be excluded",
			pkgsOnly: true,
			refs: []gotest.Reference{
				{Package: "github.com/example/pkg1", FuncName: "", TRunName: ""},                 // package - should be included
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: ""},         // function - should be excluded
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: "subtest1"}, // t.Run - should be excluded
				{Package: "github.com/example/pkg2", FuncName: "TestFunc", TRunName: "subtest2"}, // t.Run - should be excluded
			},
			expected: []expectedItem{
				{title: "github.com/example/pkg1", pkg: "github.com/example/pkg1", funcName: "", tRunCount: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			result := newItems(tt.showFullRefs, tt.pkgsOnly, tt.refs...)

			// verify no t.Run items are included in the result
			for i, listItem := range result {
				actualItem, ok := listItem.(item)
				if !ok {
					continue
				}
				if actualItem.ref.TRunName != "" {
					t.Errorf("found t.Run item at index %d: %s (TRunName: %s) - t.Run cases should be excluded from items",
						i, actualItem.Title(), actualItem.ref.TRunName)
				}
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
