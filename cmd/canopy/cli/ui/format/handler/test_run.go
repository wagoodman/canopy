package handler

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

var _ partybus.Handler = (*TestRun)(nil)

// TestRun is a handler that captures a complete test run result from the event bus.
// It stores the run information for later use by presenters.
type TestRun struct {
	// Run is the captured test run result.
	*gotest.Run
}

// NewTestRun creates a new test run handler that will capture run results.
func NewTestRun() *TestRun {
	return &TestRun{}
}

// Handle processes partybus events, extracting and storing test run results.
func (r *TestRun) Handle(e partybus.Event) error {
	if e.Type == event.GoTestRunType {
		o, err := parser.ParseGoTestRunType(e)
		if err != nil {
			return fmt.Errorf("failed to parse test suite result: %w", err)
		}
		r.Run = o
	}
	return nil
}
