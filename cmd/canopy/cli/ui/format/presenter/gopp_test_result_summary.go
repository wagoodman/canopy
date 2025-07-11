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

type GoPPTestResultSummaryConfig struct {
	// Color enables/ disables color output
	Color bool

	// WriteToStderr controls whether the summary is written to stderr instead of stdout
	WriteToStderr bool

	// PackageNameWidth is the width of the package name in the summary (controls where the aux component column starts)
	PackageNameWidth int

	// PackageCount is the number of packages in the test run
	PackageCount int

	// HidePackageCount hides the package count in the summary
	HidePackageCount bool

	// RunningState is a short string indicating a spinner if running, or the conclusion state if not running
	RunningState string

	// DurationFromEvents controls whether the timer should be driven by event timestamps or by the wall clock
	DurationFromEvents bool

	// ShowRunningPackages toggles whether to show the full name of packages that have tests running in the summary
	ShowRunningPackages bool

	// ShowRunningTests toggles whether to show the full name of tests in progress in the summary
	ShowRunningTests bool

	// ShowRunningSubTests toggles whether to show the full name of sub-tests in progress in the summary
	ShowRunningSubTests bool
}

func (c GoPPTestResultSummaryConfig) New(run gotest.Run) Presenter {
	return GoPPTestResultSummary{
		config: c,
		style:  style.NewGo(c.Color),
		run:    run,
	}
}

type GoPPTestResultSummary struct {
	config GoPPTestResultSummaryConfig
	style  style.Go
	run    gotest.Run
}

func (s GoPPTestResultSummary) Present(stdout, stderr io.Writer) error {
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	var runningFooter string
	if s.config.ShowRunningPackages || s.config.ShowRunningTests {
		runningFooter = s.runningFooter()
	}

	footer := s.summaryFooter()

	if _, err := fmt.Fprintln(w, runningFooter+footer); err != nil {
		return fmt.Errorf("failed to write summary footer: %w", err)
	}

	return nil
}

func (s GoPPTestResultSummary) runningFooter() string {
	runningRefs := s.run.Result.ReferencesByAction(gotest.RunAction)

	var lines []string
	for i, ref := range runningRefs {
		var line string
		switch {
		case s.config.ShowRunningPackages && ref.IsPackage():
			line = Package{
				Status:         s.config.RunningState,
				Name:           ref.Package,
				TestsCompleted: 0,
				Aux:            nil,
				Trailer:        "",
				Style:          s.style,
				FormatStatus:   false,
				MaxTestName:    s.config.PackageNameWidth,
			}.String()
		case s.config.ShowRunningSubTests && ref.IsSubTest():
			subtestBranch := "  └── "
			if i+1 < len(runningRefs) && runningRefs[i+1].IsSubTest() {
				subtestBranch = "  ├── "
			}
			line = Package{
				Status:         "",
				Name:           s.style.Aux.Render(subtestBranch) + ref.SubTestName(true),
				TestsCompleted: 0,
				Aux:            nil,
				Trailer:        "",
				Style:          s.style,
				FormatStatus:   false,
				MaxTestName:    s.config.PackageNameWidth,
			}.String()
		case s.config.ShowRunningTests && !ref.IsSubTest() && !ref.IsPackage():
			line = Package{
				Status:         s.config.RunningState,
				Name:           ref.String(true),
				TestsCompleted: 0,
				Aux:            nil,
				Trailer:        "",
				Style:          s.style,
				FormatStatus:   false,
				MaxTestName:    s.config.PackageNameWidth,
			}.String()
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

func (s GoPPTestResultSummary) summaryFooter() string {
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

	elapsed := s.run.Result.Elapsed(!s.config.DurationFromEvents)
	if elapsed > 0 {
		result += "\t" + s.style.Aux.Render(elapsed.Round(time.Millisecond).String())
	} else {
		result += "\t" + s.style.Aux.Render("compiling...")
	}

	if coverage, ok := s.run.Result.Coverage(); ok {
		// match the same format changes used in the gostd handlers
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("[%0.1f%% coverage]", coverage))
	}

	return result
}
