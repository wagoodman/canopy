package presenter

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ Presenter = (*GoTestResultSummary)(nil)

// Timing model. A run moves through these phases, and every timer shown is anchored to one of them:
//
//	launch     canopy starts the `go test` process (GoSummaryConfig.StartedAt)
//	compiling  go test loads and builds packages. Nothing is reported for a package until its binary is linked.
//	starting   per package: go test writes the package's "start" event, checks the test cache, then execs the test
//	           binary. The binary hasn't reported anything yet. On macOS this can take seconds because the OS scans
//	           each freshly linked binary on its first exec. Package init and a silent TestMain also land here.
//	running    per package: the binary has reported (a test started, or package-level output)
//	done       per package: the package pass/fail/skip event. For the run: the run-end event (EndedAt).
//
// Compiling and starting overlap across packages: go test links, launches, and runs binaries concurrently.
//
// The timers:
//
//	footer        wall clock from launch to run end, so compiling is included (see elapsed). Without a launch
//	              time (non-TTY output, replays) it falls back to the span of the events seen.
//	started after from launch to when canopy received the first test event, noted once at the far right of the
//	              first package result line whose tests ran as "(started after 14.08s)" (see gostd.firstTestNote).
//	              It never changes once known, so it sits beside that result rather than in this summary.
//	package row   from the package's "start" event to now, so the starting phase is included (see runningRows).
//	              This matches go test's own number: the "ok pkg 3.4s" line times the binary from just before exec
//	              to exit, and the JSON pass/fail Elapsed counts from the "start" event itself. The two differ only
//	              by the cache check. Switching a row from starting to running changes its glyph, never its timer,
//	              so the live row hands off to the same number go prints when the package finishes.
//	tests         per finished package, from its first test event to its last, shown after go's number on the
//	              "ok"/"FAIL" line as "[tests 0.021s]" (see gotest.PackagePhases and gostd.withTestsElapsed). Only
//	              shown when that package's startup reaches gotest.NotableStartup, the same bar the started after note
//	              uses.
//
// Startup, as split out by gotest.PackagePhases, ends at the first test event, while the starting state ends at the
// binary's first event of any kind. They differ only when a package writes output before its first test (e.g.
// TestMain logging during setup): that package's row shows running, but the setup still counts toward startup.

// startedGlyph and waitingGlyph mark the phases of the waiting line shown before any test has reported. The
// waiting glyph also marks package rows in the starting phase. Both are plain text symbols with no emoji
// presentation, so terminals won't render them as color emoji.
//
// The started count is how many packages have sent a "start" event, which go test only sends once a package's test
// binary is built and every package before it (in the order given to go test) has started. That makes it a lower
// bound on what is built: a package that hasn't started may still be compiling, or be built and waiting its turn.
// So the UI says "started", never "compiling".
const (
	startedGlyph = "⛭"
	waitingGlyph = "⧖"
)

// elapsedPlaceholder fills the elapsed column for lines that have no elapsed time. It must be the same width as a
// rendered elapsed value, otherwise the following tab lands on a different tab stop and the stats column is offset.
var elapsedPlaceholder = strings.Repeat(" ", len(formatElapsed(0, true)))

