package test

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// run represents a single test execution within a session, managing event collection and lifecycle.
type run struct {
	// session is the parent session containing this test run.
	session *session
	// complete indicates whether the test run has finished.
	complete bool
	// uuid uniquely identifies this test run.
	uuid uuid.UUID
}

// RunInfo contains metadata and configuration for a completed or in-progress test run.
type RunInfo struct {
	// UUID uniquely identifies the test run.
	UUID uuid.UUID
	// Started is when the test run began.
	Started time.Time
	// Ended is when the test run completed (nil if still running).
	Ended *time.Time
	// Coverage is the test coverage percentage (nil if not available).
	Coverage *float64
	// Config contains the runner configuration used for this test run.
	Config gotest.RunnerConfig
}

// newRunInfo converts a database test run record into a RunInfo struct.
// Returns RunInfo with deserialized configuration and metadata.
func newRunInfo(run db.TestRun) (RunInfo, error) {
	var cfg gotest.RunnerConfig
	if err := json.Unmarshal(run.Config, &cfg); err != nil {
		// don't silently return an empty config: a re-run would lose OnlyRefs/Coverage/UserArgs
		return RunInfo{}, fmt.Errorf("unable to unmarshal runner config for run %q: %w", run.UUID, err)
	}
	id, err := uuid.Parse(run.UUID)
	if err != nil {
		return RunInfo{}, fmt.Errorf("invalid run uuid %q: %w", run.UUID, err)
	}
	return RunInfo{
		UUID:     id,
		Started:  run.Started,
		Ended:    run.Ended,
		Coverage: run.Coverage,
		Config:   cfg,
	}, nil
}

// addEvent records a test event to the run's persistent storage.
// Returns an error if the run is already complete or if storage fails.
func (r run) addEvent(event gotest.Event) error {
	if r.complete {
		return fmt.Errorf("cannot add event to completed test run")
	}
	return r.session.store.AddTestEvent(r.uuid, event)
}

// end marks the test run as complete and records final coverage information.
// Coverage can be nil if not available. Safe to call even if store is nil.
func (r *run) end(coverage *db.CoverageInput) error {
	r.complete = true
	if r.session == nil || r.session.store == nil {
		return nil
	}
	return r.session.store.EndTestRun(r.uuid, coverage)
}
