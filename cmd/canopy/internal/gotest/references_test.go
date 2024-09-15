package gotest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMinimizeSelection(t *testing.T) {
	all := []Reference{
		{"pkg1", "TestFunc1", "Case1"},
		{"pkg1", "TestFunc1", "Case2"},
		{"pkg1", "TestFunc2", "Case1"},
		{"pkg1", "TestFunc2", "Case2"},
		{"pkg1", "TestFunc3", "Case1"},
		{"pkg1", "TestFunc3", "Case2"},
		{"pkg2", "TestFunc1", "Case1"},
		{"pkg2", "TestFunc1", "Case2"},
	}

	tests := []struct {
		name           string
		selected       []Reference
		expectedResult []Reference
	}{
		{
			name: "select a single case",
			selected: []Reference{
				{"pkg1", "TestFunc2", "Case1"},
			},
			expectedResult: []Reference{
				{"pkg1", "TestFunc2", "Case1"},
			},
		},
		{
			name: "select a single test function",
			selected: []Reference{
				{"pkg1", "TestFunc1", "Case1"},
				{"pkg1", "TestFunc1", "Case2"},
			},
			expectedResult: []Reference{
				{"pkg1", "TestFunc1", ""},
			},
		},
		{
			name: "select a single package",
			selected: []Reference{
				{"pkg2", "TestFunc1", "Case1"},
				{"pkg2", "TestFunc1", "Case2"},
			},
			expectedResult: []Reference{
				{"pkg2", "", ""},
			},
		},
		{
			name: "select all test cases",
			selected: []Reference{
				{"pkg1", "TestFunc1", "Case1"},
				{"pkg1", "TestFunc1", "Case2"},
				{"pkg1", "TestFunc2", "Case1"},
				{"pkg1", "TestFunc2", "Case2"},
				{"pkg1", "TestFunc3", "Case1"},
				{"pkg1", "TestFunc3", "Case2"},
				{"pkg2", "TestFunc1", "Case1"},
				{"pkg2", "TestFunc1", "Case2"},
			},
			expectedResult: nil,
		},
		{
			name: "select mixture",
			selected: []Reference{
				{"pkg1", "TestFunc1", "Case1"},
				{"pkg1", "TestFunc1", "Case2"},
				{"pkg1", "TestFunc2", "Case1"},
				{"pkg1", "TestFunc3", "Case1"},
				{"pkg2", "TestFunc1", "Case1"},
				{"pkg2", "TestFunc1", "Case2"},
			},
			expectedResult: []Reference{
				{"pkg1", "TestFunc1", ""},
				{"pkg1", "TestFunc2", "Case1"},
				{"pkg1", "TestFunc3", "Case1"},
				{"pkg2", "", ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinimizeReferences(all, tt.selected)
			if d := cmp.Diff(tt.expectedResult, result, cmpopts.SortSlices(func(x, y Reference) bool {
				if x.Package != y.Package {
					return x.Package < y.Package
				}
				if x.FuncName != y.FuncName {
					return x.FuncName < y.FuncName
				}
				return x.TRunName < y.TRunName
			})); d != "" {
				t.Errorf("unexpected result (-want +got):\n%s", d)
			}
		})
	}
}
