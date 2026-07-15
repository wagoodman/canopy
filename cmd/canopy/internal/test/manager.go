package test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

// Manager coordinates test session lifecycle, including creation, persistence, and retrieval.
// It maintains the current session state and provides methods for running tests and accessing results.
type Manager struct {
	// config contains the manager configuration including database settings.
	config Config
	store
	sessionManager
	*session

	// dbStore holds a reference to the underlying db.Store for direct access (e.g., prune operations).
	// This is nil when using noopStore.
	dbStore *db.Store
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
	// SessionName, when set, resolves to a find-or-create session by that name instead of a fresh session per run.
	SessionName string
	// NoStore uses a no-op store (no persistence) for format-only operations.
	NoStore bool
	// Retention configures automatic cleanup of old test runs on startup.
	Retention RetentionConfig
}

// RetentionConfig controls automatic pruning of old test data.
type RetentionConfig struct {
	// MaxRuns is the maximum number of test runs to retain (0 = unlimited).
	MaxRuns int
	// MaxAge is the maximum age of test runs to keep (0 = unlimited).
	MaxAge time.Duration
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
	// SourceState is the optional git source state captured before test execution.
	// Nil when the store is disabled or the directory is not a git repo.
	SourceState *db.SourceStateInput
}

// NewManager creates a new test session manager with the given configuration.
// If ExistingSession is provided, that session is resumed. If LoadLastSession is true,
// the most recent session is resumed. Otherwise a new session will be created on first run.
// Returns an error if both ExistingSession and LoadLastSession are specified.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.ExistingSession != uuid.Nil && cfg.LoadLastSession {
		return nil, errors.New("cannot specify both an existing session and to load the last session")
	}

	var st store
	var sm sessionManager
	var underlyingStore *db.Store

	if cfg.NoStore {
		ns := newNoopStore()
		st = ns
		sm = ns
	} else {
		dbs, err := newDBStore(cfg)
		if err != nil {
			return nil, err
		}
		if dbs == nil {
			return nil, fmt.Errorf("no store created")
		}

		// apply retention policy before starting a new session
		applyRetentionPolicy(dbs.Store, cfg.Retention)

		st = dbs
		sm = dbs
		underlyingStore = dbs.Store
	}

	m := &Manager{
		config:         cfg,
		store:          st,
		sessionManager: sm,
		dbStore:        underlyingStore,
	}

	switch {
	case cfg.ExistingSession != uuid.Nil:
		ts, err := sm.getSession(cfg.ExistingSession)
		if err != nil {
			return nil, err
		}
		m.session = ts
	case cfg.LoadLastSession:
		sessions, err := st.ListSessions()
		if err != nil {
			return nil, fmt.Errorf("unable to list test sessions: %w", err)
		}

		if len(sessions) == 0 {
			return nil, fmt.Errorf("no test sessions found")
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Started.After(sessions[j].Started)
		})

		ts, err := sm.getSession(sessions[0].UUID)
		if err != nil {
			return nil, err
		}
		m.session = ts
	case cfg.SessionName != "":
		// resolve the named session up front so runs append to it instead of minting a fresh session
		ts, err := sm.getOrCreateSession(cfg.SessionName)
		if err != nil {
			return nil, err
		}
		m.session = ts
	}

	return m, nil
}

