package commands

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

func TestResolveRunID(t *testing.T) {
	tests := []struct {
		name         string
		runIDArg     string
		setupSessions int  // number of sessions to create
		runsPerSession int  // number of runs per session
		expectMostRecent bool // whether to expect the most recent run
		wantErr      require.ErrorAssertionFunc
	}{
		{
			name:         "explicit valid run ID",
			runIDArg:     "", // will be set during test
			setupSessions: 1,
			runsPerSession: 1,
			wantErr:      require.NoError,
		},
		{
			name:     "explicit invalid run ID format",
			runIDArg: "not-a-uuid",
			wantErr:  require.Error,
		},
		{
			name:         "no sessions in database",
			runIDArg:     "",
			setupSessions: 0,
			wantErr:      require.Error,
		},
		{
			name:         "single session with one run",
			runIDArg:     "",
			setupSessions: 1,
			runsPerSession: 1,
			expectMostRecent: true,
		},
		{
			name:         "multiple runs - returns most recent",
			runIDArg:     "",
			setupSessions: 1,
			runsPerSession: 3,
			expectMostRecent: true,
		},
		{
			name:         "multiple sessions - returns most recent run",
			runIDArg:     "",
			setupSessions: 3,
			runsPerSession: 2,
			expectMostRecent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			// create in-memory manager for testing
			m, err := test.NewManager(test.Config{
				DBRoot:    t.TempDir(),
				Ephemeral: true,
			})
			require.NoError(t, err)
			defer func() {
				require.NoError(t, m.Close())
			}()

			// populate the database with test sessions and runs
			store := m.DBStore()
			require.NotNil(t, store)

			var allRunIDs []uuid.UUID
			var mostRecentRunID uuid.UUID
			var mostRecentTime time.Time

			for i := 0; i < tt.setupSessions; i++ {
				sessionID, err := store.StartTestSession()
				require.NoError(t, err)

				for j := 0; j < tt.runsPerSession; j++ {
					// add a small delay to ensure distinct timestamps
					time.Sleep(1 * time.Millisecond)

					runID, err := store.StartTestRun(sessionID, gotest.RunnerConfig{})
					require.NoError(t, err)
					allRunIDs = append(allRunIDs, runID)

					// track the most recent run
					runInfo, err := store.GetTestRun(runID)
					require.NoError(t, err)

					if mostRecentRunID == uuid.Nil || runInfo.Started.After(mostRecentTime) {
						mostRecentRunID = runID
						mostRecentTime = runInfo.Started
					}
				}
			}

			// set explicit run ID for first test case
			if tt.name == "explicit valid run ID" && len(allRunIDs) > 0 {
				tt.runIDArg = allRunIDs[0].String()
			}

			got, err := resolveRunID(m, tt.runIDArg)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			// verify the result
			if tt.expectMostRecent {
				require.Equal(t, mostRecentRunID, got, "should return the most recent run")
			} else if tt.runIDArg != "" {
				expected, _ := uuid.Parse(tt.runIDArg)
				require.Equal(t, expected, got, "should return the explicitly requested run")
			}
		})
	}
}

func TestResolveRunID_ErrorMessages(t *testing.T) {
	tests := []struct {
		name           string
		runIDArg       string
		setupSessions  int
		wantErrContain string
	}{
		{
			name:           "invalid UUID format",
			runIDArg:       "not-a-uuid",
			wantErrContain: "invalid run ID",
		},
		{
			name:           "no sessions found",
			runIDArg:       "",
			setupSessions:  0,
			wantErrContain: "no test sessions found in database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := test.NewManager(test.Config{
				DBRoot:    t.TempDir(),
				Ephemeral: true,
			})
			require.NoError(t, err)
			defer func() {
				require.NoError(t, m.Close())
			}()

			// populate the database with test sessions if needed
			store := m.DBStore()
			require.NotNil(t, store)

			for i := 0; i < tt.setupSessions; i++ {
				_, err := store.StartTestSession()
				require.NoError(t, err)
			}

			_, err = resolveRunID(m, tt.runIDArg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErrContain)
		})
	}
}
