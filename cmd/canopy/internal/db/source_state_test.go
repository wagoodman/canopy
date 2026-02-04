package db

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSourceStateRoundTrip(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	modTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	input := &SourceStateInput{
		Commit: "abc123def456",
		Branch: "main",
		Dirty:  true,
		DirtyFiles: []DirtyFileInput{
			{
				Path:        "cmd/main.go",
				ContentHash: "deadbeef01234567",
				ModTime:     &modTime,
			},
			{
				Path:        "pkg/util.go",
				ContentHash: "",
				ModTime:     nil, // deleted file
			},
		},
	}

	err = store.AddSourceState(runID, input)
	require.NoError(t, err)

	got, err := store.GetSourceState(runID)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Equal(t, "abc123def456", got.Commit)
	require.Equal(t, "main", got.Branch)
	require.True(t, got.Dirty)
	require.Len(t, got.DirtyFiles, 2)

	// compare file states ignoring ID fields
	ignoreIDs := cmp.FilterPath(func(p cmp.Path) bool {
		last := p.Last().String()
		return last == ".ID" || last == ".SourceStateID"
	}, cmp.Ignore())

	if d := cmp.Diff(FileState{
		Path:        "cmd/main.go",
		ContentHash: "deadbeef01234567",
		ModTime:     &modTime,
	}, got.DirtyFiles[0], ignoreIDs); d != "" {
		t.Errorf("first file mismatch (-want +got):\n%s", d)
	}

	if d := cmp.Diff(FileState{
		Path:        "pkg/util.go",
		ContentHash: "",
		ModTime:     nil,
	}, got.DirtyFiles[1], ignoreIDs); d != "" {
		t.Errorf("second file mismatch (-want +got):\n%s", d)
	}
}

func TestSourceStateRoundTrip_NoSourceState(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	err = store.EndTestRun(runID, nil)
	require.NoError(t, err)

	got, err := store.GetSourceState(runID)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSourceStateRoundTrip_CleanRepo(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	input := &SourceStateInput{
		Commit: "abc123def456",
		Branch: "main",
		Dirty:  false,
	}

	err = store.AddSourceState(runID, input)
	require.NoError(t, err)

	got, err := store.GetSourceState(runID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "abc123def456", got.Commit)
	require.Equal(t, "main", got.Branch)
	require.False(t, got.Dirty)
	require.Empty(t, got.DirtyFiles)
}

func TestGetSourceState_NonexistentRun(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	_, err = store.GetSourceState(uuid.New())
	require.Error(t, err)
}
