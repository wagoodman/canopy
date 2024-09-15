package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/go-partybus"
)

var _ Presenter = (*ReportEvent)(nil)

type ReportEvent struct {
	event partybus.Event
}

func NewReportEvent(e partybus.Event) Presenter {
	return &ReportEvent{event: e}
}

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
