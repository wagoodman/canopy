package commands

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

func TestCollectRunEntries(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	later := now.Add(5 * time.Second)

	sessionID := uuid.MustParse("aaaaaaaa-1111-2222-3333-444444444444")
	runID1 := uuid.MustParse("bbbbbbbb-1111-2222-3333-444444444444")
	runID2 := uuid.MustParse("cccccccc-1111-2222-3333-444444444444")

	tests := []struct {
		name     string
		sessions []test.SessionInfo
		want     []runListEntry
	}{
		{
			name:     "no sessions",
			sessions: nil,
			want:     nil,
		},
		{
			name: "session with no runs",
			sessions: []test.SessionInfo{
				{
					UUID:    sessionID,
					Started: now,
				},
			},
			want: nil,
		},
		{
			name: "single session with one completed run",
			sessions: []test.SessionInfo{
				{
					UUID:    sessionID,
					Started: now,
					Runs: []test.RunInfo{
						{
							UUID:    runID1,
							Started: now,
							Ended:   &later,
						},
					},
				},
			},
			want: []runListEntry{
				{
					RunID:     runID1.String(),
					SessionID: sessionID.String(),
					Started:   now,
					Ended:     &later,
					Elapsed:   "5s",
				},
			},
		},
		{
			name: "single session with an in-progress run",
			sessions: []test.SessionInfo{
				{
					UUID:    sessionID,
					Started: now,
					Runs: []test.RunInfo{
						{
							UUID:    runID1,
							Started: now,
						},
					},
				},
			},
			want: []runListEntry{
				{
					RunID:     runID1.String(),
					SessionID: sessionID.String(),
					Started:   now,
				},
			},
		},
		{
			name: "multiple sessions with multiple runs",
			sessions: []test.SessionInfo{
				{
					UUID:    sessionID,
					Started: now,
					Runs: []test.RunInfo{
						{
							UUID:    runID1,
							Started: now,
							Ended:   &later,
						},
					},
				},
				{
					UUID:    uuid.MustParse("dddddddd-1111-2222-3333-444444444444"),
					Started: later,
					Runs: []test.RunInfo{
						{
							UUID:    runID2,
							Started: later,
						},
					},
				},
			},
			want: []runListEntry{
				{
					RunID:     runID1.String(),
					SessionID: sessionID.String(),
					Started:   now,
					Ended:     &later,
					Elapsed:   "5s",
				},
				{
					RunID:     runID2.String(),
					SessionID: "dddddddd-1111-2222-3333-444444444444",
					Started:   later,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectRunEntries(tt.sessions)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("unexpected entries (-want +got):\n%s", d)
			}
		})
	}
}

func TestWriteRunsJSON(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	later := now.Add(5 * time.Second)

	entries := []runListEntry{
		{
			RunID:     "bbbbbbbb-1111-2222-3333-444444444444",
			SessionID: "aaaaaaaa-1111-2222-3333-444444444444",
			Started:   now,
			Ended:     &later,
			Elapsed:   "5s",
		},
	}

	var buf bytes.Buffer
	err := writeRunsJSON(&buf, entries)
	require.NoError(t, err)

	// verify the output is valid JSON
	var got []runListEntry
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	if d := cmp.Diff(entries, got); d != "" {
		t.Errorf("unexpected JSON output (-want +got):\n%s", d)
	}
}

func TestWriteRunsTable(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	later := now.Add(5 * time.Second)

	entries := []runListEntry{
		{
			RunID:     "bbbbbbbb-1111-2222-3333-444444444444",
			SessionID: "aaaaaaaa-1111-2222-3333-444444444444",
			Started:   now,
			Ended:     &later,
			Elapsed:   "5s",
		},
		{
			RunID:     "cccccccc-1111-2222-3333-444444444444",
			SessionID: "dddddddd-1111-2222-3333-444444444444",
			Started:   later,
		},
	}

	var stdout, stderr bytes.Buffer
	err := writeRunsTable(&stdout, &stderr, entries)
	require.NoError(t, err)

	// stdout should contain only run IDs, one per line
	stdoutLines := stdout.String()
	require.Contains(t, stdoutLines, "bbbbbbbb-1111-2222-3333-444444444444\n")
	require.Contains(t, stdoutLines, "cccccccc-1111-2222-3333-444444444444\n")

	// stderr should contain the table with metadata (go-pretty uppercases headers)
	stderrContent := stderr.String()
	require.Contains(t, stderrContent, "RUN ID")
	require.Contains(t, stderrContent, "SESSION")
	require.Contains(t, stderrContent, "STARTED")
	require.Contains(t, stderrContent, "ELAPSED")
	require.Contains(t, stderrContent, "bbbbbbbb") // run ID in table
	require.Contains(t, stderrContent, "aaaaaaaa") // abbreviated session ID
	require.Contains(t, stderrContent, "5s")       // elapsed time
}