type GoSummaryConfig struct {
	// Color enables/ disables color output
	Color bool

	// WriteToStderr controls whether the summary is written to stderr instead of stdout
	WriteToStderr bool

	// PackageNameWidth is the width of the package name in the summary (controls where the aux component column starts)
	PackageNameWidth int

	// StripPackagePrefix removes the given prefix from package names in the summary (usually the module path)
	StripPackagePrefix string

	// ShowPackageCount toggles whether the package count is shown in the summary
	ShowPackageCount bool

	// ShowTotalTestCount toggles whether the total test count is shown in the summary
	ShowTotalTestCount bool

	// RunningState is a short string indicating a spinner if running, or the conclusion state if not running
	RunningState string

	// Window is the current terminal window size, used to determine how much space is available for rendering
	Window tea.WindowSizeMsg

	// DurationFromEvents controls whether the timer should be driven by event timestamps or by the wall clock
	DurationFromEvents bool

	// ShowRunningTests toggles whether to show the full name of tests in progress in the summary
	ShowRunningTests bool

	// ShowElapsedForRunningPackages toggles whether the elapsed time for each package is shown in the summary. It
	// counts from the package's "start" event, so it includes the starting phase (see the timing model above).
	ShowElapsedForRunningPackages bool

	ShowTestStatsForRunningPackages bool

	ShowSummaryForUnrenderedPackages bool

	// LoosePackageOrder is used to determine if the packages should be rendered in strict alphabetical order
	// or allow for skipping ahead across packages that are taking a long time to complete (based on the stale duration).
	LoosePackageOrder    bool
	StalePackageDuration time.Duration

	CombineMultipleRuns bool

	// HidePackagesWithNoTests indicates whether packages with no tests are being hidden from display.
	// When true and there are hidden packages, the summary footer will show a count of such packages.
	HidePackagesWithNoTests bool

	// Canceled indicates the run was interrupted before completion, so the summary should report a
	// cancellation instead of a PASS/FAIL conclusion.
	Canceled bool

	// Running indicates the run-end event has not been seen yet. The results alone can't tell this: between
	// packages (e.g. while the next package is still compiling) every reference seen so far has concluded,
	// which would otherwise read as a final PASS/FAIL.
	Running bool

	// StartedAt is the wall clock time canopy launched the go test process (the launch phase in the timing model).
	// When set, the footer timer counts from here instead of from the first event, so compiling is included.
	// Canopy's own setup before launch (e.g. resolving packages) is not.
	StartedAt time.Time

	// EndedAt is the wall clock time the run-end event arrived, which stops the footer timer. Zero while running.
	EndedAt time.Time
}

