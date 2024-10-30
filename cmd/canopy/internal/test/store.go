package test

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ store = (*dbStore)(nil)

type store interface {
	// sessionManager
	sessionStore
	runStore
}

type sessionManager interface {
	newSession() (*session, error)
	getSession(uuid uuid.UUID) (*session, error)
}

type sessionStore interface {
	GetSessionInfo(id uuid.UUID) (*SessionInfo, error)
	ListSessions() ([]SessionInfo, error)
	EndTestSession(sessionID uuid.UUID) error
}

type runStore interface {
	StartTestRun(sessionID uuid.UUID, cfg gotest.RunnerConfig) (uuid.UUID, error)
	GetRunInfo(runID uuid.UUID) (RunInfo, error)
	GetTestEvents(runID uuid.UUID) ([]gotest.Event, error)
	AddTestEvent(runID uuid.UUID, event gotest.Event) error
	EndTestRun(runID uuid.UUID, coverage *float64) error
}

type dbStore struct {
	*db.Store
}

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

func dbPath(root string) string {
	if root == "" {
		panic("DB root is empty")
	}
	return filepath.Join(root, "db", db.Version, "canopy.db")
}

func (s dbStore) GetSessionInfo(id uuid.UUID) (*SessionInfo, error) {
	se, err := s.Store.GetTestSession(id)
	if err != nil {
		return nil, err
	}

	var runs []RunInfo

	if se.TestRuns != nil {
		for _, r := range *se.TestRuns {
			runs = append(runs, newRunInfo(r))
		}
	}

	si := newSessionInfo(se, runs)

	return &si, nil
}

func (s dbStore) getSession(uuid uuid.UUID) (*session, error) {
	ts, err := s.Store.GetTestSession(uuid)
	if err != nil {
		return nil, err
	}
	return &session{
		store:    &s,
		complete: ts.Ended != nil,
		uuid:     uuid,
	}, nil
}

func (s dbStore) newSession() (*session, error) {
	id, err := s.Store.StartTestSession()
	if err != nil {
		return nil, err
	}
	return &session{
		store: &s,
		uuid:  id,
	}, nil
}

func (s dbStore) ListSessions() ([]SessionInfo, error) {
	sessions, err := s.Store.GetTestSessions()
	if err != nil {
		return nil, err
	}

	var sessionInfos []SessionInfo
	for _, se := range sessions {
		var runs []RunInfo

		if se.TestRuns != nil {
			for _, r := range *se.TestRuns {
				runs = append(runs, newRunInfo(r))
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

func (s dbStore) GetRunInfo(runID uuid.UUID) (RunInfo, error) {
	tr, err := s.Store.GetTestRun(runID)
	if err != nil {
		return RunInfo{}, err
	}
	return newRunInfo(tr), nil
}

func (s dbStore) GetTestEvents(runID uuid.UUID) ([]gotest.Event, error) {
	eventInfos, err := s.Store.GetTestEvents(runID)
	if err != nil {
		return nil, err
	}

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
			JSONL: "", // TODO: is this a problem?
			Index: e.Index,
			Time:  e.Time,
			Reference: gotest.Reference{
				Package:  e.Reference.Package,
				FuncName: e.Reference.FuncName,
				TRunName: e.Reference.TRunName,
			},
			Action:      gotest.Action(e.Action),
			Output:      e.Output,
			Annotations: annotations,
			Error:       eventErr,
		})
	}

	return events, nil
}
