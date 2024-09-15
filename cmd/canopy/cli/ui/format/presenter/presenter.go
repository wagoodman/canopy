package presenter

import (
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

type Presenter interface {
	Present(stdout, stderr io.Writer) error
}

type EventFactory func(e partybus.Event) Presenter

type TestRunFactory func(tr gotest.Run) Presenter
