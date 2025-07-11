package presenter

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ Presenter = (*JestTestResultSummary)(nil)

type jestStyle struct {
	bold      lipgloss.Style
	wideTitle lipgloss.Style
	success   lipgloss.Style
	failed    lipgloss.Style
	skipped   lipgloss.Style
}

func newJestStyle(color bool) jestStyle {
	if color {
		return jestStyle{
			bold:      lipgloss.NewStyle().Bold(true),
			wideTitle: lipgloss.NewStyle().Width(10).Bold(true),
			success:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			failed:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			skipped:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		}
	}
	return jestStyle{
		bold:      lipgloss.NewStyle().Bold(true),
		wideTitle: lipgloss.NewStyle().Width(10).Bold(true),
		success:   lipgloss.NewStyle(),
		failed:    lipgloss.NewStyle(),
		skipped:   lipgloss.NewStyle(),
	}
}

type JestTestResultSummaryConfig struct {
	// Color enables/ disables color output
	Color bool

	// ShowElapsed controls whether the elapsed time is shown in the summary
	ShowElapsed bool

	// WriteToStderr controls whether the summary is written to stderr instead of stdout
	WriteToStderr bool

	// DurationFromEvents controls whether the timer should be driven by event timestamps or by the wall clock
	DurationFromEvents bool
}

func (c JestTestResultSummaryConfig) New(run gotest.Run) Presenter {
	return JestTestResultSummary{
		config: c,
		style:  newJestStyle(c.Color),
		run:    run,
	}
}

type JestTestResultSummary struct {
	config JestTestResultSummaryConfig
	style  jestStyle
	run    gotest.Run
}

func (s JestTestResultSummary) Present(stdout, stderr io.Writer) error {
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	stats := s.run.Result.TestStats()

	var header string

	var tests []string

	if stats.Passed > 0 {
		tests = append(tests, s.style.success.Render(fmt.Sprintf("%d passed", stats.Passed)))
	}

	if stats.Failed > 0 {
		tests = append(tests, s.style.failed.Render(fmt.Sprintf("%d failed", stats.Failed)))
	}

	if stats.Skipped > 0 {
		tests = append(tests, s.style.skipped.Render(fmt.Sprintf("%d skipped", stats.Skipped)))
	}

	total := stats.Total()
	if stats.Passed != total || total == 0 {
		tests = append(tests, fmt.Sprintf("%d total", total))
	}

	summary := s.style.wideTitle.Render("Tests: ") + strings.Join(tests, ", ") + "\n"

	if s.config.ShowElapsed {
		el := s.run.Result.Elapsed(!s.config.DurationFromEvents)
		el = el.Truncate(time.Millisecond)
		summary += s.style.wideTitle.Render("Elapsed:") + fmt.Sprintf("%s\n", el)
	}

	percent, ok := s.run.Result.Coverage()
	if ok {
		summary += s.style.wideTitle.Render("Coverage:") + fmt.Sprintf("%0.2f%%\n", percent)
	}

	report := header + summary

	// remove all whitespace padding from the end of the report
	reportText := strings.TrimRight(report, "\n ") + "\n"

	if _, err := fmt.Fprint(w, reportText); err != nil {
		return fmt.Errorf("failed to write final report: %w", err)
	}

	return nil
}
