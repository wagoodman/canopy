package test

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
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

func (s *noopStore) GetTestEventsBatch(_ uuid.UUID, _, _ int) ([]gotest.Event, bool, error) {
	return nil, false, nil
}

func (s *noopStore) GetTestEventCount(_ uuid.UUID) (int64, error) {
	return 0, nil
}

func (s *noopStore) AddTestEvent(_ uuid.UUID, _ gotest.Event) error {
	return nil // the key optimization: do nothing
}

func (s *noopStore) EndTestRun(_ uuid.UUID, _ *db.CoverageInput) error {
	return nil
}

func (s *noopStore) SetRunCoverageDir(_ uuid.UUID, _ string) error {
	return nil
}

func (s *noopStore) GetFailuresByRun(_ uuid.UUID) ([]db.FailedTestDetails, error) {
	return nil, nil
}

func (s *noopStore) AddSourceState(_ uuid.UUID, _ *db.SourceStateInput) error {
	return nil
}
