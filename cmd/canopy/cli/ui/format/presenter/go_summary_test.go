package presenter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestGoTestResultSummary_Present(t *testing.T) {
	cases := []struct {
		name      string
		fixture   string
		presenter GoTestResultSummary
	}{
		{
			name:    "failing package",
			fixture: "mixed-verbose.json",
			presenter: GoTestResultSummary{
				config: GoSummaryConfig{
					Color:              false,
					WriteToStderr:      true,
					PackageNameWidth:   100,
					DurationFromEvents: true,
				},
				style: style.NewGo(false),
			},
		},
		{
			name:    "passing package",
			fixture: "mixed-verbose.json",
			presenter: GoTestResultSummary{
				config: GoSummaryConfig{
					Color:              false,
					WriteToStderr:      true,
					PackageNameWidth:   100,
					DurationFromEvents: true,
				},
				style: style.NewGo(false),
			},
		},
		{
			name:    "panic package",
			fixture: "panic-verbose.json",
			presenter: GoTestResultSummary{
				config: GoSummaryConfig{
					Color:              false,
					WriteToStderr:      true,
					PackageNameWidth:   100,
					DurationFromEvents: true,
				},
				style: style.NewGo(false),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}

			subject := tt.presenter
			subject.results = newJoinedResults(*fixtureRun(t, tt.fixture))

			err := subject.Present(&sb, &sb)
			require.NoError(t, err)

			snaps.MatchSnapshot(t, sb.String())
		})
	}

}

func TestGoTestResultSummary_Canceled(t *testing.T) {
	// a canceled run must report CANCELED, never a false PASS, even when every concluded test passed
	subject := GoTestResultSummary{
		config: GoSummaryConfig{
			Color:              false,
			WriteToStderr:      true,
			PackageNameWidth:   100,
			DurationFromEvents: true,
			Canceled:           true,
		},
		style: style.NewGo(false),
	}
	subject.results = newJoinedResults(*fixtureRun(t, "mixed-verbose.json"))

	sb := strings.Builder{}
	require.NoError(t, subject.Present(&sb, &sb))

	require.Contains(t, sb.String(), style.CanceledGlyph)
	require.Contains(t, sb.String(), "canceled by user")
	require.NotContains(t, sb.String(), "PASS")
}

func TestGoTestResultSummary_Extras(t *testing.T) {
	// extras trail the elapsed time rather than living in the summary column, otherwise a long summary knocks the
	// elapsed time out of alignment with the package lines above. Package progress only exists mid-run, so it gets its own
	// line under the stats column instead of widening the summary line.
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkgs := golist.NewPackageCollection(
		golist.Package{ImportPath: "example.com/a", Dir: "/a"},
		golist.Package{ImportPath: "example.com/b", Dir: "/b"},
		golist.Package{ImportPath: "example.com/c", Dir: "/c"},
	)
	a := gotest.NewReference("example.com/a", "")
	testA := gotest.NewReference("example.com/a", "TestA")
	b := gotest.NewReference("example.com/b", "")

	subject := newWaitingSubject(pkgs, []gotest.Event{
		{Reference: a, Action: gotest.StartAction, Time: started.Add(time.Second)},
		{Reference: testA, Action: gotest.RunAction, Time: started.Add(4 * time.Second)},
		{Reference: testA, Action: gotest.PassAction, Time: started.Add(5 * time.Second)},
		{Reference: a, Action: gotest.PassAction, Time: started.Add(5 * time.Second)},
		{Reference: b, Action: gotest.StartAction, Time: started.Add(2 * time.Second)},
		{Reference: b, Action: gotest.OutputAction, Output: "?   \texample.com/b\t[no test files]\n", Annotations: []gotest.Annotation{gotest.NoTestFiles}, Time: started.Add(2 * time.Second)},
		{Reference: b, Action: gotest.SkipAction, Time: started.Add(2 * time.Second)},
	}, false)
	subject.config.StartedAt = started
	subject.config.EndedAt = started.Add(6 * time.Second)
	subject.config.HidePackagesWithNoTests = true

	require.Equal(t,
		// two of three packages done fills 13 of the 20 cells
		"⠋\t\t1 passed tests                          \t6s   \t(1 pkg w/o tests)\n\t\t└─ ━━━━━━━━━━━━━───────  2/3 pkgs done",
		subject.summaryFooter(),
	)
}

