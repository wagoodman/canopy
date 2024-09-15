package handler

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

type BusHandler interface {
	partybus.Handler
}

type TestEventHandler interface {
	OnGoTestEvent(gotest.Event) error
}

type Handler interface {
	BusHandler
	TestEventHandler
	fmt.Stringer // anything buffered not written to the writer
}
