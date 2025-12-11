// Package handler provides event handlers that process test execution events
// and coordinate their formatting via presenters. It includes aggregation
// utilities and multi-package test output coordination.
package handler

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

// BusHandler is a handler that can process partybus events.
type BusHandler interface {
	partybus.Handler
}

// TestEventHandler is a handler that can process Go test events.
type TestEventHandler interface {
	OnGoTestEvent(gotest.Event) error
}

// Handler combines bus event handling with test event processing and string
// representation of any buffered output not yet written.
type Handler interface {
	BusHandler
	TestEventHandler
	fmt.Stringer // anything buffered not written to the writer
}
