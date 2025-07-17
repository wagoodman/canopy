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

var _ Presenter = (*GoTestResultSummary)(nil)

type GoTestResultSummaryConfig struct {
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

	// ShowRunningTests toggles whether to show the full name of tests in progress in the summary
	ShowRunningTests bool

	// ShowElapsedForRunningPackages toggles whether the elapsed time for each package is shown in the summary
	ShowElapsedForRunningPackages bool

	ShowTestStatsForRunningPackages bool

	ShowSummaryForUnrenderedPackages bool

	// LoosePackageOrder is used to determine if the packages should be rendered in strict alphabetical order
	// or allow for skipping ahead across packages that are taking a long time to complete (based on the stale duration).
	LoosePackageOrder    bool
	StalePackageDuration time.Duration
}

func DefaultGoTestResultSummaryConfig() GoTestResultSummaryConfig {
	return GoTestResultSummaryConfig{
		Color:                            true,
		ShowRunningTests:                 true,
		ShowElapsedForRunningPackages:    true,
		ShowTestStatsForRunningPackages:  true,
		ShowSummaryForUnrenderedPackages: true,
		// we're running with a true wall clock, so we want to use that. Otherwise you'll see the timers jitter,
		// only updating when there is a test event that arrives.
		DurationFromEvents:   false,
		LoosePackageOrder:    true,            // allow the UI to skip ahead to packages that are taking a long time to complete
		StalePackageDuration: 2 * time.Second, // this is the duration that a package can be stale before the UI skips ahead to the next package
	}
}

func (c GoTestResultSummaryConfig) WithColor(color bool) GoTestResultSummaryConfig {
	c.Color = color
	return c
}

func (c GoTestResultSummaryConfig) WithWriteToStderr(writeToStderr bool) GoTestResultSummaryConfig {
	c.WriteToStderr = writeToStderr
	return c
}

func (c GoTestResultSummaryConfig) WithPackageNameWidth(width int) GoTestResultSummaryConfig {
	c.PackageNameWidth = width
	return c
}

func (c GoTestResultSummaryConfig) WithPackageCount(count int) GoTestResultSummaryConfig {
	c.PackageCount = count
	return c
}

func (c GoTestResultSummaryConfig) WithShowPackageCount(show bool) GoTestResultSummaryConfig {
	c.ShowPackageCount = show
	return c
}

func (c GoTestResultSummaryConfig) WithShowTotalTestCount(show bool) GoTestResultSummaryConfig {
	c.ShowTotalTestCount = show
	return c
}

func (c GoTestResultSummaryConfig) WithRunningState(state string) GoTestResultSummaryConfig {
	c.RunningState = state
	return c
}

func (c GoTestResultSummaryConfig) WithDurationFromEvents(durationFromEvents bool) GoTestResultSummaryConfig {
	c.DurationFromEvents = durationFromEvents
	return c
}

func (c GoTestResultSummaryConfig) WithShowRunningTests(show bool) GoTestResultSummaryConfig {
	c.ShowRunningTests = show
	return c
}

func (c GoTestResultSummaryConfig) WithShowElapsedForRunningPackages(show bool) GoTestResultSummaryConfig {
	c.ShowElapsedForRunningPackages = show
	return c
}

func (c GoTestResultSummaryConfig) WithShowTestStatsForRunningPackages(show bool) GoTestResultSummaryConfig {
	c.ShowTestStatsForRunningPackages = show
	return c
}

func (c GoTestResultSummaryConfig) WithShowSummaryForUnrenderedPackages(show bool) GoTestResultSummaryConfig {
	c.ShowSummaryForUnrenderedPackages = show
	return c
}

func (c GoTestResultSummaryConfig) WithLoosePackageOrder(loose bool) GoTestResultSummaryConfig {
	c.LoosePackageOrder = loose
	return c
}

func (c GoTestResultSummaryConfig) WithStalePackageDuration(duration time.Duration) GoTestResultSummaryConfig {
	c.StalePackageDuration = duration
	return c
}

func (c GoTestResultSummaryConfig) New(run gotest.Run) Presenter {
	return GoTestResultSummary{
		config: c,
		style:  style.NewGo(c.Color),
		run:    run,
	}
}

