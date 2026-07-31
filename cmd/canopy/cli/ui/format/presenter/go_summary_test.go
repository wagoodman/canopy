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

	require.Contains(t, sb.String(), canceledGlyph)
	require.Contains(t, sb.String(), "canceled by user")
	require.NotContains(t, sb.String(), "PASS")
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
