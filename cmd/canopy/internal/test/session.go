package test

import (
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// session represents a test session containing one or more test runs.
// Sessions provide a logical grouping for related test executions.
type session struct {
	// store provides persistence for the session and its runs.
	store store
	// complete indicates whether the session has been marked as ended.
	complete bool
	// uuid uniquely identifies this session.
	uuid uuid.UUID
}

// SessionInfo contains metadata and run history for a test session.
type SessionInfo struct {
	// UUID uniquely identifies the session.
	UUID uuid.UUID `json:"uuid"`
	// Started is when the session was created.
	Started time.Time `json:"started"`
	// Ended is when the session was marked complete (nil if still active).
	Ended *time.Time `json:"ended"`
	// Runs contains information about all test runs in this session.
	Runs []RunInfo `json:"runs"`
}

// newSessionInfo converts a database test session record into a SessionInfo struct.
// Includes all associated test runs in the returned information.
func newSessionInfo(se db.TestSession, runs []RunInfo) SessionInfo {
	return SessionInfo{
		UUID:    uuid.MustParse(se.UUID),
		Started: se.Started,
		Ended:   se.Ended,
		Runs:    runs,
	}
}

// newRun creates a new test run within this session using the given configuration.
// Returns a run handle that can be used to add events and end the run.
func (s session) newRun(cfg gotest.RunnerConfig) (*run, error) {
	id, err := s.store.StartTestRun(s.uuid, cfg)
	if err != nil {
		return nil, err
	}

	return &run{
		session: &s,
		uuid:    id,
	}, nil
}

// info retrieves the current session information including all runs.
// Returns SessionInfo with metadata and run history.
func (s session) info() (*SessionInfo, error) {
	return s.store.GetSessionInfo(s.uuid)
}

// end marks the session as complete in persistent storage.
// Safe to call even if store is nil.
func (s *session) end() error {
	s.complete = true
	if s.store == nil {
		return nil
	}
	return s.store.EndTestSession(s.uuid)
}
