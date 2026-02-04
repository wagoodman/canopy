package db

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"golang.org/x/tools/cover"
)

func defaultRunnerConfig() gotest.RunnerConfig {
	return gotest.RunnerConfig{}
}

func TestCoverageDataRoundTrip(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	// create a session and run
	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	profiles := []*cover.Profile{
		{
			FileName: "github.com/foo/bar/baz.go",
			Mode:     "set",
			Blocks: []cover.ProfileBlock{
				{StartLine: 10, StartCol: 2, EndLine: 20, EndCol: 5, NumStmt: 3, Count: 1},
				{StartLine: 25, StartCol: 1, EndLine: 30, EndCol: 10, NumStmt: 2, Count: 0},
			},
		},
		{
			FileName: "github.com/foo/bar/qux.go",
			Mode:     "set",
			Blocks: []cover.ProfileBlock{
				{StartLine: 5, StartCol: 1, EndLine: 15, EndCol: 3, NumStmt: 4, Count: 2},
			},
		},
	}

	pct := 75.5
	err = store.EndTestRun(runID, &CoverageInput{
		Percent:  pct,
		Profiles: profiles,
	})
	require.NoError(t, err)

	// verify the run has the coverage percentage set
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.NotNil(t, run.Coverage)
	require.InDelta(t, pct, *run.Coverage, 0.01)

	// retrieve structured coverage data
	covData, err := store.GetCoverageData(runID)
	require.NoError(t, err)
	require.NotNil(t, covData)

	require.Equal(t, "set", covData.Mode)
	require.Len(t, covData.Files, 2)

	// verify first file
	require.Equal(t, "github.com/foo/bar/baz.go", covData.Files[0].FileName)
	require.Len(t, covData.Files[0].Blocks, 2)

	if d := cmp.Diff(CoverageBlock{
		StartLine: 10, StartCol: 2, EndLine: 20, EndCol: 5, NumStmt: 3, Count: 1,
	}, covData.Files[0].Blocks[0], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".FileCoverageID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("first block mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(CoverageBlock{
		StartLine: 25, StartCol: 1, EndLine: 30, EndCol: 10, NumStmt: 2, Count: 0,
	}, covData.Files[0].Blocks[1], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".FileCoverageID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("second block mismatch (-want +got):\n%s", d)
	}

	// verify second file
	require.Equal(t, "github.com/foo/bar/qux.go", covData.Files[1].FileName)
	require.Len(t, covData.Files[1].Blocks, 1)

	if d := cmp.Diff(CoverageBlock{
		StartLine: 5, StartCol: 1, EndLine: 15, EndCol: 3, NumStmt: 4, Count: 2,
	}, covData.Files[1].Blocks[0], cmp.FilterPath(func(p cmp.Path) bool {
		return p.Last().String() == ".ID" || p.Last().String() == ".FileCoverageID"
	}, cmp.Ignore())); d != "" {
		t.Errorf("third block mismatch (-want +got):\n%s", d)
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
	covData, err := store.GetCoverageData(runID)
	require.NoError(t, err)
	require.Nil(t, covData)

	// verify run has no coverage percentage
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.Nil(t, run.Coverage)
}

func TestCoverageDataRoundTrip_EmptyProfiles(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	// end run with coverage percentage but no profiles
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
	covData, err := store.GetCoverageData(runID)
	require.NoError(t, err)
	require.Nil(t, covData)
}

func TestGetCoverageData_NonexistentRun(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	_, err = store.GetCoverageData(uuid.New())
	require.Error(t, err)
}
