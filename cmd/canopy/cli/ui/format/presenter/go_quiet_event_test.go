package presenter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
)

func TestParseAndFormatPackageLine_ElapsedPrecision(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{name: "ok rounds to two decimals", line: "ok  \tex.com/a\t2.542s\n", want: "ex.com/a\t 2.54s  \n"},
		{name: "fail rounds to two decimals", line: "FAIL\tex.com/a\t7.836s\n", want: "ex.com/a\t 7.84s  \n"},
		{name: "a note after the time is kept", line: "ok  \tex.com/a\t0.010s [no tests to run]\n", want: "ex.com/a\t 0.01s [no tests to run]\n"},
		{name: "cached has no time to round", line: "ok  \tex.com/a\t(cached)\n", want: "ex.com/a\t(cached)\n"},
		{name: "only the elapsed field is touched", line: "ok  \tex.com/a\t1.000s\ttests 0.90s\n", want: "ex.com/a\t 1.00s  \t[tests 0.90s]\n"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormatPackageLine(tt.line, style.NewGo(false), 0, "")
			require.True(t, strings.HasSuffix(got, tt.want), "got %q, want suffix %q", got, tt.want)
		})
	}
}
