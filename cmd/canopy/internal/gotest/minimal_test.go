package gotest

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGroupIntoRuns(t *testing.T) {
	tests := []struct {
		name string
		refs References
		want [][]string // per-group set of ref strings
	}{
		{
			name: "empty",
			refs: nil,
			want: nil,
		},
		{
			name: "package refs group together",
			refs: References{
				{Package: "a"},
				{Package: "b"},
			},
			want: [][]string{{"a", "b"}},
		},
		{
			name: "function and subtest refs split per package",
			refs: References{
				{Package: "a"},
				{Package: "b", FuncName: "TestFoo"},
				{Package: "b", FuncName: "TestBar", TRunName: "case_two"},
			},
			want: [][]string{
				{"a"},
				{"b/TestFoo", "b/TestBar/case_two"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GroupIntoRuns(tt.refs)
			if len(got) != len(tt.want) {
				t.Fatalf("group count: got %d want %d", len(got), len(tt.want))
			}
			for i, group := range got {
				var names []string
				for _, ref := range group {
					names = append(names, ref.String(true))
				}
				if diff := cmp.Diff(tt.want[i], names); diff != "" {
					t.Errorf("group %d mismatch (-want +got):\n%s", i, diff)
				}
			}
		})
	}
}

// TestSingleCaseRunPipeline is the regression that motivated the fix: a single selected
// table case must survive minimization/grouping and become an anchored -run for just that case.
func TestSingleCaseRunPipeline(t *testing.T) {
	defs := Definitions{
		{Module: "m", ImportPath: "m/pkg", FnName: "TestTable", Cases: []string{"case_one", "case_two", "case_three"}},
		{Module: "m", ImportPath: "m/pkg", FnName: "TestOther"},
	}

	tests := []struct {
		name     string
		selected References
		wantRun  string // the -run= flag we expect to reach `go test`
	}{
		{
			name:     "single case runs only that case",
			selected: References{{Package: "m/pkg", FuncName: "TestTable", TRunName: "case_two"}},
			wantRun:  "-run=^TestTable/case_two$",
		},
		{
			name:     "whole function runs the whole table",
			selected: References{{Package: "m/pkg", FuncName: "TestTable"}},
			wantRun:  "-run=^TestTable$",
		},
		{
			name: "all cases collapse to the function",
			selected: References{
				{Package: "m/pkg", FuncName: "TestTable", TRunName: "case_one"},
				{Package: "m/pkg", FuncName: "TestTable", TRunName: "case_two"},
				{Package: "m/pkg", FuncName: "TestTable", TRunName: "case_three"},
			},
			wantRun: "-run=^TestTable$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minimized := MinimizeReferences(defs.References(), tt.selected)
			var got []string
			for _, group := range GroupIntoRuns(minimized) {
				got = append(got, runFilters(group)...)
			}
			joined := strings.Join(got, " ")
			if !strings.Contains(joined, tt.wantRun) {
				t.Errorf("expected %q in run flags, got %q", tt.wantRun, joined)
			}
		})
	}
}
