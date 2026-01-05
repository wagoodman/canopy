package presenter

import (
	"fmt"
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/go-partybus"
)

var _ Presenter = (*ReportEvent)(nil)

// PrintEvent shows whatever string has been passed.
type PrintEvent struct {
	// event is the report event to present.
	event partybus.Event
}

// NewPrintEvent creates a presenter for a simple string.
func NewPrintEvent(e partybus.Event) Presenter {
	return PrintEvent{event: e}
}

// Present writes the print event text to stdout.
func (p PrintEvent) Present(stdout, _ io.Writer) error {
	msg, err := parser.ParsePrintType(p.event)
	if err != nil {
		return fmt.Errorf("failed to parse print event: %w", err)
	}

	if _, err := fmt.Fprint(stdout, msg); err != nil {
		return fmt.Errorf("failed to write print event to stdout: %w", err)
	}
	return nil
}
