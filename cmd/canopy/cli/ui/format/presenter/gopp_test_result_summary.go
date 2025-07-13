package presenter

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/scylladb/go-set/strset"
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

	// ShowPackageCount toggles whether the package count is shown in the summary
	ShowPackageCount bool

	// ShowTotalTestCount toggles whether the total test count is shown in the summary
	ShowTotalTestCount bool

	// RunningState is a short string indicating a spinner if running, or the conclusion state if not running
	RunningState string

	// DurationFromEvents controls whether the timer should be driven by event timestamps or by the wall clock
	DurationFromEvents bool

	// ShowRunningPackages toggles whether to show the full name of packages that have tests running in the summary
	// ShowRunningPackages bool

	// ShowRunningTests toggles whether to show the full name of tests in progress in the summary
	ShowRunningTests bool

	// ShowRunningSubTests toggles whether to show the full name of sub-tests in progress in the summary
	// ShowRunningSubTests bool
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
	if s.config.ShowRunningTests {
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

	// these references are in started order... but that doesn't mean they are in the logical topological order if t.Parallel() is used across tests / subtests
	sort.Sort(gotest.References(runningRefs))

	var testFuncsByPackage = make(map[string]*strset.Set)
	var statsByPackage = make(map[string]gotest.ResultStats)
	var testCountByFunction = make(map[string]map[string]int)
	pkgsSet := strset.New()
	var pkgs []string
	for _, ref := range runningRefs {
		if ref.IsPackage() {
			continue
		}
		if !pkgsSet.Has(ref.Package) {
			pkgsSet.Add(ref.Package)
			pkgs = append(pkgs, ref.Package)
			statsByPackage[ref.Package] = s.run.Result.ReferenceTestStats(ref.PackageRef(), false)
		}

		if ref.IsSubTest() {
			continue
		}

		if _, ok := testFuncsByPackage[ref.Package]; !ok {
			testFuncsByPackage[ref.Package] = strset.New()
		}
		testFuncsByPackage[ref.Package].Add(ref.FuncName)
		if _, ok := testCountByFunction[ref.Package]; !ok {
			testCountByFunction[ref.Package] = make(map[string]int)
		}
		testCountByFunction[ref.Package][ref.FuncName]++
	}

	var lines []string
	for _, pkg := range pkgs {
		lines = append(lines, Package{
			Status:         s.config.RunningState,
			NameAsAux:      true,
			Name:           pkg,
			TestsCompleted: 0,
			Aux:            nil,
			Trailer:        "",
			Style:          s.style,
			FormatStatus:   false,
			MaxTestName:    s.config.PackageNameWidth,
		}.String())

		stats := statsByPackage[pkg]

		fmtStats := "\t\t" + s.style.Aux.Render(" ├── ") + s.renderStats(stats)

		funcs := testFuncsByPackage[pkg].List()
		sort.Strings(funcs)

		fmtTests := "\t\t" + s.style.Aux.Render(" └── ") + fmt.Sprintf("%d running: %s", len(funcs), strings.Join(funcs, ", "))

		lines = append(lines, fmtStats, fmtTests)
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

func (s GoPPTestResultSummary) summaryFooter() string {
	passed, isRunning := s.run.Result.Passed()

	var status string
	switch {
	case isRunning:
		if s.config.RunningState != "" {
			status = s.style.Running.Render(s.config.RunningState)
		} else {
			status = s.style.Running.Render("RUNNING")
		}
	case !passed:
		status = s.style.Failed.Render("FAIL")
	default:
		status = s.style.Success.Render("PASS")
	}

	width := lipgloss.Width(status)
	switch {
	case width == 0:
		status = "\t\t"
	case width < 4:
		status += "\t\t"
	case width < 8:
		status += "\t"
	}

	result := status
	var sections []string

	if s.config.ShowPackageCount {
		sections = append(sections, fmt.Sprintf("%d packages", s.config.PackageCount))
	}

	sections = append(sections, s.renderStats(s.run.Result.TestStats()))

	summary := strings.Join(sections, ", ")
	wideSummary := lipgloss.NewStyle().Width(s.config.PackageNameWidth).Render(summary)

	result += wideSummary

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

func (s GoPPTestResultSummary) renderStats(stats gotest.ResultStats) string {
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
	case s.config.ShowTotalTestCount && total != stats.Passed:
		tests = append(tests, fmt.Sprintf("%d total", stats.Total()))
	}

	testSummaryCount := strings.Join(tests, " / ")

	return fmt.Sprintf("%s%s", testSummaryCount, testCountSuffix)
}
