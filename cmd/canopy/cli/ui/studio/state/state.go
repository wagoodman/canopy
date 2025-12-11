// Package state provides interfaces and adapters for managing test run state,
// including current runs, historical runs, and test execution control.
package state

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// State combines the ability to view the current run, start new runs, and
// access historical run data.
type State interface {
	// RunViewer     // can interpret a single result
	// CurrentRun returns a viewer for the currently active test run.
	CurrentRun() RunViewer

	// RunController can kick off new runs
	RunController

	// RunStore can access historical runs
	RunStore

	// Updater can update the state of a current run that is not stored
	Updater
}

// RunStore provides access to historical test run data from persistent storage.
type RunStore interface {
	// GetRun retrieves a test run by its UUID.
	GetRun(uuid.UUID) (*gotest.Run, error)
}

// RunViewer provides read-only access to a test run's configuration, results,
// and events.
type RunViewer interface {
	// Config returns the runner configuration for this test run.
	Config() gotest.RunnerConfig

	// References returns all test references in the run, optionally filtered by
	// the provided predicates.
	References(...func(gotest.Reference) bool) []gotest.Reference

	// ReferenceConclusiveAction returns the final action (pass/fail/skip) for a
	// given test reference.
	ReferenceConclusiveAction(gotest.Reference) gotest.Action

	// ReferenceOutput(gotest.Reference, io.Writer) error // simply not used....

	// ReferenceEvents returns all events associated with a test reference.
	ReferenceEvents(gotest.Reference) []gotest.Event

	// TestStats returns aggregate statistics for all tests in the run.
	TestStats() gotest.ResultStats

	// Elapsed returns the duration of the test run. If stillRunning is true,
	// the duration is calculated from the start time to now.
	Elapsed(stillRunning bool) time.Duration

	// Coverage returns the code coverage percentage and whether coverage data
	// is available.
	Coverage() (float64, bool)

	// Passed returns whether all tests passed and whether the run is still in progress.
	Passed() (bool, bool)
}

// RunController manages the execution of test runs.
type RunController interface {
	// StartTests initiates a new test run with the given configuration. Returns
	// the Run handle and a channel for error notifications.
	StartTests(context.Context, test.RunConfig) (*gotest.Run, <-chan error)
}

// Updater allows updating a test run's state with new events.
type Updater interface {
	// Update processes a test event, updating the run's result state.
	Update(event gotest.Event)
}

// state is an internal implementation of the State interface.
type state struct {
	// current is the currently active test run.
	current *gotest.Run
	RunStore
	RunController
}

// configAdapter adapts a gotest.Run to the RunViewer interface.
type configAdapter struct {
	cfg gotest.RunnerConfig
	*gotest.Result
	*gotest.Run
}

// NewRunViewer creates a RunViewer for the given test run.
func NewRunViewer(run *gotest.Run) RunViewer {
	return configAdapter{
		cfg:    run.Config,
		Result: &run.Result,
		Run:    run,
	}
}

// Config implements RunViewer.
func (r configAdapter) Config() gotest.RunnerConfig {
	return r.cfg
}

// New creates a new State with the given run controller and run store.
func New(manager RunController, store RunStore) State {
	return &state{
		RunController: manager,
		RunStore:      store,
	}
}

// SetCurrentRun sets the currently active test run.
func (s *state) SetCurrentRun(r *gotest.Run) {
	s.current = r
}

// CurrentRun implements State, returning a viewer for the current run.
func (s *state) CurrentRun() RunViewer {
	if s.current == nil {
		return nil
	}
	return NewRunViewer(s.current)
}

// Update implements Updater, updating the current run's result with a new event.
func (s *state) Update(event gotest.Event) {
	if s.current == nil {
		return
	}
	s.current.Result.Update(event)
}
