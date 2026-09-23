package gotest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// the fixture has a tested package, an untested package called by it, and an untested package main.
// expected totals were recorded from `go tool cover -func` on the same fixture: untested main counts
// at 0%, and the untested package only gets credit under -coverpkg.
func TestRunner_CoverageMatchesGoToolCover(t *testing.T) {
	fixture, err := filepath.Abs("testdata/coverage-module")
	require.NoError(t, err)
	t.Chdir(fixture)

	const (
		tested   = "example.com/covfixture/tested"
		untested = "example.com/covfixture/untested"
		tool     = "example.com/covfixture/cmd/tool"
	)

	tests := []struct {
		name      string
		userArgs  []string
		wantTotal float64
		wantPkgs  map[string]float64
	}{
		{
			name:      "plain cover",
			wantTotal: 22.2,
			wantPkgs:  map[string]float64{tested: 66.67, untested: 0, tool: 0},
		},
		{
			name:      "coverpkg",
			userArgs:  []string{"-coverpkg=./..."},
			wantTotal: 44.4,
			wantPkgs:  map[string]float64{tested: 66.67, untested: 50, tool: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// run twice with the cache on: the second run is served from the go test cache and must
			// still produce the same coverage.
			for i := range 2 {
				dir := t.TempDir()
				run, err := NewRunner(RunnerConfig{
					Coverage:    true,
					CoverageDir: dir,
					UserArgs:    append([]string{"./..."}, tt.userArgs...),
				}).Run(context.Background(), ResultConfig{})
				require.NoError(t, err, "run %d", i)

				total, ok := run.Result.Coverage()
				require.True(t, ok, "run %d: coverage should be recorded", i)
				require.InDelta(t, tt.wantTotal, total, 0.01, "run %d", i)
				require.FileExists(t, filepath.Join(dir, coverProfileName))

				got := map[string]float64{}
				for _, p := range run.PackageCoverage {
					got[p.PackagePath] = p.Percent
				}
				require.Len(t, got, len(tt.wantPkgs), "run %d: %v", i, got)
				for pkg, want := range tt.wantPkgs {
					require.InDelta(t, want, got[pkg], 0.01, "run %d: %s", i, pkg)
				}
			}
		})
	}
}

func TestRunner_NoProfileLeavesCoverageUnset(t *testing.T) {
	r := NewRunner(RunnerConfig{Coverage: true, CoverageDir: t.TempDir()})
	run := &Run{Result: *NewResult(ResultConfig{})}
	require.NoError(t, r.recordCoverage(run))
	_, ok := run.Result.Coverage()
	require.False(t, ok)
}
