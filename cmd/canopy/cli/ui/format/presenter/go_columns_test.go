package presenter

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// expandTabs4 expands tabs at stops every 4 columns, the same as the TUI does before drawing (see ui.expandTabs).
func expandTabs4(s string) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := 4 - col%4
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

var columnTime = regexp.MustCompile(`\d+(\.\d+)?m?s\b`)

// the TUI shows static package result lines above live rows (running, starting, unrendered) and the summary footer.
// They are built in different places, so this checks that after tab expansion the elapsed time ends in the same
// column on all of them, and the column after it (coverage or test stats) starts in the same place.
func TestPackageColumns_StaticAndLiveRowsAligned(t *testing.T) {
	const nameWidth = 30
	// old enough to clear the one second row filter, young enough for the unrendered rollup to show
	now := time.Now().Add(-1500 * time.Millisecond)
	ev := func(pkg, test string, action gotest.Action) gotest.Event {
		return gotest.Event{Reference: gotest.NewReference(pkg, test), Action: action, Time: now}
	}

	run := gotest.NewRun(gotest.RunnerConfig{})
	run.Result = *gotest.NewResult(gotest.ResultConfig{})
	for _, e := range []gotest.Event{
		ev("example.com/running", "", gotest.StartAction),
		ev("example.com/running", "TestA", gotest.RunAction),
		ev("example.com/running", "TestB", gotest.RunAction),
		ev("example.com/running", "TestB", gotest.PassAction),
		ev("example.com/starting", "", gotest.StartAction),
		ev("example.com/zdone", "", gotest.StartAction),
		ev("example.com/zdone", "TestC", gotest.RunAction),
		ev("example.com/zdone", "TestC", gotest.PassAction),
		ev("example.com/zdone", "", gotest.PassAction),
	} {
		run.Result.Update(e)
	}

	subject := DefaultGoTestResultSummaryConfig().
		WithColor(false).
		WithPackageNameWidth(nameWidth).
		WithRunningState("⠋").
		New(*run).(GoTestResultSummary)
	subject.config.Running = true

	var lines []string
	for _, l := range []string{
		"ok  \texample.com/a\t0.740s\tcoverage: 14.1% of statements\n",
		"ok  \texample.com/b\t17.760s ●\tcoverage: 12.4% of statements\n",
		"ok  \texample.com/c\t(cached)\tcoverage: 2.3% of statements\n",
		"\texample.com/d\t\tcoverage: 0.0% of statements\n",
	} {
		lines = append(lines, parseAndFormatPackageLine(l, style.NewGo(false), nameWidth, ""))
	}
	rows := subject.runningRows()
	require.Len(t, rows, 3, "running, starting, and unrendered rows")
	lines = append(lines, rows...)
	for _, l := range strings.Split(subject.summaryFooter(), "\n") {
		if columnTime.MatchString(l) {
			lines = append(lines, l)
		}
	}

	var rendered [][]rune
	var timeEnds []int
	for _, l := range lines {
		r := []rune(strings.TrimRight(expandTabs4(l), "\n"))
		rendered = append(rendered, r)
		t.Log(string(r))
		if loc := columnTime.FindStringIndex(string(r)); loc != nil {
			timeEnds = append(timeEnds, len([]rune(string(r)[:loc[1]])))
		}
	}
	require.NotEmpty(t, timeEnds)

	// what the reader sees as the start of the column after elapsed (coverage, or the live rows' stats) is its first
	// visible character past the elapsed column's mark slot, so padding inside the field can't hide a misalignment
	var nextStarts []int
	for _, r := range rendered {
		for i := timeEnds[0] + 2; i < len(r); i++ {
			if r[i] != ' ' {
				nextStarts = append(nextStarts, i)
				break
			}
		}
	}

	for i := range timeEnds {
		assert.Equal(t, timeEnds[0], timeEnds[i], "time end column of timed line %d", i)
	}
	for i := range nextStarts {
		assert.Equal(t, nextStarts[0], nextStarts[i], "next column start of line %d", i)
	}
}
