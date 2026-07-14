package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSessionTestRuns(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	run1, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	run2, err := store.StartTestRun(sessionID, defaultRunnerConfig())
	require.NoError(t, err)

	// a second session's runs must not leak into the first session's results
	otherSession, err := store.StartTestSession()
	require.NoError(t, err)
	_, err = store.StartTestRun(otherSession, defaultRunnerConfig())
	require.NoError(t, err)

	runs, err := store.GetSessionTestRuns(sessionID, true)
	require.NoError(t, err)
	require.Len(t, runs, 2)

	got := map[string]bool{}
	for _, r := range runs {
		got[r.UUID] = true
	}
	require.True(t, got[run1.String()])
	require.True(t, got[run2.String()])
}
