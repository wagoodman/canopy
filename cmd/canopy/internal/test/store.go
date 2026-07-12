package test

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/go-homedir"
)

var _ store = (*dbStore)(nil)

// store defines the interface for persisting test sessions, runs, and events.
// Implementations provide both session management and data persistence.
type store interface {
	// sessionManager
	sessionStore
	runStore
}

// sessionManager provides methods for creating and retrieving session instances.
type sessionManager interface {
	// newSession creates a new test session.
	newSession() (*session, error)
	// getSession retrieves an existing session by UUID.
	getSession(uuid uuid.UUID) (*session, error)
}

// sessionStore defines persistence operations for test session metadata.
type sessionStore interface {
	// GetSessionInfo retrieves session information including all runs.
	GetSessionInfo(id uuid.UUID) (*SessionInfo, error)
	// ListSessions returns all stored test sessions.
	ListSessions() ([]SessionInfo, error)
	// EndTestSession marks a session as complete.
	EndTestSession(sessionID uuid.UUID) error
}

// runStore defines persistence operations for test runs and events.
type runStore interface {
	// StartTestRun creates a new test run within a session.
	StartTestRun(sessionID uuid.UUID, cfg gotest.RunnerConfig) (uuid.UUID, error)
	// GetRunInfo retrieves metadata for a specific test run.
	GetRunInfo(runID uuid.UUID) (RunInfo, error)
	// GetTestEvents retrieves all events for a test run.
	GetTestEvents(runID uuid.UUID) ([]gotest.Event, error)
	// GetTestEventsBatch retrieves a batch of events with pagination.
	GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]gotest.Event, bool, error)
	// GetTestEventCount returns the total number of events for a run.
	GetTestEventCount(runID uuid.UUID) (int64, error)
	// AddTestEvent records a test event to a run.
	AddTestEvent(runID uuid.UUID, event gotest.Event) error
	// EndTestRun marks a run as complete with optional coverage data.
	EndTestRun(runID uuid.UUID, coverage *db.CoverageInput) error
	// GetFailuresByRun retrieves all failure data for a specific test run.
	GetFailuresByRun(runID uuid.UUID) ([]db.FailedTestDetails, error)
	// AddSourceState stores source state data for a test run.
	AddSourceState(runID uuid.UUID, state *db.SourceStateInput) error
}

// dbStore implements the store interface using SQLite via GORM.
type dbStore struct {
	*db.Store
}

// newDBStore creates a new database-backed store using the provided configuration.
// Returns nil store if DBRoot is empty. The database path is expanded from home directory if needed.
func newDBStore(cfg Config) (*dbStore, error) {
	path := dbPath(cfg.DBRoot)
	if path == "" {
		return nil, nil
	}

	path, err := homedir.Expand(path)
	if err != nil {
		return nil, fmt.Errorf("unable to expand db path: %v", err)
	}
	d, err := db.New(path)
	if err != nil {
		return nil, fmt.Errorf("unable to create database: %v", err)
	}

	return &dbStore{
		Store: d,
	}, nil
}

// dbPath constructs the full database file path from the given root directory.
// Panics if root is empty as this indicates a programming error.
func dbPath(root string) string {
	if root == "" {
		panic("DB root is empty")
	}
	return filepath.Join(root, "db", db.Version, "canopy.db")
}

// GetSessionInfo retrieves session metadata and all associated runs.
func (s dbStore) GetSessionInfo(id uuid.UUID) (*SessionInfo, error) {
	se, err := s.GetTestSession(id)
	if err != nil {
		return nil, err
	}

	var runs []RunInfo

	if se.TestRuns != nil {
		for _, r := range *se.TestRuns {
			ri, err := newRunInfo(r)
			if err != nil {
				log.WithFields("run", r.UUID, "error", err).Warn("skipping unreadable test run")
				continue
			}
			runs = append(runs, ri)
		}
	}

	si := newSessionInfo(se, runs)

	return &si, nil
}