func TestGoTestResultSummary_PackageProgressLine(t *testing.T) {
	// the rows are live (wall clock), so anchor events in the recent past
	now := time.Now().Add(-5 * time.Second)
	ev := func(pkg, test string, action gotest.Action) gotest.Event {
		return gotest.Event{Reference: gotest.NewReference(pkg, test), Action: action, Time: now}
	}

	pkgs := golist.NewPackageCollection(
		golist.Package{ImportPath: "example.com/failed", Dir: "/failed"},
		golist.Package{ImportPath: "example.com/passed", Dir: "/passed"},
		golist.Package{ImportPath: "example.com/running", Dir: "/running"},
		golist.Package{ImportPath: "example.com/starting", Dir: "/starting"},
		golist.Package{ImportPath: "example.com/waiting", Dir: "/waiting"},
	)
	events := []gotest.Event{
		ev("example.com/failed", "", gotest.StartAction),
		ev("example.com/failed", "TestA", gotest.RunAction),
		ev("example.com/failed", "TestA", gotest.FailAction),
		ev("example.com/failed", "", gotest.FailAction),
		ev("example.com/passed", "", gotest.StartAction),
		ev("example.com/passed", "TestA", gotest.RunAction),
		ev("example.com/passed", "TestA", gotest.PassAction),
		ev("example.com/passed", "", gotest.PassAction),
		ev("example.com/running", "", gotest.StartAction),
		ev("example.com/running", "TestA", gotest.RunAction),
		ev("example.com/starting", "", gotest.StartAction),
	}

	t.Run("names packages and calls out failures", func(t *testing.T) {
		line, ok := newWaitingSubject(pkgs, events, false).packageProgressLine()
		require.True(t, ok)
		// without color only the done share can show: heavy for passed and failed, light for the rest
		require.Equal(t, "└─ ━━━━━━━━────────────  2/5 pkgs done (1 failed)", line)
	})

	t.Run("with color every state but waiting is the heavy line, grouped in phase order", func(t *testing.T) {
		subject := newWaitingSubject(pkgs, events, false)
		subject.config.Color = true
		// passed, failed, running and starting (4 cells each) are told apart by color, waiting by line weight
		require.Equal(t, strings.Repeat("━", 16)+strings.Repeat("─", 4), subject.packageBar(subject.packageCounts()))
	})

	t.Run("without a known package set the total is what has been seen", func(t *testing.T) {
		line, ok := newWaitingSubject(nil, events, false).packageProgressLine()
		require.True(t, ok)
		require.True(t, strings.HasSuffix(line, "  2/4 pkgs done (1 failed)"), "got %q", line)
	})

	t.Run("in-flight states are left to the bar", func(t *testing.T) {
		subject := newWaitingSubject(pkgs, events, false)
		require.Equal(t, "10/50 pkgs done", subject.packageLegend(packageCounts{passed: 10, running: 5, starting: 5, waiting: 30}))
	})

	t.Run("gone once the run ends or is canceled", func(t *testing.T) {
		_, ok := newWaitingSubject(pkgs, events, true).packageProgressLine()
		require.False(t, ok)

		canceled := newWaitingSubject(pkgs, events, false)
		canceled.config.Canceled = true
		_, ok = canceled.packageProgressLine()
		require.False(t, ok)
	})
}

