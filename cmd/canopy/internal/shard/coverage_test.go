package shard

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeProfile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestMergeCoverProfiles(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{
			name: "disjoint union",
			a:    "mode: set\nx/a.go:1.1,2.1 1 1\n",
			b:    "mode: set\nx/a.go:3.1,4.1 1 0\nx/b.go:1.1,2.1 2 1\n",
			want: "mode: set\nx/a.go:1.1,2.1 1 1\nx/a.go:3.1,4.1 1 0\nx/b.go:1.1,2.1 2 1\n",
		},
		{
			name: "overlap set ors",
			a:    "mode: set\nx/a.go:1.1,2.1 1 1\nx/a.go:3.1,4.1 1 0\n",
			b:    "mode: set\nx/a.go:1.1,2.1 1 0\nx/a.go:3.1,4.1 1 1\n",
			want: "mode: set\nx/a.go:1.1,2.1 1 1\nx/a.go:3.1,4.1 1 1\n",
		},
		{
			name: "overlap count sums",
			a:    "mode: count\nx/a.go:1.1,2.1 1 2\n",
			b:    "mode: count\nx/a.go:1.1,2.1 1 3\n",
			want: "mode: count\nx/a.go:1.1,2.1 1 5\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			merged, err := MergeCoverProfiles([]string{writeProfile(t, dir, "a", tt.a), writeProfile(t, dir, "b", tt.b)})
			require.NoError(t, err)
			out := filepath.Join(dir, "out")
			require.NoError(t, WriteCoverProfile(out, merged))
			got, err := os.ReadFile(out)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestMergeCoverProfiles_mixedModes(t *testing.T) {
	dir := t.TempDir()
	a := writeProfile(t, dir, "a", "mode: set\nx/a.go:1.1,2.1 1 1\n")
	b := writeProfile(t, dir, "b", "mode: count\nx/a.go:1.1,2.1 1 1\n")
	_, err := MergeCoverProfiles([]string{a, b})
	require.Error(t, err)
}

// expected total is from `go tool cover -func=testdata/math.cover`
func TestCoveragePercent_matchesGoToolCover(t *testing.T) {
	merged, err := MergeCoverProfiles([]string{"testdata/math.cover"})
	require.NoError(t, err)
	got := CoveragePercent(merged)
	assert.Equal(t, 86.4, math.Round(got*10)/10)
}