func DefaultGoTestResultSummaryConfig() GoSummaryConfig {
	return GoSummaryConfig{
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

func (c GoSummaryConfig) WithColor(color bool) GoSummaryConfig {
	c.Color = color
	return c
}

func (c GoSummaryConfig) WithWriteToStderr(writeToStderr bool) GoSummaryConfig {
	c.WriteToStderr = writeToStderr
	return c
}

func (c GoSummaryConfig) WithPackageNameWidth(width int) GoSummaryConfig {
	c.PackageNameWidth = width
	return c
}

func (c GoSummaryConfig) WithStripPackagePrefix(prefix string) GoSummaryConfig {
	c.StripPackagePrefix = prefix
	return c
}

func (c GoSummaryConfig) WithShowPackageCount(show bool) GoSummaryConfig {
	c.ShowPackageCount = show
	return c
}

func (c GoSummaryConfig) WithShowTotalTestCount(show bool) GoSummaryConfig {
	c.ShowTotalTestCount = show
	return c
}

func (c GoSummaryConfig) WithRunningState(state string) GoSummaryConfig {
	c.RunningState = state
	return c
}

func (c GoSummaryConfig) WithDurationFromEvents(durationFromEvents bool) GoSummaryConfig {
	c.DurationFromEvents = durationFromEvents
	return c
}

func (c GoSummaryConfig) WithShowRunningTests(show bool) GoSummaryConfig {
	c.ShowRunningTests = show
	return c
}

func (c GoSummaryConfig) WithShowElapsedForRunningPackages(show bool) GoSummaryConfig {
	c.ShowElapsedForRunningPackages = show
	return c
}

func (c GoSummaryConfig) WithShowTestStatsForRunningPackages(show bool) GoSummaryConfig {
	c.ShowTestStatsForRunningPackages = show
	return c
}

func (c GoSummaryConfig) WithShowSummaryForUnrenderedPackages(show bool) GoSummaryConfig {
	c.ShowSummaryForUnrenderedPackages = show
	return c
}

func (c GoSummaryConfig) WithLoosePackageOrder(loose bool) GoSummaryConfig {
	c.LoosePackageOrder = loose
	return c
}

func (c GoSummaryConfig) WithStalePackageDuration(duration time.Duration) GoSummaryConfig {
	c.StalePackageDuration = duration
	return c
}

func (c GoSummaryConfig) WithCombineMultipleRuns(combine bool) GoSummaryConfig {
	c.CombineMultipleRuns = combine
	return c
}

func (c GoSummaryConfig) WithHidePackagesWithNoTests(hide bool) GoSummaryConfig {
	c.HidePackagesWithNoTests = hide
	return c
}

func (c GoSummaryConfig) New(runs ...gotest.Run) Presenter {
	return GoTestResultSummary{
		config:  c,
		style:   style.NewGo(c.Color),
		results: newJoinedResults(runs...),
	}
}

type GoTestResultSummary struct {
	config  GoSummaryConfig
	style   style.Go
	results result
}

func (s GoTestResultSummary) Present(stdout, stderr io.Writer) error {
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	var rows []string
	if s.config.ShowRunningTests {
		rows = s.runningRows()
	}

	footer, waiting := s.waitingFooter(len(rows) > 0)
	if !waiting {
		footer = s.summaryFooter()
	}

	var block string
	if len(rows) > 0 {
		block = strings.Join(rows, "\n") + "\n"
	}

	if _, err := fmt.Fprintln(w, block+footer); err != nil {
		return fmt.Errorf("failed to write summary footer: %w", err)
	}

	return nil
}

// runningRows renders one row per in-flight package in presentation order, preceded by a rollup row for completed
// packages that aren't shown individually. The caller places these above the footer.
func (s GoTestResultSummary) runningRows() []string {
	runningPkgRefs := s.inFlightPackages()

	var lines []string
	if row, ok := s.unrenderedRow(runningPkgRefs); ok {
		lines = append(lines, row)
	}

	for _, runningPkgRef := range runningPkgRefs {
		// counted from the package's "start" event, so the starting phase is included. This is deliberate: it is the
		// same baseline go test uses for the "ok pkg 3.4s" line that replaces this row (see the timing model above).
		elapsed := s.results.ReferenceElapsed(runningPkgRef, !s.config.DurationFromEvents)
		if elapsed < 1*time.Second {
			// low pass filter for events... otherwise we'll see a jitter of a lot of packages that show up briefly
			// as running, but may be removed when completed without printing the final result in cases where
			// a previous package in sort order is still running.
			continue
		}

		// starting packages get a row each rather than a shared count: they hold back the body's alphabetical output
		// just like running packages do, so without their rows the unrendered rollup has no visible cause
		starting := s.packageStarting(runningPkgRef)

		var aux []string
		if s.config.ShowElapsedForRunningPackages {
			elapsedStr := formatElapsed(elapsed, true)
			aux = append(aux, elapsedStr)
		}

		if s.config.ShowTestStatsForRunningPackages {
			if starting {
				aux = append(aux, s.style.Waiting.Render("(starting)"))
			} else {
				aux = append(aux, s.renderStats(s.results.ReferenceTestStats(runningPkgRef, false), true))
			}
		}

		lines = append(lines, Package{
			Status:       s.packageStatus(starting),
			NameAsAux:    true,
			Name:         runningPkgRef.Package,
			Aux:          aux,
			Trailer:      "",
			Style:        s.style,
			FormatStatus: false,
			MaxTestName:  s.config.PackageNameWidth,
			StripPrefix:  s.config.StripPackagePrefix,
		}.String())
	}

	return lines
}

// unrenderedRow is the rollup row for completed packages that the body hasn't printed yet because an earlier
// package (in presentation order) is still in flight. Starting packages count as in flight here too, since the body
// waits on them the same way.
func (s GoTestResultSummary) unrenderedRow(inFlightPkgRefs []gotest.Reference) (string, bool) {
	if !s.config.ShowSummaryForUnrenderedPackages || len(inFlightPkgRefs) == 0 {
		return "", false
	}

	completedPkgRefsAfter, pkgStats := s.completedPkgsAfter(s.firstNonStaleRunningRef(inFlightPkgRefs))
	if len(completedPkgRefsAfter) == 0 {
		return "", false
	}

	aux := []string{elapsedPlaceholder}
	// these packages are done, so no results means none are coming (e.g. no test files), not that we're waiting
	if stats := s.mergeStats(pkgStats); s.config.ShowTestStatsForRunningPackages && stats.Total() > 0 {
		aux = append(aux, s.renderStats(stats, true))
	}

	return Package{
		Status:       "", // no status for unrendered packages, these are completed
		NameAsAux:    true,
		Name:         fmt.Sprintf("(%d unrendered pkgs)", len(completedPkgRefsAfter)),
		Aux:          aux,
		Style:        s.style,
		FormatStatus: false,
		MaxTestName:  s.config.PackageNameWidth,
		StripPrefix:  s.config.StripPackagePrefix,
	}.String(), true
}

// inFlightPackages returns the packages that haven't concluded, in presentation (alphabetical) order. That is every
// package with a running test, plus any started package without one: its test binary is still launching, or it is
// between tests (e.g. in TestMain). Without the latter a package can be in flight with no row at all.
func (s GoTestResultSummary) inFlightPackages() []gotest.Reference {
	pkgsSet := strset.New()
	var refs []gotest.Reference
	add := func(pkgRef gotest.Reference) {
		if !pkgsSet.Has(pkgRef.Package) {
			pkgsSet.Add(pkgRef.Package)
			refs = append(refs, pkgRef)
		}
	}

	for _, ref := range s.results.ReferencesByAction(gotest.RunAction) {
		if !ref.IsPackage() {
			add(ref.PackageRef())
		}
	}

	for _, pkgRef := range s.results.Packages() {
		if !s.results.ReferenceConclusiveAction(pkgRef).Completed() {
			add(pkgRef)
		}
	}

	sort.Sort(gotest.References(refs))
	return refs
}

// packageStatus is the status for in-flight package rows: a static waiting glyph while the test binary is still
// launching, the spinner once it is running tests, or the canceled glyph once interrupted.
func (s GoTestResultSummary) packageStatus(starting bool) string {
	switch {
	case s.config.Canceled:
		// packages still in flight were interrupted, a frozen spinner frame would read as a hung UI. Match the
		// footer's canceled glyph color so the whole interrupted block reads as one state.
		return s.style.Failed.Render(style.CanceledGlyph)
	case starting:
		return s.style.Aux.Render(waitingGlyph)
	}
	return s.style.Running.Render(s.config.RunningState)
}

// packageStarting reports whether the package is in the starting phase: go test wrote its "start" event (the binary
// is linked and about to be exec'd) but the binary hasn't reported anything yet. Anything the binary writes, a test
// starting or package-level output, lands as a second event or a child reference, which moves it to running.
func (s GoTestResultSummary) packageStarting(pkgRef gotest.Reference) bool {
	events := s.results.ReferenceEvents(pkgRef)
	return len(events) == 1 && events[0].Action == gotest.StartAction && len(s.results.Children(pkgRef)) == 0
}

// elapsed is the footer timer. With a known launch time it is wall clock time from launch to run end (or now), which
// covers compiling, starting, and running. Otherwise it falls back to the span from the first event seen, which
// misses any compiling that happened before that event.
func (s GoTestResultSummary) elapsed() time.Duration {
	if s.config.StartedAt.IsZero() {
		return s.results.Elapsed(!s.config.DurationFromEvents)
	}
	end := s.config.EndedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(s.config.StartedAt)
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
		elapsed := s.results.ReferenceElapsed(ref, true)
		if elapsed <= s.config.StalePackageDuration {
			return &ref
		}
	}
	return nil
}

func (s GoTestResultSummary) mergeStats(statsByRef map[gotest.Reference]gotest.ResultStats) gotest.ResultStats {
	var mergedStats gotest.ResultStats
	for _, stats := range statsByRef {
		mergedStats.Merge(stats)
	}
	return mergedStats
}

func (s GoTestResultSummary) completedPkgsAfter(startRunningPkgRef *gotest.Reference) ([]gotest.Reference, map[gotest.Reference]gotest.ResultStats) {
	// add one more line that represents the stats for all unrendered packages (packages after the last running package, that are completed)
	// the order should be compared to the presentation order, which is alphabetical order (not order of started/finished)
	pkgRefs := s.results.Packages()

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
		action := s.results.ReferenceConclusiveAction(pkgRef)
		if action.Completed() {
			// only include packages that are completed
			completedPkgsAfter = append(completedPkgsAfter, pkgRef)
			pkgStats[pkgRef] = s.results.ReferenceTestStats(pkgRef, false)
		}
	}
	return completedPkgsAfter, pkgStats
}

