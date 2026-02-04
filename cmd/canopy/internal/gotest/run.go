package gotest

import (
	"github.com/google/uuid"
	"golang.org/x/tools/cover"
)

// Run represents a single `go test` execution session with its configuration and accumulated results.
// Each run has a unique ID that ties together all events and results from that execution.
type Run struct {
	ID uuid.UUID

	Config           RunnerConfig
	Result           Result
	CoverageProfiles []*cover.Profile // raw profiles for structured storage
}

// NewRun creates a new test execution session with a unique identifier.
// The Result field will be populated during test execution.
func NewRun(config RunnerConfig) *Run {
	return &Run{
		ID:     uuid.New(),
		Config: config,
	}
}
