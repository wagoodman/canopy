package state

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

type State interface {
	// RunViewer     // can interpret a single result
	CurrentRun() RunViewer
	RunController // can kick off new runs
	RunStore      // can access historical runs
	Updater       // can update the state of a current run that is not stored
}

type RunStore interface {
	GetRun(uuid.UUID) (*gotest.Run, error)
}

type RunViewer interface {
	Config() gotest.RunnerConfig
	References() []gotest.Reference
	ReferenceConclusiveAction(gotest.Reference) gotest.Action
	// ReferenceOutput(gotest.Reference, io.Writer) error // simply not used....
	ReferenceEvents(gotest.Reference) []gotest.Event
	TestStats() gotest.ResultStats
	Elapsed(bool) time.Duration
	Coverage() (float64, bool)
	Passed() (bool, bool) // indicates pass/fail and is-still-running
}

type RunController interface {
	StartTests(context.Context, test.RunConfig) (*gotest.Run, <-chan error)
}

type Updater interface {
	Update(event gotest.Event)
}

type state struct {
	current *gotest.Run
	RunStore
	RunController
}

type configAdapter struct {
	cfg gotest.RunnerConfig
	*gotest.Result
	*gotest.Run
}

func NewRunViewer(run *gotest.Run) RunViewer {
	return configAdapter{
		cfg:    run.Config,
		Result: &run.Result,
		Run:    run,
	}
}

func (r configAdapter) Config() gotest.RunnerConfig {
	return r.cfg
}

func New(manager RunController, store RunStore) State {
	return &state{
		RunController: manager,
		RunStore:      store,
	}
}

func (s *state) SetCurrentRun(r *gotest.Run) {
	s.current = r
}

func (s *state) CurrentRun() RunViewer {
	if s.current == nil {
		return nil
	}
	return NewRunViewer(s.current)
}

func (s *state) Update(event gotest.Event) {
	if s.current == nil {
		return
	}
	s.current.Result.Update(event)
}