// footerStatus renders the pass/fail/running/canceled glyph, tab-padded to the status column width.
func (s GoTestResultSummary) footerStatus() string {
	return statusColumn(s.footerStatusGlyph())
}

// footerStatusGlyph renders the pass/fail/running/canceled indicator without any column padding.
func (s GoTestResultSummary) footerStatusGlyph() string {
	var status string
	switch {
	case s.config.Canceled:
		// a canceled run takes precedence over any pass/fail/running state: the results are incomplete,
		// so reporting PASS would be misleading. use a glyph here (the word doesn't fit the status
		// column) and explain the interruption on a trailer line below the summary.
		status = s.style.Failed.Render(style.CanceledGlyph)
	case s.config.Running:
		runningState := s.config.RunningState
		if runningState == "" {
			runningState = "RUNNING"
		}
		status = s.style.Running.Render(runningState)
	case !s.results.Passed():
		status = s.style.Failed.Render("FAIL")
	default:
		status = s.style.Success.Render("PASS")
	}

	return status
}

// footerBranch leads a footer line that hangs off the line above it.
const footerBranch = "└─ "

// summaryFooter renders the footer once tests have reported. Mid-run it has two levels, the way the results are
// produced: packages on top (the status glyph, then the package progress bar), and the tests from those packages
// branching underneath. Once the run ends or is canceled the package level goes away, collapsing to the tests line.
//
//	⣧       ━━━━━━━━━━━━────────  43/89 pkgs completed (10 failed)
//	        └─ 459 passed / 41 failed / 5 skipped tests       9.59s   (6 pkgs w/o tests)
//
//	PASS    812 passed / 5 skipped tests                      14.2s   (6 pkgs w/o tests)
func (s GoTestResultSummary) summaryFooter() string {
	var result string
	if packages, ok := s.packageProgressLine(); ok {
		// the branch indents the tests line, so its summary column narrows by the same amount to keep the elapsed time
		// on the tab stop the package rows use
		result = s.footerStatus() + packages + "\n" +
			statusColumn("") + s.style.Aux.Render(footerBranch) + s.testsLine(s.config.PackageNameWidth-lipgloss.Width(footerBranch))
	} else {
		result = s.footerStatus() + s.testsLine(s.config.PackageNameWidth)
	}

	if s.config.Canceled {
		// call out the interruption in red on its own trailer line, since the glyph alone is ambiguous
		result += "\n" + s.style.Failed.Render("└──▶ canceled by user")
	}

	return result
}

