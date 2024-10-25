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

type Manager struct {
	config Config
	store
	*session
}

type Config struct {
	DBRoot          string
	Ephemeral       bool
	ExistingSession uuid.UUID
	LoadLastSession bool
}

type RunConfig struct {
	LogTestFailuresAsErrors bool
	Runner                  gotest.RunnerConfig
	Result                  gotest.ResultConfig
	Reader                  io.ReadCloser // prevent from running the test, get events from here instead
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.ExistingSession != uuid.Nil && cfg.LoadLastSession {
		return nil, errors.New("cannot specify both an existing session and to load the last session")
	}

	s, err := newStore(cfg)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("no store created")
	}
	m := &Manager{
		config: cfg,
		store:  *s,
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

func (s *Manager) CurrentSession() (*SessionInfo, error) {
	if s.session == nil {
		return nil, nil
	}

	sessionInfo, err := s.store.getSessionInfo(s.session.uuid)
	if err != nil {
		return nil, err
	}

	return sessionInfo, nil
}

func (s *Manager) RunTests(ctx context.Context, cfg RunConfig) (*gotest.Run, error) {
	r, errs := s.StartTests(ctx, cfg)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (s *Manager) StartTests(ctx context.Context, cfg RunConfig) (*gotest.Run, <-chan error) { //nolint:funlen
	var r *gotest.Run
	var errs <-chan error
	var runModel *run
	var err error

	if s.session == nil {
		s.session, err = s.store.newSession()
		if err != nil {
			done := make(chan error)
			go func() {
				done <- err
				close(done)
			}()
			return nil, done
		}
	}

	runModel, err = s.session.newRun(cfg.Runner)
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

	return r, errs
}

func (s *Manager) GetRun(runID uuid.UUID) (*gotest.Run, error) {
	tr, err := s.store.db.GetTestRun(runID)
	if err != nil {
		return nil, err
	}

	runInfo := newRunInfo(tr)

	eventInfos, err := s.store.db.GetTestEvents(runID)
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

	r := &gotest.Run{
		ID:     runInfo.UUID,
		Config: runInfo.Config,
		Result: *gotest.NewResult(gotest.ResultConfig{
			TrackOtherOutput:   true,
			TrackFailingOutput: true,
		}),
	}

	r.Result.SetCoverage(tr.Coverage)

	for _, e := range events {
		r.Result.Update(e)
	}

	return r, nil
}

// func (s *Manager) GetRunResult(runID uuid.UUID) (*gotest.RunnerConfig, *gotest.Result, error) {
//	r, err := s.store.db.GetTestRun(runID)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	eventInfos, err := s.store.db.GetTestEvents(runID)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	var events []gotest.Event
//	for _, e := range eventInfos {
//		var annotations []gotest.Annotation
//		for _, a := range e.Annotations {
//			annotations = append(annotations, gotest.Annotation(a.Value))
//		}
//
//		events = append(events, gotest.Event{
//			RunID: runID,
//			JSONL: "", // TODO: is this a problem?
//			Time:  e.Time,
//			Reference: gotest.Reference{
//				Package:  e.Reference.Package,
//				FuncName: e.Reference.FuncName,
//				TRunName: e.Reference.TRunName,
//			},
//			Action:      gotest.Action(e.Action),
//			Output:      e.Output,
//			Annotations: annotations,
//		})
//
//	}
//
//	result := gotest.NewResult(gotest.ResultConfig{
//		TrackOtherOutput:   true,
//		TrackFailingOutput: true,
//	})
//
//	for _, e := range events {
//		result.Update(e)
//	}
//
//	runInfo := newRunInfo(r)
//
//	return &runInfo.Config, result, nil
//}

func (s *Manager) Close() error {
	if s.config.Ephemeral {
		if s.config.DBRoot != "" {
			return os.RemoveAll(s.config.DBRoot)
		}
	}
	if s.session != nil {
		return s.session.end()
	}
	return nil
}
