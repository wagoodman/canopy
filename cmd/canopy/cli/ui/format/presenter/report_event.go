package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/go-partybus"
)

var _ Presenter = (*ReportEvent)(nil)

// ReportEvent presents CLI report events to stdout, trimming trailing whitespace.
type ReportEvent struct {
	// event is the report event to present.
	event partybus.Event
}

// NewReportEvent creates a presenter for CLI report events.
func NewReportEvent(e partybus.Event) Presenter {
	return ReportEvent{event: e}
}

// Present writes the report text to stdout with trailing whitespace removed.
func (p ReportEvent) Present(stdout, _ io.Writer) error {
	_, report, err := parser.ParseCLIReport(p.event)
	if err != nil {
		return fmt.Errorf("failed to parse report: %w", err)
	}

	// remove all whitespace padding from the end of the report
	reportText := strings.TrimRight(report, "\n ") + "\n"

	if _, err := fmt.Fprint(stdout, reportText); err != nil {
		return fmt.Errorf("failed to write final report to stdout: %w", err)
	}
	return nil
}