// testsLine renders the test counts padded to colWidth, then the elapsed time and the extras that follow it. The
// caller supplies whatever leads the line (a status column, or a branch).
func (s GoTestResultSummary) testsLine(colWidth int) string {
	var sections []string

	if s.config.ShowPackageCount {
		sections = append(sections, fmt.Sprintf("%d pkgs", len(s.results.Packages())))
	}

	stats := s.results.TestStats()
	sections = append(sections, s.renderStats(stats, false))

	summary := strings.Join(sections, " ")
	// pad to the column width, but never below the content width, else lipgloss word-wraps a summary wider than the
	// column (e.g. the waiting state).
	if w := lipgloss.Width(summary); w > colWidth {
		colWidth = w
	}
	result := lipgloss.NewStyle().Width(colWidth).Render(summary)

	if elapsed := s.elapsed(); elapsed > 0 {
		result += "\t" + s.style.Aux.Render(formatElapsed(elapsed, false))
	}

	if coverage, ok := s.results.Coverage(); ok {
		// match the same format changes used in the gostd handlers
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("[%0.1f%% coverage]", coverage))
	}

	if s.config.HidePackagesWithNoTests && stats.PackagesWithNoTests > 0 {
		// this lives after the elapsed column (not in the summary column) so a long summary doesn't push the
		// elapsed time out of alignment with the package lines above it.
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("(%s w/o tests)", plural(stats.PackagesWithNoTests, "pkg")))
	}

	return result
}

