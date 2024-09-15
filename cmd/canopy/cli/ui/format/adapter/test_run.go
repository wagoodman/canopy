package adapter

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
)

var _ HandledPresenter = (*TestRun)(nil)

type TestRun struct {
	*handler.TestRun
	factory presenter.TestRunFactory
}

func NewTestRun(p presenter.TestRunFactory) *TestRun {
	return &TestRun{
		TestRun: handler.NewTestRun(),
		factory: p,
	}
}

func (t TestRun) Present(stdout, stderr io.Writer) error {
	if t.factory == nil {
		return nil
	}

	if t.TestRun == nil {
		return fmt.Errorf("no test run to present")
	}

	if t.TestRun.Run == nil {
		return nil
	}

	pres := t.factory(*t.TestRun.Run)

	if pres == nil {
		return nil
	}

	return pres.Present(stdout, stderr)
}
