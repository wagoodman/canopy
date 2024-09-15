package test

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
)

type store struct {
	db *db.Store
}

type SessionInfo struct {
	UUID    uuid.UUID  `json:"uuid"`
	Started time.Time  `json:"started"`
	Ended   *time.Time `json:"ended"`
	Runs    []RunInfo  `json:"runs"`
}

func newStore(cfg Config) (*store, error) {
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

	return &store{
		db: d,
	}, nil
}

func dbPath(root string) string {
	if root == "" {
		panic("DB root is empty")
	}
	return filepath.Join(root, "db", db.Version, "canopy.db")
}

func newSessionInfo(se db.TestSession, runs []RunInfo) SessionInfo {
	return SessionInfo{
		UUID:    uuid.MustParse(se.UUID),
		Started: se.Started,
		Ended:   se.Ended,
		Runs:    runs,
	}
}

func (s store) getSessionInfo(id uuid.UUID) (*SessionInfo, error) {
	se, err := s.db.GetTestSession(id)
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

func (s store) getSession(uuid uuid.UUID) (*session, error) {
	ts, err := s.db.GetTestSession(uuid)
	if err != nil {
		return nil, err
	}
	return &session{
		store:    &s,
		complete: ts.Ended != nil,
		uuid:     uuid,
	}, nil
}

func (s store) newSession() (*session, error) {
	id, err := s.db.StartTestSession()
	if err != nil {
		return nil, err
	}
	return &session{
		store: &s,
		uuid:  id,
	}, nil
}

func (s store) ListSessions() ([]SessionInfo, error) {
	sessions, err := s.db.GetTestSessions()
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
