package commands

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestAffectedPackages(t *testing.T) {
	// synthetic import graph (Deps is transitive, as `go list` emits it):
	//   a           (leaf, the thing that changes)
	//   b -> a      (direct dependent)
	//   c -> b -> a (transitive dependent; Deps already flattened to [a, b])
	//   d           (test imports a only)
	//   e           (test imports c; c transitively depends on a)
	//   u -> x      (unrelated)
	pkgs := []golist.Package{
		{ImportPath: "a"},
		{ImportPath: "b", Deps: []string{"a"}},
		{ImportPath: "c", Deps: []string{"a", "b"}},
		{ImportPath: "d", TestImports: []string{"a"}},
		{ImportPath: "e", XTestImports: []string{"c"}},
		{ImportPath: "u", Deps: []string{"x"}},
		{ImportPath: "x"},
	}

	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			name:    "changed package itself is affected",
			changed: []string{"a"},
			// a (self), b (direct dep), c (transitive dep), d (test import), e (test import -> transitive)
			want: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:    "direct dependent only",
			changed: []string{"b"},
			// b (self), c (direct dep), e (test imports c which depends on b)
			want: []string{"b", "c", "e"},
		},
		{
			name:    "unrelated leaf affects only its dependents",
			changed: []string{"x"},
			want:    []string{"u", "x"},
		},
		{
			name:    "no changes yields empty set",
			changed: nil,
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := affectedPackages(pkgs, strset.New(tt.changed...))
			gotList := got.List()
			sort.Strings(gotList)

			if diff := cmp.Diff(tt.want, gotList); diff != "" {
				t.Errorf("affectedPackages() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
