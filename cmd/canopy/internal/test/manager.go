package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// Manager coordinates test session lifecycle, including creation, persistence, and retrieval.
// It maintains the current session state and provides methods for running tests and accessing results.
type Manager struct {
	// config contains the manager configuration including database settings.
	config Config
	store
	sessionManager
	*session
}

// Config configures the test manager's behavior and persistence.
type Config struct {
	// DBRoot is the directory for SQLite database storage.
	DBRoot string
	// Ephemeral indicates whether to use temporary storage that's deleted on close.
	Ephemeral bool
	// ExistingSession specifies a session UUID to resume (mutually exclusive with LoadLastSession).
	ExistingSession uuid.UUID
	// LoadLastSession indicates whether to resume the most recent session.
	LoadLastSession bool
}

// RunConfig configures a single test run execution.
type RunConfig struct {
	// LogTestFailuresAsErrors determines whether test failures are logged as errors.
	LogTestFailuresAsErrors bool
	// Runner configures how tests are discovered and executed.
	Runner gotest.RunnerConfig
	// Result configures what information is tracked during test execution.
	Result gotest.ResultConfig
	// Reader provides pre-recorded test events for replay (prevents actual test execution).
	Reader io.ReadCloser // prevent from running the test, get events from here instead
}

// NewManager creates a new test session manager with the given configuration.
// If ExistingSession is provided, that session is resumed. If LoadLastSession is true,
// the most recent session is resumed. Otherwise a new session will be created on first run.
// Returns an error if both ExistingSession and LoadLastSession are specified.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.ExistingSession != uuid.Nil && cfg.LoadLastSession {
		return nil, errors.New("cannot specify both an existing session and to load the last session")
	}

	s, err := newDBStore(cfg)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("no store created")
	}
	m := &Manager{
		config:         cfg,
		store:          s,
		sessionManager: s,
	}

	if cfg.ExistingSession != uuid.Nil {
		ts, err := s.getSession(cfg.ExistingSession)
		if err != nil {
			return nil, err
		}
		m.session = ts
	} else if cfg.LoadLastSession {
		sessions, err := s.ListSessions()
		if err != nil {
			return nil, fmt.Errorf("unable to list test sessions: %w", err)
		}

		if len(sessions) == 0 {
			return nil, fmt.Errorf("no test sessions found")
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Started.After(sessions[j].Started)
		})

		ts, err := s.getSession(sessions[0].UUID)
		if err != nil {
			return nil, err
		}
		m.session = ts
	}

	return m, nil
}

// CurrentSession returns information about the active session, or nil if no session exists.
func (s *Manager) CurrentSession() (*SessionInfo, error) {
	if s.session == nil {
		return nil, nil
	}

	sessionInfo, err := s.info()
	if err != nil {
		return nil, err
	}

	return sessionInfo, nil
}

// RunTests executes tests synchronously, blocking until all tests complete or an error occurs.
// Creates a new session if one doesn't exist. Returns the completed test run or the first error encountered.
func (s *Manager) RunTests(ctx context.Context, cfg RunConfig) (*gotest.Run, error) {
	r, errs := s.StartTests(ctx, cfg)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// StartTests begins test execution asynchronously, returning immediately with a run handle and error channel.
// Creates a new session if one doesn't exist. Events are logged, published to the bus, and persisted to storage.
// If cfg.Reader is provided, events are replayed from that reader instead of executing tests.
// The error channel will be closed when execution completes or fails.
func (s *Manager) StartTests(ctx context.Context, cfg RunConfig) (*gotest.Run, <-chan error) { //nolint:funlen
	var r *gotest.Run
	var errs <-chan error
	var runModel *run
	var err error

	if s.session == nil {
		s.session, err = s.newSession()
		if err != nil {
			done := make(chan error)
			go func() {
				done <- err
				close(done)
			}()
			return nil, done
		}
	}

	runModel, err = s.newRun(cfg.Runner)
	if err != nil {
		done := make(chan error)
		go func() {
			done <- err
			close(done)
		}()
		return nil, done
	}

	onEvent := func(event *gotest.Event) {
		if event == nil {
			// this is the end of the test run...
			var coverage *float64
			cov, ok := r.Result.Coverage()
			if ok {
				coverage = &cov
			}
			if err := runModel.end(coverage); err != nil {
				log.WithFields("error", err).Error("unable to end test run")
			}

			bus.TestRun(*r)
			return
		}

		// a test event has occurred...
		e := *event

		logEvent(e, cfg.LogTestFailuresAsErrors)

		bus.TestEvent(e)

		if runModel != nil {
			if err := runModel.addEvent(e); err != nil {
				// TODO:
				panic(err)
			}
		}
	}

	if cfg.Reader != nil {
		// replay json events from the reader
		var evs <-chan *gotest.Event
		r, evs = gotest.StartReplayRun(cfg.Reader, cfg.Runner, cfg.Result)

		// we don't expect any errors, but we're making this adapter for the caller, which in other modes can get errors
		errsCtrl := make(chan error)

		go func() {
			defer close(errsCtrl)
			for e := range evs {
				onEvent(e)
			}
		}()

		errs = errsCtrl
	} else {
		// run the test ourselves
		r, errs = gotest.NewRunner(cfg.Runner).Start(ctx, cfg.Result, onEvent)
	}

	bus.TestRunRequest(r.ID, cfg.Runner)

	return r, errs
}

// GetRun retrieves a previously executed test run by ID, including all events and results.
// Reconstructs the full test run state from persisted events.
func (s *Manager) GetRun(runID uuid.UUID) (*gotest.Run, error) {
	runInfo, err := s.GetRunInfo(runID)
	if err != nil {
		return nil, err
	}

	events, err := s.GetTestEvents(runID)
	if err != nil {
		return nil, err
	}

	r := &gotest.Run{
		ID:     runInfo.UUID,
		Config: runInfo.Config,
		Result: *gotest.NewResult(gotest.ResultConfig{
			TrackOtherOutput:   true,
			TrackFailingOutput: true,
		}),
	}

	r.Result.SetCoverage(runInfo.Coverage)

	for _, e := range events {
		r.Result.Update(e)
	}

	return r, nil
}

// Close ends the current session and cleans up resources.
// If configured as ephemeral, the database directory is removed.
func (s *Manager) Close() error {
	if s.config.Ephemeral {
		if s.config.DBRoot != "" {
			return os.RemoveAll(s.config.DBRoot)
		}
	}
	if s.session != nil {
		return s.end()
	}
	return nil
}
