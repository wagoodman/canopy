package presenter

import (
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
)

func TestJestTestResultSummary_Present(t *testing.T) {
	cases := []struct {
		name      string
		fixture   string
		presenter JestTestResultSummary
	}{
		{
			name:    "failing package",
			fixture: "mixed-verbose.json",
			presenter: JestTestResultSummary{
				config: JestTestResultSummaryConfig{
					Color:         false,
					WriteToStderr: true,
					ShowElapsed:   true,
				},
				style: newJestStyle(false),
			},
		},
		{
			name:    "passing package",
			fixture: "mixed-verbose.json",
			presenter: JestTestResultSummary{
				config: JestTestResultSummaryConfig{
					Color:         false,
					WriteToStderr: true,
					ShowElapsed:   true,
				},
				style: newJestStyle(false),
			},
		},
		{
			name:    "panic package",
			fixture: "panic-verbose.json",
			presenter: JestTestResultSummary{
				config: JestTestResultSummaryConfig{
					Color:         false,
					WriteToStderr: true,
					ShowElapsed:   true,
				},
				style: newJestStyle(false),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}

			subject := tt.presenter
			subject.run = *fixtureRun(t, tt.fixture)

			err := subject.Present(&sb, &sb)
			require.NoError(t, err)

			snaps.MatchSnapshot(t, sb.String())
		})
	}

}
