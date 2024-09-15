package test

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type session struct {
	store    *store
	complete bool
	uuid     uuid.UUID
}

func (s session) newRun(cfg gotest.RunnerConfig) (*run, error) {
	id, err := s.store.db.StartTestRun(s.uuid, cfg)
	if err != nil {
		return nil, err
	}

	return &run{
		session: &s,
		uuid:    id,
	}, nil
}

func (s session) ListRuns() ([]RunInfo, error) {
	runs, err := s.store.db.GetSessionTestRuns(s.uuid, true)
	if err != nil {
		return nil, err
	}

	var runInfos []RunInfo
	for _, run := range runs {
		var cfg gotest.RunnerConfig
		if err := json.Unmarshal(run.Config, &cfg); err != nil {
			return nil, fmt.Errorf("unable to unmarshal runner config: %w", err)
		}
		runInfos = append(runInfos, newRunInfo(run))
	}
	return runInfos, nil
}

func (s *session) end() error {
	s.complete = true
	if s.store == nil {
		return nil
	}
	return s.store.db.EndTestSession(s.uuid)
}
