// Package presenter defines interfaces and factories for converting test
// data into formatted output. Presenters handle the actual rendering of
// test results, summaries, and events to stdout/stderr.
package presenter

import (
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

// Presenter converts test data into formatted output written to stdout/stderr.
type Presenter interface {
	Present(stdout, stderr io.Writer) error
}

// EventFactory creates a presenter from a partybus event.
type EventFactory func(e partybus.Event) Presenter

// TestRunFactory creates a presenter from one or more test run results.
type TestRunFactory func(tr ...gotest.Run) Presenter
