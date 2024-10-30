package test

import (
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type session struct {
	store    store
	complete bool
	uuid     uuid.UUID
}

type SessionInfo struct {
	UUID    uuid.UUID  `json:"uuid"`
	Started time.Time  `json:"started"`
	Ended   *time.Time `json:"ended"`
	Runs    []RunInfo  `json:"runs"`
}

func newSessionInfo(se db.TestSession, runs []RunInfo) SessionInfo {
	return SessionInfo{
		UUID:    uuid.MustParse(se.UUID),
		Started: se.Started,
		Ended:   se.Ended,
		Runs:    runs,
	}
}

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

func (s session) info() (*SessionInfo, error) {
	return s.store.GetSessionInfo(s.uuid)
}

func (s *session) end() error {
	s.complete = true
	if s.store == nil {
		return nil
	}
	return s.store.EndTestSession(s.uuid)
}