// progressBarWidth is the fixed width of the package progress bar, in cells. It stays short no matter how many
// packages there are: it shows the split of packages by state, not the state of each package.
const progressBarWidth = 20

// packageCounts is the run's packages by state (see the timing model above). Passed and failed together are the
// packages that are done. Waiting is only known when the run knows its package set up front.
type packageCounts struct {
	passed, failed, running, starting, waiting int
}

func (c packageCounts) done() int {
	return c.passed + c.failed
}

func (c packageCounts) total() int {
	return c.done() + c.running + c.starting + c.waiting
}

func (s GoTestResultSummary) packageCounts() packageCounts {
	var c packageCounts

	seen := strset.New()
	for _, pkgRef := range s.results.Packages() {
		if seen.Has(pkgRef.Package) {
			continue
		}
		seen.Add(pkgRef.Package)
		switch action := s.results.ReferenceConclusiveAction(pkgRef); {
		case action == gotest.FailAction:
			c.failed++
		case action.Completed():
			c.passed++
		}
	}

	for _, pkgRef := range s.inFlightPackages() {
		switch {
		case s.results.ReferenceConclusiveAction(pkgRef).Completed():
			// already counted as done (e.g. a package that died with a test still marked running)
		case s.packageStarting(pkgRef):
			c.starting++
		default:
			c.running++
		}
	}

	if progress := s.results.BuildProgress(); progress.Known() {
		// not started means still building, or built and queued behind an earlier package (see startedGlyph)
		c.waiting = max(progress.Expected-progress.Started, 0)
	}

	return c
}

// packageProgressLine renders the package level of the footer: a stacked bar showing the split of packages by state,
// then how many are done, e.g. "━━━━━━━━━━━━────────  43/89 pkgs completed (10 failed)". It only exists mid-run, so it
// never appears in the final summary.
func (s GoTestResultSummary) packageProgressLine() (string, bool) {
	if !s.config.Running || s.config.Canceled {
		return "", false
	}

	c := s.packageCounts()
	if c.total() == 0 {
		return "", false
	}

	return s.packageBar(c) + "  " + s.packageLegend(c), true
}

// packageBar is a horizontal stacked bar chart of the packages by state: grouped and in phase order (passed, failed,
// running, starting, waiting), never interleaved. Each state takes the color its package rows use: green and red for
// done, the running yellow, and faint for starting and waiting. The two faint states are told apart by line weight,
// waiting being the light line. Without color only the done share can be shown, as heavy line over light.
func (s GoTestResultSummary) packageBar(c packageCounts) string {
	heavy, light := "━", "─"
	segments := []struct {
		n     int
		style lipgloss.Style
		cell  string
	}{
		{c.passed, s.style.Success, heavy},
		{c.failed, s.style.Failed, heavy},
		{c.running, s.style.Running, heavy},
		{c.starting, s.style.Aux, heavy},
		{c.waiting, s.style.Aux, light},
	}

	counts := make([]int, len(segments))
	for i, seg := range segments {
		counts[i] = seg.n
	}

	var sb strings.Builder
	for i, cells := range apportion(counts, progressBarWidth) {
		if cells == 0 {
			continue
		}
		cell := segments[i].cell
		if !s.config.Color {
			cell = light
			if i <= 1 { // passed, failed
				cell = heavy
			}
		}
		sb.WriteString(segments[i].style.Render(strings.Repeat(cell, cells)))
	}
	return sb.String()
}

