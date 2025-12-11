package adapter

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
)

var _ HandledPresenter = (*TestRun)(nil)

// TestRun combines test run handling with presentation, allowing it to capture
// test run results and then format them for output.
type TestRun struct {
	// TestRun handles and stores test run events.
	*handler.TestRun

	// factory creates a presenter from the captured test run.
	factory presenter.TestRunFactory
}

// NewTestRun creates an adapter that captures test run events and presents
// them using the provided factory.
func NewTestRun(p presenter.TestRunFactory) *TestRun {
	return &TestRun{
		TestRun: handler.NewTestRun(),
		factory: p,
	}
}

// Present formats and writes the captured test run to stdout/stderr.
func (t TestRun) Present(stdout, stderr io.Writer) error {
	if t.factory == nil {
		return nil
	}

	if t.TestRun == nil {
		return fmt.Errorf("no test run to present")
	}

	if t.Run == nil {
		return nil
	}

	pres := t.factory(*t.Run)

	if pres == nil {
		return nil
	}

	return pres.Present(stdout, stderr)
}
