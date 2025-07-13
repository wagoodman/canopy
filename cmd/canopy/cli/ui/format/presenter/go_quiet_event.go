package presenter

import (
	"fmt"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
)

type GoQuietEventFactory struct {
	Style            style.Go
	IDE              ide.Context
	PackageNameWidth int
}

func NewGoQuietEventFactory(sty style.Go, i ide.Context, packageNameWidth int) GoQuietEventFactory {
	return GoQuietEventFactory{
		Style:            sty,
		IDE:              i,
		PackageNameWidth: packageNameWidth,
	}
}

func (f GoQuietEventFactory) NewEvent(e gotest.Event, midPanic bool) fmt.Stringer {
	return goxxQuietEvent{
		Style:            f.Style,
		IDE:              f.IDE,
		PackageNameWidth: f.PackageNameWidth,
		Event:            e,
		Panic:            midPanic,
	}
}

type goxxQuietEvent struct {
	Style            style.Go
	IDE              ide.Context
	Event            gotest.Event
	PackageNameWidth int
	Panic            bool
}

func (p goxxQuietEvent) Present(stdout, _ io.Writer) error {
	if _, err := fmt.Fprint(stdout, p.String()); err != nil {
		return fmt.Errorf("failed to write go test event output to stdout: %w", err)
	}
	return nil
}

func (p goxxQuietEvent) String() string {
	e := p.Event
	if e.Reference.IsPackage() {
		return p.formatPackage(e)
	}

	// indent
	return strings.Repeat("    ", strings.Count(e.Reference.TestName(false), "/")) + p.format(e)
}

func (p goxxQuietEvent) formatPackage(e gotest.Event) string {
	if output.HasFailedPackageMarking(e.Output) || output.HasPackageOKMarking(e.Output) || output.HasUnknownPackageMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, p.Style, p.PackageNameWidth)
	}
	return e.Output
}

func (p goxxQuietEvent) format(e gotest.Event) string {
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

func parseAndFormatPackageLine(s string, st style.Go, maxTestName int) string {
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

	if len(fields) > 2 {
		aux = fields[2:]
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
	}.String()
}
