package gotest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_dfsTreeIterator_Next(t *testing.T) {
	tests := []struct {
		name string
		tree *Tree
		want []Reference
	}{
		{
			name: "empty tree",
			tree: NewTree(),
			want: nil,
		},
		{
			name: "single node",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
			},
		},
		{
			name: "single test",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
			},
		},
		{
			name: "single case",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
			},
		},
		{
			name: "multiple cases",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/not_unique"),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/not_unique"),
			},
		},
		{
			name: "duplicate case",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique#01"),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique#01"),
			},
		},
		{
			name: "many cases",
			tree: func() *Tree {
				tr := NewTree()
				tr.Add(
					NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/not_unique"),
					NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/oops"),
				)
				return tr
			}(),
			want: []Reference{
				NewReference("github.com/wagoodman/canopy/internal/example/strings", ""),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/unique"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/not_unique"),
				NewReference("github.com/wagoodman/canopy/internal/example/strings", "TestHasUniqueChars/oops"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.tree.IterateDF()
			var gotAll []Reference
			for n := d.Next(); n != nil; n = d.Next() {
				gotAll = append(gotAll, n.Reference)
			}
			if d := cmp.Diff(tt.want, gotAll); d != "" {
				t.Errorf("IterateDF() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
