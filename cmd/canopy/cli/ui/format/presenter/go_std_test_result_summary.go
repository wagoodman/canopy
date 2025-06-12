package presenter

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ Presenter = (*JestTestResultSummary)(nil)

type GoStdTestResultSummaryConfig struct {
	Color            bool
	WriteToStderr    bool
	PackageNameWidth int
	PackageCount     int
	HidePackageCount bool
	RunningState     string
	StaticTimer      bool

	ShowRunningPackages bool
	ShowRunningTests    bool
}

func (c GoStdTestResultSummaryConfig) New(run gotest.Run) Presenter {
	return GoStdTestResultSummary{
		config: c,
		style:  style.NewGoStd(c.Color),
		run:    run,
	}
}

type GoStdTestResultSummary struct {
	config GoStdTestResultSummaryConfig
	style  style.GoStd
	run    gotest.Run
}

func (s GoStdTestResultSummary) Present(stdout, stderr io.Writer) error { //nolint:funlen
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	footer, err := s.summaryFooter()
	if err != nil {
		return fmt.Errorf("failed to create summary footer: %w", err)
	}

	if _, err := fmt.Fprintln(w, footer); err != nil {
		return fmt.Errorf("failed to write summary footer: %w", err)
	}

	return nil
}

func (s GoStdTestResultSummary) summaryFooter() (string, error) { //nolint:funlen

	passed, isRunning := s.run.Result.Passed()

	var result string
	switch {
	case isRunning:
		if s.config.RunningState != "" {
			result += s.style.Running.Render(s.config.RunningState)
		} else {
			result += s.style.Running.Render("RUNNING")
		}
	case !passed:
		result += s.style.Failed.Render("FAIL")
	default:
		result += s.style.Success.Render("PASS")
	}

	stats := s.run.Result.TestStats()

	var tests []string

	if stats.Passed > 0 {
		tests = append(tests, s.style.Success.Render(fmt.Sprintf("%d passed", stats.Passed)))
	}

	if stats.Failed > 0 {
		tests = append(tests, s.style.Failed.Render(fmt.Sprintf("%d failed", stats.Failed)))
	}

	if stats.Skipped > 0 {
		tests = append(tests, s.style.Skipped.Render(fmt.Sprintf("%d skipped", stats.Skipped)))
	}

	total := stats.Total()
	testCountSuffix := " tests"
	switch {
	case total == 0:
		tests = append(tests, s.style.Waiting.Render("(waiting for tests results)"))
		testCountSuffix = ""
	case total != stats.Passed:
		tests = append(tests, fmt.Sprintf("%d total", stats.Total()))
	}

	testSummaryCount := strings.Join(tests, " / ")

	var sections []string

	if !s.config.HidePackageCount {
		sections = append(sections, fmt.Sprintf("%d packages", s.config.PackageCount))
	}

	sections = append(sections, fmt.Sprintf("%s%s", testSummaryCount, testCountSuffix))

	summary := strings.Join(sections, ", ")
	wideSummary := lipgloss.NewStyle().Width(s.config.PackageNameWidth).Render(summary)

	result += "\t" + wideSummary

	elapsed := s.run.Result.Elapsed(!s.config.StaticTimer)
	if elapsed > 0 {
		result += "\t" + s.style.Aux.Render(elapsed.Round(time.Millisecond).String())
	} else {
		result += "\t" + s.style.Aux.Render("compiling...")
	}

	if coverage, ok := s.run.Result.Coverage(); ok {
		// match the same format changes used in the gostd handlers
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("[%0.1f%% coverage]", coverage))
	}

	return result, nil
}
