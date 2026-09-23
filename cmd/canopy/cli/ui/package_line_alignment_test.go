package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var (
	renderedElapsed  = regexp.MustCompile(`\d+\.\d\ds`)
	renderedCoverage = regexp.MustCompile(`\d+\.\d% covered`)
)

// the TUI expands tabs itself (stops every tabWidth columns), so the columns of package result lines have to line up
// after that expansion: the start of coverage on every line, and the end of each time whether or not it has a
// startup mark.
func TestPackageLines_ColumnsAligned(t *testing.T) {
	lines := []string{
		"ok  \tgithub.com/anchore/syft/cmd/syft\t0.230s\tcoverage: 66.7% of statements in ./...\n",
		"\tgithub.com/anchore/syft/cmd/syft\t\tcoverage: 0.0% of statements\n",
		"ok  \tgithub.com/anchore/syft/cmd/syft\t6.200s ◕\tcoverage: 11.8% of statements in ./...\n",
		"ok  \tgithub.com/anchore/syft/cmd/syft\t17.760s ◔\tcoverage: 5.4% of statements in ./...\t(started after 5.66s)\n",
		"ok  \tgithub.com/anchore/syft/cmd/syft\t17.760s\tcoverage: 100.0% of statements in ./...\n",
		"ok  \tgithub.com/anchore/syft/cmd/syft\t(cached)\tcoverage: 2.3% of statements in ./...\n",
		"FAIL\tgithub.com/anchore/syft/cmd/syft\t1.100s ●\tcoverage: 2.3% of statements in ./...\n",
	}

	factory := presenter.NewGoQuietEventFactory(presenter.GoEventConfig{Style: style.NewGo(false), PackageNameWidth: 40})
	var coverage, timeEnds []int
	for _, l := range lines {
		e := gotest.Event{Reference: gotest.NewReference("github.com/anchore/syft/cmd/syft", ""), Action: gotest.OutputAction, Output: l}
		rendered := []rune(expandTabs(factory.NewEvent(e, false).String()))
		t.Log(strings.TrimRight(string(rendered), "\n"))

		// columns in runes, since the startup marks are multi-byte
		if loc := renderedCoverage.FindStringIndex(string(rendered)); loc != nil {
			coverage = append(coverage, len([]rune(string(rendered)[:loc[0]])))
		}
		if loc := renderedElapsed.FindStringIndex(string(rendered)); loc != nil {
			timeEnds = append(timeEnds, len([]rune(string(rendered)[:loc[1]])))
		}
	}
	for i := range coverage {
		assert.Equal(t, coverage[0], coverage[i], "coverage column of line %d", i)
	}
	for i := range timeEnds {
		assert.Equal(t, timeEnds[0], timeEnds[i], "time end column of timed line %d", i)
	}
}