type GoTestResultSummary struct {
	config GoTestResultSummaryConfig
	style  style.Go
	run    gotest.Run
}

func (s GoTestResultSummary) Present(stdout, stderr io.Writer) error {
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

func (s GoTestResultSummary) runningFooter() string { //nolint:funlen
	runningRefs := s.run.Result.ReferencesByAction(gotest.RunAction)

	// these references are in started order... but that doesn't mean they are in the logical topological order if t.Parallel() is used across tests / subtests
	sort.Sort(gotest.References(runningRefs))

	var testFuncsByPackage = make(map[string]*strset.Set)
	var statsByPackage = make(map[string]gotest.ResultStats)
	var testCountByFunction = make(map[string]map[string]int)
	pkgsSet := strset.New()
	var runningPkgRefs []gotest.Reference
	for _, ref := range runningRefs {
		if ref.IsPackage() {
			continue
		}
		if !pkgsSet.Has(ref.Package) {
			pkgsSet.Add(ref.Package)
			runningPkgRefs = append(runningPkgRefs, ref.PackageRef())
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

	includeRollupLine := func() {
		if s.config.ShowSummaryForUnrenderedPackages {
			completedPkgRefsAfter, pkgStats := s.completedPkgsAfter(s.firstNonStaleRunningRef(runningPkgRefs))

			if len(completedPkgRefsAfter) > 0 {
				aux := []string{"\t"} // no elapsed time for unrendered packages
				if s.config.ShowTestStatsForRunningPackages {
					aux = append(aux, s.renderStats(s.mergeStats(pkgStats), true))
				}

				lines = append(lines, Package{
					Status:       "", // no status for unrendered packages, these are completed
					NameAsAux:    true,
					Name:         fmt.Sprintf("(%d unrendered packages)", len(completedPkgRefsAfter)),
					Aux:          aux,
					Trailer:      "",
					Style:        s.style,
					FormatStatus: false,
					MaxTestName:  s.config.PackageNameWidth,
				}.String())
			}
		}
	}

	for i, runningPkgRef := range runningPkgRefs {
		if i == 0 {
			includeRollupLine()
		}

		elapsed := s.run.Result.ReferenceElapsed(runningPkgRef, !s.config.DurationFromEvents)
		if elapsed < 1*time.Second {
			// low pass filter for events... otherwise we'll see a jitter of a lot of packages that show up briefly
			// as running, but may be removed when completed without printing the final result in cases where
			// a previous package in sort order is still running.
			continue
		}

		var aux []string
		if s.config.ShowElapsedForRunningPackages {
			elapsedStr := formatElapsed(elapsed, true)
			aux = append(aux, elapsedStr)
		}

		if s.config.ShowTestStatsForRunningPackages {
			stats := statsByPackage[runningPkgRef.Package]
			aux = append(aux, s.renderStats(stats, true))
		}

		lines = append(lines, Package{
			Status:       s.config.RunningState,
			NameAsAux:    true,
			Name:         runningPkgRef.Package,
			Aux:          aux,
			Trailer:      "",
			Style:        s.style,
			FormatStatus: false,
			MaxTestName:  s.config.PackageNameWidth,
		}.String())
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

func (s GoTestResultSummary) firstNonStaleRunningRef(runningPkgRefs []gotest.Reference) *gotest.Reference {
	if len(runningPkgRefs) == 0 {
		return nil
	}
	if !s.config.LoosePackageOrder {
		return &runningPkgRefs[0]
	}
	// find the first non-stale running package reference
	for i := range runningPkgRefs {
		ref := runningPkgRefs[i]
		elapsed := s.run.Result.ReferenceElapsed(ref, true)
		if elapsed <= s.config.StalePackageDuration {
			return &ref
		}
	}
	return nil
}

func (s GoTestResultSummary) mergeStats(statsByRef map[gotest.Reference]gotest.ResultStats) gotest.ResultStats {
	var mergedStats gotest.ResultStats
	for _, stats := range statsByRef {
		mergedStats = mergedStats.Merge(stats)
	}
	return mergedStats
}

func (s GoTestResultSummary) completedPkgsAfter(startRunningPkgRef *gotest.Reference) ([]gotest.Reference, map[gotest.Reference]gotest.ResultStats) {
	// add one more line that represents the stats for all unrendered packages (packages after the last running package, that are completed)
	// the order should be compared to the presentation order, which is alphabetical order (not order of started/finished)
	pkgRefs := s.run.Result.Packages()

	sort.Sort(gotest.References(pkgRefs))
	refIdx := -1

	if startRunningPkgRef != nil {
		start := *startRunningPkgRef
		for idx, r := range pkgRefs {
			// we're looking for the reference to start after the given reference...
			if r == start {
				refIdx = idx
				break
			}
		}
	}
	if refIdx == -1 {
		return nil, nil
	}

	var completedPkgsAfter []gotest.Reference
	pkgStats := make(map[gotest.Reference]gotest.ResultStats)
	for _, pkgRef := range pkgRefs[refIdx+1:] {
		action := s.run.Result.ReferenceConclusiveAction(pkgRef)
		if action.Completed() {
			// only include packages that are completed
			completedPkgsAfter = append(completedPkgsAfter, pkgRef)
			pkgStats[pkgRef] = s.run.Result.ReferenceTestStats(pkgRef, false)
		}
	}
	return completedPkgsAfter, pkgStats
}

func (s GoTestResultSummary) summaryFooter() string {
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

	sections = append(sections, s.renderStats(s.run.Result.TestStats(), false))

	summary := strings.Join(sections, ", ")
	wideSummary := lipgloss.NewStyle().Width(s.config.PackageNameWidth).Render(summary)

	result += wideSummary

	elapsed := s.run.Result.Elapsed(!s.config.DurationFromEvents)
	if elapsed > 0 {
		result += "\t" + s.style.Aux.Render(formatElapsed(elapsed, false))
	} else {
		result += "\t" + s.style.Aux.Render("compiling...")
	}

	if coverage, ok := s.run.Result.Coverage(); ok {
		// match the same format changes used in the gostd handlers
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("[%0.1f%% coverage]", coverage))
	}

	return result
}

func (s GoTestResultSummary) renderStats(stats gotest.ResultStats, asAux bool) string {
	var tests []string

	if stats.Passed > 0 {
		st := s.style.Success
		if asAux {
			st = st.Faint(true)
		}
		tests = append(tests, st.Render(fmt.Sprintf("%d passed", stats.Passed)))
	}

	if stats.Failed > 0 {
		st := s.style.Failed
		if asAux {
			st = st.Faint(true)
		}
		tests = append(tests, st.Render(fmt.Sprintf("%d failed", stats.Failed)))
	}

	if stats.Skipped > 0 {
		st := s.style.Skipped
		if asAux {
			st = st.Faint(true)
		}
		tests = append(tests, st.Render(fmt.Sprintf("%d skipped", stats.Skipped)))
	}

	total := stats.Total()
	var testCountSuffix string
	if !asAux {
		testCountSuffix = " tests"
	}
	switch {
	case total == 0:
		tests = append(tests, s.style.Waiting.Render("(waiting for tests results)"))
		testCountSuffix = ""
	case s.config.ShowTotalTestCount && total != stats.Passed:
		totalStr := fmt.Sprintf("%d total", stats.Total())
		if asAux {
			totalStr = s.style.Aux.Render(totalStr)
		}
		tests = append(tests, totalStr)
	}

	testSummaryCount := strings.Join(tests, " / ")

	return fmt.Sprintf("%s%s", testSummaryCount, testCountSuffix)
}

func formatElapsed(elapsed time.Duration, short bool) string {
	elapsed = elapsed.Round(time.Millisecond)

	// no more detail than 2 decimal places
	if short {
		elapsed = elapsed.Truncate(time.Second)
	} else {
		elapsed = elapsed.Truncate(time.Millisecond * 10)
	}

	// even a short duration should use the same sized aux slot as a longer duration (and standard go test elapsed time output)
	return fmt.Sprintf("%-5s", elapsed.String())
}
