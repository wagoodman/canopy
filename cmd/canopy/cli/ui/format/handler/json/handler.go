package json

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

type handler struct {
	writer io.Writer
}

func NewHandler(writer io.Writer) partybus.Handler {
	return handler{
		writer: writer,
	}
}

func (n handler) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		fmt.Fprint(n.writer, goTestEvent.JSONL)
	}
	return nil
}
