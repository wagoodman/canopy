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

func TestGoTestResultSummary_PackagesWithNoTests(t *testing.T) {
	// the no-tests count must come after the elapsed time rather than inside the summary column, otherwise a long
	// summary pushes the elapsed time out of alignment with the package lines above it.
	subject := GoTestResultSummary{
		config: GoSummaryConfig{
			Color:                   false,
			PackageNameWidth:        100,
			DurationFromEvents:      true,
			HidePackagesWithNoTests: true,
		},
		style: style.NewGo(false),
	}
	subject.results = newJoinedResults(*fixtureRun(t, "mixed-verbose.json"))

	sb := strings.Builder{}
	require.NoError(t, subject.Present(&sb, &sb))
	out := sb.String()

	elapsed := formatElapsed(subject.results.Elapsed(false), false)
	noTests := "with no tests)"
	require.Contains(t, out, elapsed)
	require.Contains(t, out, noTests)
	require.Less(t, strings.Index(out, elapsed), strings.Index(out, noTests))
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
			name: "nothing compiled yet",
			pkgs: pkgs,
			want: "⠋\t\t⛭ compiling 0/3 packages\n",
		},
		{
			name:   "partially compiled",
			pkgs:   pkgs,
			events: []gotest.Event{start("example.com/a", 0), start("example.com/b", 1500*time.Millisecond)},
			want:   "⠋\t\t⛭ compiling 2/3 packages  1.5s\n",
		},
		{
			name: "compiled, waiting for test output",
			pkgs: pkgs,
			events: []gotest.Event{
				start("example.com/a", 0),
				start("example.com/b", time.Second),
				start("example.com/c", 2*time.Second),
				{Reference: gotest.NewReference("example.com/a", "TestA"), Action: gotest.RunAction, Time: base.Add(3 * time.Second)},
			},
			want: "⠋\t\t⧖ waiting for test output  3s\n",
		},
		{
			name: "unknown package set",
			want: "⠋\t\t⛭ compiling\n",
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
		require.NotContains(t, sb.String(), "compiling")
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
		require.NotContains(t, sb.String(), "compiling")
		require.NotContains(t, sb.String(), "waiting for test output")
	})

	t.Run("results arrive while packages still compile", func(t *testing.T) {
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
		// once tests are reporting, the footer sticks to test status (build progress is only for the wait before)
		require.NotContains(t, sb.String(), "compiling")
		// every result seen so far passed, but the run isn't done while packages are still compiling
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
		require.NotContains(t, sb.String(), "compiling")
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
