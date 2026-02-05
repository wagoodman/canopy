package db

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cover"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func defaultRunnerConfig() gotest.RunnerConfig {
	return gotest.RunnerConfig{}
}

func TestCoverageDataRoundTrip(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	pkgs := []cover.PackageResult{
		{PackagePath: "github.com/foo/bar/pkg1", Percent: 41.1},
		{PackagePath: "github.com/foo/bar/pkg2", Percent: 87.5},
	}

	funcs := []cover.FunctionResult{
		{FilePath: "pkg1/file.go", Line: 12, FuncName: "hello", Percent: 100.0},
		{FilePath: "pkg1/file.go", Line: 18, FuncName: "unused", Percent: 0.0},
		{FilePath: "pkg2/other.go", Line: 5, FuncName: "doWork", Percent: 87.5},
	}

	pct := 41.1
	err = store.EndTestRun(runID, &CoverageInput{
		Percent:     pct,
		CoverageDir: "/tmp/coverage/test-run-1",
		Packages:    pkgs,
		Functions:   funcs,
	})
	require.NoError(t, err)

	// verify the run has the coverage percentage and dir set
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.NotNil(t, run.Coverage)
	require.InDelta(t, pct, *run.Coverage, 0.01)
	require.Equal(t, "/tmp/coverage/test-run-1", run.CoverageDir)

	// verify package coverage round-trip
	gotPkgs, err := store.GetPackageCoverage(runID)
	require.NoError(t, err)
	require.Len(t, gotPkgs, 2)

	if d := cmp.Diff(PackageCoverage{
		PackagePath: "github.com/foo/bar/pkg1",
		Percent:     41.1,
	}, gotPkgs[0], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".RunID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("first package mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(PackageCoverage{
		PackagePath: "github.com/foo/bar/pkg2",
		Percent:     87.5,
	}, gotPkgs[1], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".RunID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("second package mismatch (-want +got):\n%s", d)
	}

	// verify function coverage round-trip
	gotFuncs, err := store.GetFunctionCoverage(runID)
	require.NoError(t, err)
	require.Len(t, gotFuncs, 3)

	if d := cmp.Diff(FunctionCoverage{
		FilePath: "pkg1/file.go",
		Line:     12,
		FuncName: "hello",
		Percent:  100.0,
	}, gotFuncs[0], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".RunID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("first function mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(FunctionCoverage{
		FilePath: "pkg1/file.go",
		Line:     18,
		FuncName: "unused",
		Percent:  0.0,
	}, gotFuncs[1], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".RunID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("second function mismatch (-want +got):\n%s", d)
	}
}

func TestCoverageDataRoundTrip_NoCoverage(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	// end run without coverage
	err = store.EndTestRun(runID, nil)
	require.NoError(t, err)

	// verify no coverage data
	pkgs, err := store.GetPackageCoverage(runID)
	require.NoError(t, err)
	require.Empty(t, pkgs)

	funcs, err := store.GetFunctionCoverage(runID)
	require.NoError(t, err)
	require.Empty(t, funcs)

	// verify run has no coverage percentage
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.Nil(t, run.Coverage)
	require.Empty(t, run.CoverageDir)
}

func TestCoverageDataRoundTrip_EmptyCoverageResults(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	// end run with coverage percentage but no package/function results
	pct := 50.0
	err = store.EndTestRun(runID, &CoverageInput{
		Percent: pct,
	})
	require.NoError(t, err)

	// verify the percentage is still stored
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.NotNil(t, run.Coverage)
	require.InDelta(t, pct, *run.Coverage, 0.01)

	// but no structured coverage data
	pkgs, err := store.GetPackageCoverage(runID)
	require.NoError(t, err)
	require.Empty(t, pkgs)
}

func TestGetCoverageData_NonexistentRun(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	_, err = store.GetPackageCoverage(uuid.New())
	require.Error(t, err)

	_, err = store.GetFunctionCoverage(uuid.New())
	require.Error(t, err)
}
