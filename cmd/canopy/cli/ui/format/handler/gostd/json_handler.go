package gostd

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

type jsonHandler struct {
	writer io.Writer
}

func NewJSONHandler(writer io.Writer) partybus.Handler {
	return jsonHandler{
		writer: writer,
	}
}

func (n jsonHandler) Handle(e partybus.Event) error {
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
