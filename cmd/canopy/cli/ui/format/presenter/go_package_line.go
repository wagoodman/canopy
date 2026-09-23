package presenter

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
)

// statusColumn pads a status indicator out to the width of the status column, so whatever follows it lands on
// the same tab stop on every line. Package rows and the summary footer both depend on this one rule to line up.
func statusColumn(status string) string {
	switch width := lipgloss.Width(status); {
	case width == 0:
		return "\t\t"
	case width < 4:
		return status + "\t\t"
	case width < 8:
		return status + "\t"
	}
	return status
}

type Package struct {
	Status         string
	Name           string
	NameAsAux      bool
	TestsCompleted int
	Aux            []string
	Trailer        string
	Style          style.Go
	FormatStatus   bool
	MaxTestName    int
	StripPrefix    string
}

func (p Package) Present(stdout, _ io.Writer) error {
	if _, err := fmt.Fprint(stdout, p.String()); err != nil {
		return fmt.Errorf("failed to write go test package output to stdout: %w", err)
	}
	return nil
}

// func FormatPackageLine(status, pkgName string, testsCompleted int, aux []string, trailer string, st style.Go, formatStatus bool, maxTestName int) string {
func (p Package) String() string {
	var status = statusColumn(p.Status)

	var aux = p.Aux
	if p.FormatStatus {
		switch {
		case output.HasPackagePassMarking(status):
			status = p.Style.Success.Render(status)
		case output.HasPackageOKMarking(status):
			status = p.Style.Success.Render(status)
		case output.HasUnknownPackageMarking(status):
			status = p.Style.Aux.Render(status)
		case output.HasFailedPackageMarking(status):
			status = p.Style.Failed.Render(status)
		case output.HasFailedPackageTrailer(status):
			status = p.Style.Failed.Render(status)
		}
	} else if p.TestsCompleted > 0 {
		runStr := fmt.Sprintf("%d tests", p.TestsCompleted)
		aux = append(aux, runStr)
	}

	if p.Name != "" {
		if p.StripPrefix != "" {
			p.Name = stripPackagePrefix(p.Name, p.StripPrefix)
		}

		// make all test names the same width
		p.Name = fmt.Sprintf("%-*s", p.MaxTestName, p.Name)
	}

	if p.NameAsAux {
		p.Name = p.Style.Aux.Render(p.Name)
	}

	for i, a := range aux {
		switch {
		case a == "" || output.IsWhitespace(a):
			// allow whitespace to occur...
			break

		case output.HasTimeMarker(a), i == 0 && packageElapsedPattern.MatchString(strings.TrimSpace(a)):
			// elapsed, possibly with a startup mark after it ("6.20s ◕")
			break

		case strings.ContainsAny(a, "(["):
			// TODO: why!?
			// already formatted
			break

		case output.HasPackageCoverageMarking(a):
			// on nearly every line, so no brackets
			a = formatCoverage(a)

		default:
			a = "[" + a + "]"
		}

		aux[i] = p.Style.Aux.Render(a)
	}

	return status + strings.Join(append([]string{p.Name}, aux...), "\t") + p.Trailer
}

// coveragePercentPattern pulls the percentage out of go's "coverage: 61.2% of statements [in <pattern>]".
var coveragePercentPattern = regexp.MustCompile(`^coverage: (\d+(?:\.\d+)?)% of statements`)

// formatCoverage rewrites go's coverage field as "61.2% covered". It isn't padded, so it starts at the column's edge
// the same as the live rows' stats do in that column. With -coverpkg go also appends " in <pattern>", which is the
// same on every line, so it is dropped.
func formatCoverage(a string) string {
	m := coveragePercentPattern.FindStringSubmatch(strings.TrimSpace(a))
	if m == nil {
		return a
	}
	pct, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return a
	}
	return fmt.Sprintf("%.1f%% covered", pct)
}

// stripPackagePrefix makes a package path relative to prefix (usually the module path). The prefix itself becomes "."
// rather than nothing, and only whole path segments are stripped, so a sibling like "<prefix>-extra" is left alone.
func stripPackagePrefix(name, prefix string) string {
	if name == prefix {
		return "."
	}
	if rest, ok := strings.CutPrefix(name, strings.TrimSuffix(prefix, "/")+"/"); ok {
		return rest
	}
	return name
}
