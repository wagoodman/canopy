// Package json provides a handler that writes raw JSON test events in
// JSONL (JSON Chunks) format without additional formatting or transformation.
package json

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

// handler writes JSON Chunks (JSONL) test events directly to output without formatting.
type handler struct {
	// writer is where JSON events are written.
	writer io.Writer
}

// NewHandler creates a handler that writes raw JSON test events in JSONL format.
func NewHandler(writer io.Writer) partybus.Handler {
	return handler{
		writer: writer,
	}
}

// Handle processes partybus events, writing test events as JSON lines.
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
