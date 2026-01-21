package test

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ store = (*noopStore)(nil)
var _ sessionManager = (*noopStore)(nil)

// noopStore is a no-operation store for format-only operations
// that don't need persistence.
type noopStore struct{}

func newNoopStore() *noopStore {
	return &noopStore{}
}

// sessionManager implementation

func (s *noopStore) newSession() (*session, error) {
	return &session{
		store: s,
		uuid:  uuid.New(),
	}, nil
}

func (s *noopStore) getSession(id uuid.UUID) (*session, error) {
	return &session{
		store: s,
		uuid:  id,
	}, nil
}

// sessionStore implementation

func (s *noopStore) GetSessionInfo(id uuid.UUID) (*SessionInfo, error) {
	return &SessionInfo{UUID: id}, nil
}

func (s *noopStore) ListSessions() ([]SessionInfo, error) {
	return nil, nil
}

func (s *noopStore) EndTestSession(_ uuid.UUID) error {
	return nil
}

// runStore implementation

func (s *noopStore) StartTestRun(_ uuid.UUID, _ gotest.RunnerConfig) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (s *noopStore) GetRunInfo(runID uuid.UUID) (RunInfo, error) {
	return RunInfo{UUID: runID}, nil
}

func (s *noopStore) GetTestEvents(_ uuid.UUID) ([]gotest.Event, error) {
	return nil, nil
}

func (s *noopStore) AddTestEvent(_ uuid.UUID, _ gotest.Event) error {
	return nil // the key optimization: do nothing
}

func (s *noopStore) EndTestRun(_ uuid.UUID, _ *float64) error {
	return nil
}
