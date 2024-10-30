package test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

type run struct {
	session  *session
	complete bool
	uuid     uuid.UUID
}

type RunInfo struct {
	UUID     uuid.UUID
	Started  time.Time
	Ended    *time.Time
	Coverage *float64
	Config   gotest.RunnerConfig
}

func newRunInfo(run db.TestRun) RunInfo {
	var cfg gotest.RunnerConfig
	if err := json.Unmarshal(run.Config, &cfg); err != nil {
		log.Errorf("unable to unmarshal runner config: %v", err)
		// TODO return error
	}
	return RunInfo{
		UUID:    uuid.MustParse(run.UUID),
		Started: run.Started,
		Ended:   run.Ended,
		Config:  cfg,
	}
}

func (r run) addEvent(event gotest.Event) error {
	if r.complete {
		return fmt.Errorf("cannot add event to completed test run")
	}
	return r.session.store.AddTestEvent(r.uuid, event)
}

func (r *run) end(coverage *float64) error {
	r.complete = true
	if r.session == nil || r.session.store == nil {
		return nil
	}
	return r.session.store.EndTestRun(r.uuid, coverage)
}
