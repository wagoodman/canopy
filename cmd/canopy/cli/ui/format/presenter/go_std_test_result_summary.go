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

func (s GoStdTestResultSummary) Present(stdout, stderr io.Writer) error {
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	passed, _ := s.run.Result.Passed()

	result := s.style.Success.Render("PASS")
	if !passed {
		result = s.style.Failed.Render("FAIL")
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

	tests = append(tests, fmt.Sprintf("%d total", stats.Total()))

	testSummaryCount := strings.Join(tests, " / ")

	summary := fmt.Sprintf("%d packages, %s tests", s.config.PackageCount, testSummaryCount)
	wideSummary := lipgloss.NewStyle().Width(s.config.PackageNameWidth).Render(summary)

	result += "\t" + wideSummary

	result += "\t" + s.style.Aux.Render(s.run.Elapsed().Round(time.Millisecond).String())

	if coverage, ok := s.run.Result.Coverage(); ok {
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("coverage: %0.1f%% of statements", coverage))
	}

	if _, err := fmt.Fprintln(w, result); err != nil {
		return fmt.Errorf("failed to write final report: %w", err)
	}

	return nil
}
