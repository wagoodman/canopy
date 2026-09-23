package presenter

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
)

// packageElapsedPattern matches the elapsed time go test puts at the start of the third field of a package's
// "ok"/"FAIL" line, e.g. "2.542s", including when a note follows it ("0.010s [no tests to run]").
var packageElapsedPattern = regexp.MustCompile(`^\d+\.\d+s\b`)

// elapsedTimeWidth fits an elapsed time up to "99.99s", and elapsedColumnWidth that plus a startup mark (" ◕").
// ponytail: 100s+ packages push the columns after it over on that line only.
const (
	elapsedTimeWidth   = 6
	elapsedColumnWidth = elapsedTimeWidth + 2
)

type GoQuietEventFactory struct {
	config GoEventConfig
}

func NewGoQuietEventFactory(cfg GoEventConfig) GoQuietEventFactory {
	return GoQuietEventFactory{
		config: cfg,
	}
}

func (f GoQuietEventFactory) NewEvent(e gotest.Event, midPanic bool) fmt.Stringer {
	return goQuietEvent{
		GoEventConfig: f.config,
		Event:         e,
		Panic:         midPanic,
	}
}

type goQuietEvent struct {
	GoEventConfig
	Event gotest.Event
	Panic bool
}

func (p goQuietEvent) Present(stdout, _ io.Writer) error {
	if _, err := fmt.Fprint(stdout, p.String()); err != nil {
		return fmt.Errorf("failed to write go test event output to stdout: %w", err)
	}
	return nil
}

func (p goQuietEvent) String() string {
	e := p.Event
	if e.Reference.IsPackage() {
		return p.formatPackage(e)
	}

	// indent
	return strings.Repeat("    ", strings.Count(e.Reference.TestName(false), "/")) + p.format(e)
}

func (p goQuietEvent) formatPackage(e gotest.Event) string {
	if output.HasAny(output.HasFailedPackageMarking, output.HasPackageOKMarking, output.HasUnknownPackageMarking)(e.Output) {
		return parseAndFormatPackageLine(e.Output, p.Style, p.PackageNameWidth, p.StripPackagePrefix)
	}
	return e.Output
}

func (p goQuietEvent) format(e gotest.Event) string {
	if p.Panic {
		return formatPanic(e.Output, p.Style)
	}
	if output.HasFailedTestMarking(e.Output) {
		return formatFailedTest(e.Output, p.Style)
	}
	if output.IsLogLine(e.Output) {
		return formatLogLine(e.PackageDirPath, e.Output, p.Style, p.IDE)
	}
	return e.Output
}

func parseAndFormatPackageLine(s string, st style.Go, maxTestName int, stripPackagePrefix string) string {
	// preserve trailer
	var trailer string
	endIdx := strings.Index(s, "\n")
	if endIdx > -1 {
		trailer = s[endIdx:]
		s = s[:endIdx]
	}

	fields := strings.Split(s, "\t")

	var pkgName, status string
	var aux []string

	if len(fields) > 0 {
		status = fields[0]
	}

	if len(fields) > 1 {
		pkgName = fields[1]
	}

	// with -cover, go prints a package with no test files (but some statements) as "\tpkg\t\tcoverage: 0.0% of
	// statements" instead of its usual "?   \tpkg\t[no test files]". Say so, the way go would have.
	if len(fields) == 4 && fields[0] == "" && fields[2] == "" && output.HasPackageCoverageMarking(fields[3]) {
		status = "?   "
		fields = append(fields, "[no test files]")
	}

	if len(fields) > 2 {
		aux = fields[2:]
		// go prints the elapsed time with three decimals ("2.542s"). Two is plenty to read and quieter.
		aux[0] = packageElapsedPattern.ReplaceAllStringFunc(aux[0], func(elapsed string) string {
			seconds, err := strconv.ParseFloat(strings.TrimSuffix(elapsed, "s"), 64)
			if err != nil {
				return elapsed
			}
			return fmt.Sprintf("%.2fs", seconds)
		})
		aux[0] = elapsedColumn(aux[0])
	}

	return Package{
		Status:         status,
		Name:           pkgName,
		TestsCompleted: 0,
		Aux:            aux,
		Trailer:        trailer,
		Style:          st,
		FormatStatus:   true,
		MaxTestName:    maxTestName,
		StripPrefix:    stripPackagePrefix,
	}.String()
}

// elapsedColumn is the one layout of the elapsed column, shared by package result lines, the live rows for running
// packages, and the summary footer, so their columns line up. It is always elapsedColumnWidth wide: a time is right
// aligned with a slot after it for a startup mark ("1.88s  " vs "6.20s ◕"), so times line up too, and anything else
// like "(cached)" or nothing at all (packages with no tests) is right aligned to the whole column.
func elapsedColumn(field string) string {
	elapsed, mark, _ := strings.Cut(strings.TrimSpace(field), " ")
	if !output.HasTimeMarker(elapsed) {
		return fmt.Sprintf("%*s", elapsedColumnWidth, strings.TrimSpace(field))
	}
	if mark == "" {
		mark = " "
	}
	return fmt.Sprintf("%*s %s", elapsedTimeWidth, elapsed, mark)
}
