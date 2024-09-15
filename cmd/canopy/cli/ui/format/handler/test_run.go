package handler

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

var _ partybus.Handler = (*TestRun)(nil)

type TestRun struct {
	*gotest.Run
}

func NewTestRun() *TestRun {
	return &TestRun{}
}

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
