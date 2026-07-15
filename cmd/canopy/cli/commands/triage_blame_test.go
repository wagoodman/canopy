package commands

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/blame"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// TestBuildBlameHistory_JoinsCommits seeds a real store with runs across distinct commits and
// confirms buildBlameHistory joins each run's outcome to its recorded commit, feeding the pure
// engine a history that resolves the pre-existing onset end to end.
func TestBuildBlameHistory_JoinsCommits(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "canopy.db"))
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	seeds := []struct {
		commit string
		pass   bool
		fp     string
	}{
		{"c1", true, ""},      // good
		{"c2", true, ""},      // good
		{"c3", false, "boom"}, // onset of the failure fingerprint
	}

	var outcomes []flaky.Outcome
	for i, s := range seeds {
		runID, err := store.StartTestRun(sessionID, gotest.RunnerConfig{})
		require.NoError(t, err)
		require.NoError(t, store.AddSourceState(runID, &db.SourceStateInput{Commit: s.commit, Branch: "main"}))

		o := flaky.Outcome{RunID: runID, Time: time.Unix(int64(i), 0)}
		if s.pass {
			o.Action = gotest.PassAction
		} else {
			o.Action = gotest.FailAction
			o.Failure = &flaky.FailureInfo{Fingerprint: s.fp}
		}
		outcomes = append(outcomes, o)
	}

	history := buildBlameHistory(store, outcomes, map[uuid.UUID]*db.SourceState{})
	require.Len(t, history, 3)
	require.Equal(t, "c1", history[0].Commit)
	require.True(t, history[0].Passed)
	require.Equal(t, "boom", history[2].Fingerprint)

	since := blame.DetectPreExisting(history, "boom", nil)
	require.NotNil(t, since)
	require.Equal(t, "c3", since.Commit)
	require.Equal(t, "c2", since.LastGoodCommit)
	// a nil distance func cannot prove adjacency, so the honest confidence is a range.
	require.Equal(t, blame.ConfidenceRange, since.Confidence)
}

// confirms a skip outcome is dropped from blame history so a t.Skip() run between passes and
// the real failure cannot masquerade as the onset.
func TestBuildBlameHistory_DropsSkips(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "canopy.db"))
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	seeds := []struct {
		commit string
		action gotest.Action
		fp     string
	}{
		{"c1", gotest.PassAction, ""},
		{"c2", gotest.SkipAction, ""}, // must not be treated as a failure
		{"c3", gotest.PassAction, ""},
		{"c4", gotest.FailAction, "boom"}, // the real onset
	}

	var outcomes []flaky.Outcome
	for i, s := range seeds {
		runID, err := store.StartTestRun(sessionID, gotest.RunnerConfig{})
		require.NoError(t, err)
		require.NoError(t, store.AddSourceState(runID, &db.SourceStateInput{Commit: s.commit, Branch: "main"}))

		o := flaky.Outcome{RunID: runID, Time: time.Unix(int64(i), 0), Action: s.action}
		if s.action == gotest.FailAction {
			o.Failure = &flaky.FailureInfo{Fingerprint: s.fp}
		}
		outcomes = append(outcomes, o)
	}

	history := buildBlameHistory(store, outcomes, map[uuid.UUID]*db.SourceState{})
	// the skip at c2 is excluded: only the two passes and the failure survive
	require.Len(t, history, 3)
	for _, rp := range history {
		require.NotEqual(t, "c2", rp.Commit)
	}

	since := blame.DetectPreExisting(history, "boom", nil)
	require.NotNil(t, since)
	require.Equal(t, "c4", since.Commit)
	require.Equal(t, "c3", since.LastGoodCommit)
}