func TestApportion(t *testing.T) {
	cases := []struct {
		name   string
		counts []int
		want   []int
	}{
		{name: "even split", counts: []int{1, 1, 1, 1, 0}, want: []int{5, 5, 5, 5, 0}},
		{name: "leftovers go to the largest remainders", counts: []int{2, 0, 0, 0, 1}, want: []int{13, 0, 0, 0, 7}},
		{name: "a lone failure keeps a cell", counts: []int{58, 1, 0, 0, 0}, want: []int{19, 1, 0, 0, 0}},
		{name: "minimums taken back from the biggest part", counts: []int{1000, 1, 1, 1, 1}, want: []int{16, 1, 1, 1, 1}},
		{name: "nothing to show", counts: []int{0, 0, 0, 0, 0}, want: []int{0, 0, 0, 0, 0}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := apportion(tt.counts, progressBarWidth)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGoTestResultSummary_WaitingFooter(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkgs := golist.NewPackageCollection(
		golist.Package{ImportPath: "example.com/a", Dir: "/a"},
		golist.Package{ImportPath: "example.com/b", Dir: "/b"},
		golist.Package{ImportPath: "example.com/c", Dir: "/c"},
	)

	start := func(pkg string, offset time.Duration) gotest.Event {
		return gotest.Event{Reference: gotest.NewReference(pkg, ""), Action: gotest.StartAction, Time: base.Add(offset)}
	}

	cases := []struct {
		name   string
		pkgs   *golist.PackageCollection
		events []gotest.Event
		want   string
	}{
		{
			name: "nothing started yet",
			pkgs: pkgs,
			want: "⠋ ⛭ started 0/3 pkgs\n",
		},
		{
			name:   "partially started",
			pkgs:   pkgs,
			events: []gotest.Event{start("example.com/a", 0), start("example.com/b", 1500*time.Millisecond)},
			want:   "⠋ ⛭ started 2/3 pkgs  1.5s\n",
		},
		{
			name: "all started, waiting for test output",
			pkgs: pkgs,
			events: []gotest.Event{
				start("example.com/a", 0),
				start("example.com/b", time.Second),
				start("example.com/c", 2*time.Second),
				{Reference: gotest.NewReference("example.com/a", "TestA"), Action: gotest.RunAction, Time: base.Add(3 * time.Second)},
			},
			want: "⠋ ⧖ waiting for test output  3s\n",
		},
		{
			name: "unknown package set",
			want: "⠋ ⛭ waiting for packages to start\n",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			subject := newWaitingSubject(tt.pkgs, tt.events, false)

			sb := strings.Builder{}
			require.NoError(t, subject.Present(&sb, &sb))
			require.Equal(t, tt.want, sb.String())
		})
	}
}

func TestGoTestResultSummary_WaitingFooterAlignment(t *testing.T) {
	// the status column is only worth padding out when there are running package lines above to line up with
	subject := newWaitingSubject(golist.NewPackageCollection(golist.Package{ImportPath: "example.com/a", Dir: "/a"}), nil, false)

	line, ok := subject.waitingFooter(true)
	require.True(t, ok)
	require.Equal(t, "⠋\t\t⛭ started 0/1 pkgs", line)

	line, ok = subject.waitingFooter(false)
	require.True(t, ok)
	require.Equal(t, "⠋ ⛭ started 0/1 pkgs", line)
}

func TestGoTestResultSummary_WaitingFooterStepsAside(t *testing.T) {
	pkg := gotest.NewReference("example.com/a", "")
	test := gotest.NewReference("example.com/a", "TestA")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkgs := golist.NewPackageCollection(golist.Package{ImportPath: "example.com/a", Dir: "/a"})

	t.Run("finished without results", func(t *testing.T) {
		subject := newWaitingSubject(pkgs, []gotest.Event{{Reference: pkg, Action: gotest.StartAction, Time: now}}, true)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		require.Contains(t, sb.String(), "(no test results)")
		require.NotContains(t, sb.String(), "started")
		require.NotContains(t, sb.String(), "waiting for test output")
	})

	t.Run("test results arrived", func(t *testing.T) {
		subject := newWaitingSubject(pkgs, []gotest.Event{
			{Reference: pkg, Action: gotest.StartAction, Time: now},
			{Reference: test, Action: gotest.RunAction, Time: now},
			{Reference: test, Action: gotest.PassAction, Time: now.Add(time.Second)},
		}, false)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		require.Contains(t, sb.String(), "1 passed tests")
		require.NotContains(t, sb.String(), "started")
		require.NotContains(t, sb.String(), "waiting for test output")
	})

	t.Run("results arrive while packages are still starting", func(t *testing.T) {
		morePkgs := golist.NewPackageCollection(
			golist.Package{ImportPath: "example.com/a", Dir: "/a"},
			golist.Package{ImportPath: "example.com/b", Dir: "/b"},
			golist.Package{ImportPath: "example.com/c", Dir: "/c"},
		)
		subject := newWaitingSubject(morePkgs, []gotest.Event{
			{Reference: pkg, Action: gotest.StartAction, Time: now},
			{Reference: test, Action: gotest.RunAction, Time: now},
			{Reference: test, Action: gotest.PassAction, Time: now.Add(time.Second)},
		}, false)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		require.Contains(t, sb.String(), "1 passed tests")
		// package progress stays visible after tests report, otherwise settled counts read as a run that is nearly done
		require.Contains(t, sb.String(), "\n\t\t└─ ────────────────────  0/3 pkgs done")
		// every result seen so far passed, but the run isn't done while packages haven't started
		require.True(t, strings.HasPrefix(sb.String(), "⠋"), "expected a running status, got %q", sb.String())
	})

	t.Run("finished while packages never started", func(t *testing.T) {
		morePkgs := golist.NewPackageCollection(
			golist.Package{ImportPath: "example.com/a", Dir: "/a"},
			golist.Package{ImportPath: "example.com/b", Dir: "/b"},
		)
		subject := newWaitingSubject(morePkgs, []gotest.Event{
			{Reference: pkg, Action: gotest.StartAction, Time: now},
			{Reference: test, Action: gotest.RunAction, Time: now},
			{Reference: test, Action: gotest.PassAction, Time: now.Add(time.Second)},
		}, true)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		// package progress is a mid-run line, never part of the final summary
		require.NotContains(t, sb.String(), "└─")
	})
}

func TestGoTestResultSummary_PackageRowStates(t *testing.T) {
	// the rows are live (wall clock), so anchor events in the recent past to clear the one second row filter
	now := time.Now().Add(-5 * time.Second)
	ev := func(pkg, test string, action gotest.Action) gotest.Event {
		return gotest.Event{Reference: gotest.NewReference(pkg, test), Action: action, Time: now}
	}

	run := gotest.NewRun(gotest.RunnerConfig{})
	run.Result = *gotest.NewResult(gotest.ResultConfig{})
	for _, e := range []gotest.Event{
		// launching: nothing from the test binary yet
		ev("example.com/launching", "", gotest.StartAction),
		ev("example.com/zlaunching", "", gotest.StartAction),
		// running a test
		ev("example.com/running", "", gotest.StartAction),
		ev("example.com/running", "TestA", gotest.RunAction),
		// between tests, e.g. TestMain wrote output before any test began
		ev("example.com/setup", "", gotest.StartAction),
		{Reference: gotest.NewReference("example.com/setup", ""), Action: gotest.OutputAction, Output: "setting up\n", Time: now},
		// done, so no row
		ev("example.com/done", "", gotest.StartAction),
		ev("example.com/done", "", gotest.PassAction),
	} {
		run.Result.Update(e)
	}

	subject := DefaultGoTestResultSummaryConfig().
		WithColor(false).
		WithPackageNameWidth(30).
		WithRunningState("⠋").
		New(*run).(GoTestResultSummary)
	subject.config.Running = true

	rows := subject.runningRows()
	require.Len(t, rows, 4)
	// each starting package keeps its own row: like running packages they hold back the body's output, so a shared
	// count would leave the unrendered rollup with no visible cause
	for i, pkg := range []string{"example.com/launching", "example.com/zlaunching"} {
		row := rows[[]int{0, 3}[i]]
		require.True(t, strings.HasPrefix(row, "⧖"), "launching package should use the waiting glyph: %q", row)
		require.Contains(t, row, pkg)
		require.Contains(t, row, "(starting)")
		// the starting phase counts toward the package's time, the same baseline go test uses for its "ok" line
		require.Contains(t, row, "5s")
	}
	require.True(t, strings.HasPrefix(rows[1], "⠋"), "running package should use the spinner: %q", rows[1])
	require.Contains(t, rows[1], "example.com/running")
	require.True(t, strings.HasPrefix(rows[2], "⠋"), "package past launch should use the spinner: %q", rows[2])
	require.Contains(t, rows[2], "example.com/setup")
	require.NotContains(t, rows[2], "(starting)")
	// the setup package has two events, its timer must keep counting rather than stop at the last one
	require.Contains(t, rows[2], "5s")
}

func TestGoTestResultSummary_UnrenderedRowWithoutResults(t *testing.T) {
	// completed packages held back behind a starting one, none of which had tests: the rollup must not claim to be
	// waiting for results that are never coming
	// old enough to clear the one second row filter, young enough to not be stale (which would skip the rollup)
	now := time.Now().Add(-1500 * time.Millisecond)
	run := gotest.NewRun(gotest.RunnerConfig{})
	run.Result = *gotest.NewResult(gotest.ResultConfig{})
	for _, e := range []gotest.Event{
		{Reference: gotest.NewReference("example.com/a", ""), Action: gotest.StartAction, Time: now},
		{Reference: gotest.NewReference("example.com/b", ""), Action: gotest.StartAction, Time: now},
		{Reference: gotest.NewReference("example.com/b", ""), Action: gotest.SkipAction, Time: now},
	} {
		run.Result.Update(e)
	}

	subject := DefaultGoTestResultSummaryConfig().
		WithColor(false).
		WithPackageNameWidth(30).
		WithRunningState("⠋").
		New(*run).(GoTestResultSummary)
	subject.config.Running = true

	rows := subject.runningRows()
	require.Len(t, rows, 2)
	require.Contains(t, rows[0], "(1 unrendered pkgs)")
	require.NotContains(t, rows[0], "waiting for test results")
}

func TestGoTestResultSummary_WallClockFooter(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkg := gotest.NewReference("example.com/a", "")
	test := gotest.NewReference("example.com/a", "TestA")
	events := []gotest.Event{
		{Reference: pkg, Action: gotest.StartAction, Time: started.Add(6 * time.Second)},
		{Reference: test, Action: gotest.RunAction, Time: started.Add(7 * time.Second)},
		{Reference: test, Action: gotest.PassAction, Time: started.Add(8 * time.Second)},
	}

	t.Run("waiting before any result counts from launch", func(t *testing.T) {
		subject := newWaitingSubject(golist.NewPackageCollection(golist.Package{ImportPath: "example.com/a", Dir: "/a"}), nil, false)
		subject.config.StartedAt = started
		subject.config.EndedAt = started.Add(2500 * time.Millisecond)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		require.Equal(t, "⠋ ⛭ started 0/1 pkgs  2.5s\n", sb.String())
	})

	t.Run("results count from launch, not the first event", func(t *testing.T) {
		subject := newWaitingSubject(nil, events, true)
		subject.config.StartedAt = started
		subject.config.EndedAt = started.Add(9 * time.Second)

		sb := strings.Builder{}
		require.NoError(t, subject.Present(&sb, &sb))
		require.Contains(t, sb.String(), "9s")
		// a one-time fact, noted on the first package result line instead
		require.NotContains(t, sb.String(), "first test")
	})
}

func newWaitingSubject(pkgs *golist.PackageCollection, events []gotest.Event, finished bool) GoTestResultSummary {
	run := gotest.NewRun(gotest.RunnerConfig{Packages: pkgs})
	run.Result = *gotest.NewResult(gotest.ResultConfig{})
	for _, e := range events {
		run.Result.Update(e)
	}

	return GoSummaryConfig{
		Color:              false,
		PackageNameWidth:   40,
		DurationFromEvents: true,
		RunningState:       "⠋",
		Running:            !finished,
	}.New(*run).(GoTestResultSummary)
}

func TestElapsedPlaceholderWidth(t *testing.T) {
	// the unrendered-packages rollup line has no elapsed time, but its placeholder must still occupy the same
	// number of columns as a rendered elapsed value, otherwise the trailing tab lands on a different tab stop
	// and the stats column is offset from the running-package lines.
	require.Equal(t, lipgloss.Width(formatElapsed(time.Second, true)), lipgloss.Width(elapsedPlaceholder))
}

func fixtureRun(t testing.TB, name string) *gotest.Run {
	fh, err := os.Open(filepath.Join("testdata", name))
	require.NoError(t, err)

	return gotest.ReplayRun(fh, gotest.RunnerConfig{}, gotest.ResultConfig{
		TrackOtherOutput:   true,
		TrackFailingOutput: true,
	}, nil)
}