// packageLegend is the text beside the bar: how many packages are done out of the total, with failures called out,
// e.g. "43/89 pkgs completed (10 failed)". The in-flight states (running, starting, waiting) are left to the bar's
// segments. It says pkgs so the count can't be mistaken for tests.
//
// Now that it leads the footer it is plain text, like the test counts below it. Only the failures are red.
func (s GoTestResultSummary) packageLegend(c packageCounts) string {
	done := fmt.Sprintf("%d/%d pkgs completed", c.done(), c.total())
	if c.failed > 0 {
		done += s.style.Failed.Render(fmt.Sprintf(" (%d failed)", c.failed))
	}
	return done
}

// apportion splits width cells across counts in proportion. Every non-zero count keeps at least one cell, so a lone
// failed package can't round away to nothing. Remaining cells go to the largest remainders so the parts always add
// up to width.
func apportion(counts []int, width int) []int {
	cells := make([]int, len(counts))
	total := 0
	for _, n := range counts {
		total += n
	}
	if total == 0 {
		return cells
	}

	used := 0
	for i, n := range counts {
		if n > 0 {
			cells[i] = max(1, n*width/total)
			used += cells[i]
		}
	}

	// the one cell minimums can overshoot: take back from the biggest parts
	for used > width {
		biggest := 0
		for i := range cells {
			if cells[i] > cells[biggest] {
				biggest = i
			}
		}
		cells[biggest]--
		used--
	}

	// hand out what flooring left over, largest remainder first
	for used < width {
		best, bestRemainder := -1, 0
		for i, n := range counts {
			if n == 0 {
				continue
			}
			if remainder := n*width - cells[i]*total; best == -1 || remainder > bestRemainder {
				best, bestRemainder = i, remainder
			}
		}
		cells[best]++
		used++
	}

	return cells
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// waitingFooter renders a compact footer for the stretch before any test has concluded. There are no stats to
// column-align yet, so padding to the package name width would only leave a wide gap. Returns false once there are
// results to show, or when none are coming (finished or canceled).
//
// hasPackageLines says whether running package lines were drawn above this one, which decides whether the status
// column is padded out to line up with them.
func (s GoTestResultSummary) waitingFooter(hasPackageLines bool) (string, bool) {
	if s.config.Canceled || !s.config.Running || s.results.TestStats().Total() > 0 {
		return "", false
	}

	progress := s.results.BuildProgress()

	var body string
	switch {
	case progress.Building() && progress.Known():
		body = startedGlyph + " started" + s.style.Aux.Render(fmt.Sprintf(" %d/%d pkgs", progress.Started, progress.Expected))
	case progress.Building():
		body = startedGlyph + " waiting for packages to start"
	default:
		body = waitingGlyph + " waiting for test output"
	}

	// only pad out to the status column when there are package lines above to line up with, otherwise the line
	// opens with a wide gap and nothing to align against
	status := s.footerStatusGlyph() + " "
	if hasPackageLines {
		status = s.footerStatus()
	}

	line := status + body

	if elapsed := s.elapsed(); elapsed > 0 {
		line += "  " + s.style.Aux.Render(strings.TrimSpace(formatElapsed(elapsed, false)))
	}

	return line, true
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
	case total == 0 && !asAux:
		// the waiting footer covers the in-progress case, so here the run was finished or canceled without results
		tests = append(tests, s.style.Waiting.Render("(no test results)"))
		testCountSuffix = ""
	case total == 0:
		tests = append(tests, s.style.Waiting.Render("(waiting for test results)"))
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
