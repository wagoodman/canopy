package gotest

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cover"
)

// Run represents a single `go test` execution session with its configuration and accumulated results.
// Each run has a unique ID that ties together all events and results from that execution.
type Run struct {
	ID uuid.UUID

	Config           RunnerConfig
	Result           Result
	PackageCoverage  []cover.PackageResult  // per-package coverage from covdata
	FunctionCoverage []cover.FunctionResult // per-function coverage from covdata

	// Canceled indicates the run was interrupted before completion (e.g. ctrl-c / context cancellation).
	Canceled bool
}

// NewRun creates a new test execution session with a unique identifier.
// The Result field will be populated during test execution.
func NewRun(config RunnerConfig) *Run {
	return &Run{
		ID:     uuid.New(),
		Config: config,
	}
}