// getSession retrieves an existing session by UUID, reconstructing the session state.
func (s dbStore) getSession(uuid uuid.UUID) (*session, error) {
	ts, err := s.GetTestSession(uuid)
	if err != nil {
		return nil, err
	}
	return &session{
		store:    &s,
		complete: ts.Ended != nil,
		uuid:     uuid,
	}, nil
}

// newSession creates a new test session in the database.
func (s dbStore) newSession() (*session, error) {
	id, err := s.StartTestSession()
	if err != nil {
		return nil, err
	}
	return &session{
		store: &s,
		uuid:  id,
	}, nil
}

// ListSessions retrieves all test sessions with their run information.
func (s dbStore) ListSessions() ([]SessionInfo, error) {
	sessions, err := s.GetTestSessions()
	if err != nil {
		return nil, err
	}

	var sessionInfos []SessionInfo
	for _, se := range sessions {
		var runs []RunInfo

		if se.TestRuns != nil {
			for _, r := range *se.TestRuns {
				ri, err := newRunInfo(r)
				if err != nil {
					log.WithFields("run", r.UUID, "error", err).Warn("skipping unreadable test run")
					continue
				}
				runs = append(runs, ri)
			}
		}

		sessionInfos = append(sessionInfos, SessionInfo{
			UUID:    uuid.MustParse(se.UUID),
			Started: se.Started,
			Ended:   se.Ended,
			Runs:    runs,
		})
	}
	return sessionInfos, nil
}

// GetRunInfo retrieves metadata for a specific test run.
func (s dbStore) GetRunInfo(runID uuid.UUID) (RunInfo, error) {
	tr, err := s.GetTestRun(runID)
	if err != nil {
		return RunInfo{}, err
	}
	return newRunInfo(tr)
}

// GetFailuresByRun retrieves all failure data for a specific test run.
func (s dbStore) GetFailuresByRun(runID uuid.UUID) ([]db.FailedTestDetails, error) {
	return s.Store.GetFailuresByRun(runID)
}

// AddSourceState stores source state data for a test run.
func (s dbStore) AddSourceState(runID uuid.UUID, state *db.SourceStateInput) error {
	return s.Store.AddSourceState(runID, state)
}

// GetTestEvents retrieves all events for a test run, converting from database format.
// Reconstructs event objects with their references, annotations, and errors.
func (s dbStore) GetTestEvents(runID uuid.UUID) ([]gotest.Event, error) {
	eventInfos, err := s.Store.GetTestEvents(runID)
	if err != nil {
		return nil, err
	}

	return s.convertDBEvents(runID, eventInfos), nil
}

// GetTestEventsBatch retrieves a batch of events with pagination support.
func (s dbStore) GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]gotest.Event, bool, error) {
	eventInfos, hasMore, err := s.Store.GetTestEventsBatch(runID, offset, limit)
	if err != nil {
		return nil, false, err
	}

	return s.convertDBEvents(runID, eventInfos), hasMore, nil
}

// GetTestEventCount returns the total number of events for a run.
func (s dbStore) GetTestEventCount(runID uuid.UUID) (int64, error) {
	return s.Store.GetTestEventCount(runID)
}

// convertDBEvents converts database event models to gotest.Event objects.
func (s dbStore) convertDBEvents(runID uuid.UUID, eventInfos []db.TestEvent) []gotest.Event {
	var events []gotest.Event
	for _, e := range eventInfos {
		var annotations []gotest.Annotation
		for _, a := range e.Annotations {
			annotations = append(annotations, gotest.Annotation(a.Value))
		}

		var eventErr error
		if e.Error != "" {
			// TODO: use gob to preserve the error type
			eventErr = errors.New(e.Error)
		}

		events = append(events, gotest.Event{
			RunID: runID,
			JSONL: "",
			Index: e.Index,
			Time:  e.Time,
			Reference: gotest.Reference{
				Package:  e.Reference.Package,
				FuncName: e.Reference.FuncName,
				TRunName: e.Reference.TRunName,
			},
			Action:      gotest.Action(e.Action),
			Output:      e.Output,
			Elapsed:     e.Elapsed,
			FailedBuild: e.FailedBuild,
			Annotations: annotations,
			Error:       eventErr,
		})
	}

	return events
}