// DBStore returns the underlying database store for direct operations (e.g., pruning).
// Returns nil when using a no-op store.
func (s *Manager) DBStore() *db.Store {
	return s.dbStore
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
func (s *Manager) StartTests(ctx context.Context, cfg RunConfig) (*gotest.Run, <-chan error) { //nolint:funlen,gocognit
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

	// set up persistent coverage directory if coverage is enabled
	if cfg.Runner.Coverage && s.config.DBRoot != "" {
		coverageDir, absErr := filepath.Abs(filepath.Join(s.config.DBRoot, "coverage", runModel.uuid.String()))
		if absErr != nil {
			log.WithFields("error", absErr).Warn("failed to resolve coverage directory path")
		} else if mkErr := os.MkdirAll(coverageDir, 0o755); mkErr != nil {
			log.WithFields("error", mkErr).Warn("failed to create coverage directory")
		} else {
			cfg.Runner.CoverageDir = coverageDir
			// persist the dir immediately so DeleteRuns can clean it up even if covdata
			// never produces data (build failure, replay, early error, aborted run).
			if setErr := runModel.setCoverageDir(coverageDir); setErr != nil {
				log.WithFields("error", setErr, "run", runModel.uuid).Warn("failed to persist coverage directory")
			}
		}
	}

	if cfg.SourceState != nil {
		if err := s.AddSourceState(runModel.uuid, cfg.SourceState); err != nil {
			log.WithFields("error", err).Warn("failed to store source state")
		}
	}

	onEvent := func(event *gotest.Event) {
		if event == nil {
			// this is the end of the test run...
			var coverage *db.CoverageInput
			cov, ok := r.Result.Coverage()
			if ok {
				coverage = &db.CoverageInput{
					Percent:     cov,
					CoverageDir: cfg.Runner.CoverageDir,
					Packages:    r.PackageCoverage,
					Functions:   r.FunctionCoverage,
				}
			}
			if err := runModel.end(coverage); err != nil {
				log.WithFields("error", err).Error("unable to end test run")
			}

			publishTestRun(*r)
			return
		}

		// a test event has occurred...
		e := *event

		logEvent(e, cfg.LogTestFailuresAsErrors)

		publishTestEvent(e)

		if runModel != nil {
			if err := runModel.addEvent(e); err != nil {
				// don't crash the whole app on a transient store error (e.g. SQLITE_BUSY, a
				// unique-constraint race, disk full) on the hot event path; log and continue.
				log.WithFields("error", err, "run", runModel.uuid).Error("failed to persist test event")
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

	if r == nil {
		// the runner failed to start (e.g. go missing from PATH, pipe/fd exhaustion);
		// errs already carries the error, so bail before dereferencing a nil run.
		return nil, errs
	}

	publishTestRunRequest(r.ID, cfg.Runner)

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

// GetTestEventsBatch retrieves a batch of test events with pagination support.
// This enables streaming large test runs without loading all events into memory.
func (s *Manager) GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]gotest.Event, bool, error) {
	return s.store.GetTestEventsBatch(runID, offset, limit)
}

// GetTestEventCount returns the total number of events for a run.
func (s *Manager) GetTestEventCount(runID uuid.UUID) (int64, error) {
	return s.store.GetTestEventCount(runID)
}

// applyRetentionPolicy prunes old test runs based on the retention configuration.
// This is called on startup for test execution commands only (not read-only commands).
func applyRetentionPolicy(store *db.Store, retention RetentionConfig) {
	if retention.MaxAge > 0 {
		if n, err := store.DeleteRunsByAge(retention.MaxAge); err != nil {
			log.WithFields("error", err).Warn("failed to prune old test runs")
		} else if n > 0 {
			log.WithFields("deleted", n).Info("pruned old test runs")
		}
	}

	if retention.MaxRuns > 0 {
		if n, err := store.DeleteRunsKeepingLast(retention.MaxRuns); err != nil {
			log.WithFields("error", err).Warn("failed to prune excess test runs")
		} else if n > 0 {
			log.WithFields("deleted", n).Info("pruned excess test runs")
		}
	}

	if _, err := store.DeleteOrphanedSessions(); err != nil {
		log.WithFields("error", err).Warn("failed to clean up orphaned sessions")
	}
}

// publishTestRun publishes a go test run event to the bus.
// This represents the overall status of a test run execution.
func publishTestRun(r gotest.Run) {
	bus.Publish(partybus.Event{
		Type:  event.GoTestRunType,
		Value: r,
	})
}

// publishTestEvent publishes a go test event to the bus.
// This is used to broadcast individual test execution events (run, pass, fail, output, etc).
func publishTestEvent(e gotest.Event) {
	bus.Publish(partybus.Event{
		Type:  event.GoTestType,
		Value: e,
	})
}

// publishTestRunRequest publishes a test run request event to the bus.
// The request includes a unique ID and the runner configuration to execute.
func publishTestRunRequest(id uuid.UUID, r gotest.RunnerConfig) {
	bus.Publish(partybus.Event{
		Type:   event.GoTestRunRequestType,
		Value:  r,
		Source: id,
	})
}
