package presenter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			subject.runs = []gotest.Run{*fixtureRun(t, tt.fixture)}
			subject.results = newJoinedResults(subject.runs...)

			err := subject.Present(&sb, &sb)
			require.NoError(t, err)

			snaps.MatchSnapshot(t, sb.String())
		})
	}

}

func fixtureRun(t testing.TB, name string) *gotest.Run {
	fh, err := os.Open(filepath.Join("testdata", name))
	require.NoError(t, err)

	return gotest.ReplayRun(fh, gotest.RunnerConfig{}, gotest.ResultConfig{
		TrackOtherOutput:   true,
		TrackFailingOutput: true,
	}, nil)
}
