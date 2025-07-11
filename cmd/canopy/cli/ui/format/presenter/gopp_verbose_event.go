package presenter

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/savioxavier/termlink"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
)

var _ Presenter = (*GoPPVerboseEvent)(nil)

type GoPPVerboseEventFactory struct {
	Style                   style.Go
	IDE                     ide.Context
	HideExecutionTestEvents bool
	PackageNameWidth        int
}

func NewGoPPVerboseEventFactory(sty style.Go, i ide.Context, hideExecutionTestEvents bool, packageNameWidth int) GoPPVerboseEventFactory {
	return GoPPVerboseEventFactory{
		Style:                   sty,
		IDE:                     i,
		HideExecutionTestEvents: hideExecutionTestEvents,
		PackageNameWidth:        packageNameWidth,
	}
}

func (f GoPPVerboseEventFactory) NewEvent(e gotest.Event, midPanic bool) fmt.Stringer {
	return GoPPVerboseEvent{
		Style:                   f.Style,
		IDE:                     f.IDE,
		HideExecutionTestEvents: f.HideExecutionTestEvents,
		PackageNameWidth:        f.PackageNameWidth,
		Event:                   e,
		Panic:                   midPanic,
	}
}

type GoPPVerboseEvent struct {
	Style                   style.Go
	IDE                     ide.Context
	Event                   gotest.Event
	HideExecutionTestEvents bool
	PackageNameWidth        int
	Panic                   bool
}

func (p GoPPVerboseEvent) Present(stdout, _ io.Writer) error {
	if _, err := fmt.Fprint(stdout, p.String()); err != nil {
		return fmt.Errorf("failed to write go test event output to stdout: %w", err)
	}
	return nil
}

func (p GoPPVerboseEvent) String() string {
	e := p.Event
	if e.Reference.IsPackage() {
		return p.formatPackage(e)
	}
	return p.format()
}

func (p GoPPVerboseEvent) formatPackage(e gotest.Event) string {
	if output.HasFailedPackageMarking(e.Output) || output.HasPassedPackageMarking(e.Output) || output.HasUnknownPackageMarking(e.Output) || output.HasPassMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, p.Style, p.PackageNameWidth)
	}
	if output.HasPackageCoverageMarking(e.Output) {
		// withhold this until you are showing the final package output
		// p.packageCoverage[e.Reference] = e.Output
		return ""
	}
	return e.Output
}

func (p GoPPVerboseEvent) format() string {
	e := p.Event
	if p.Panic {
		return formatPanic(e.Output, p.Style)
	}
	if output.HasFailedTestMarking(e.Output) {
		return formatFailedTest(e.Output, p.Style)
	}
	if output.HasTestPassMarking(e.Output) {
		return formatPassedTest(e.Output, p.Style)
	}
	if output.HasRunMarking(e.Output) || output.HasContinueMarking(e.Output) || output.HasPauseMarking(e.Output) {
		if p.HideExecutionTestEvents {
			return ""
		}
		return formatTestExecutionMark(e.Output, p.Style)
	}
	if output.IsLogLine(e.Output) {
		return formatLogLine(e.PackageDirPath, e.Output, p.Style, p.IDE)
	}
	return e.Output
}

func formatPanic(in string, sty style.Go) string {
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		prefix := " "
		switch {
		case output.HasPanicMarking(line):

			// split at the first space
			spaceIdx := strings.Index(line, " ")
			line = strings.Replace(line, ":", "", 1)
			if spaceIdx == -1 {
				line = sty.PanicTitle.Render(" " + line)
			} else {
				line = sty.PanicTitle.Render(" "+line[:spaceIdx]) + " " + line[spaceIdx:]
			}
			prefix = ""
		case line == "" && i == len(lines)-1:
			continue
		default:
			switch {
			case output.IsPanicGoRoutineLine(line):
				line = sty.PanicFile.Render(line)
			case output.IsPanicFuncLine(line):
				// format everything after the last slash, if no slash, format the entire line
				slashIdx := strings.LastIndex(line, "/")
				if slashIdx == -1 {
					line = sty.PanicFunc.Render(line)
				} else {
					line = sty.PanicFuncAux.Render(line[:slashIdx+1]) + sty.PanicFunc.Render(line[slashIdx+1:])
				}
			case output.IsPanicFileLine(line):
				// format the section of the line between the last / and the following space
				// which is the file name. If one of the two is missing, just format the whole line.
				slashIdx := strings.LastIndex(line, "/")
				if slashIdx == -1 {
					line = sty.PanicFile.Render(line)
				} else {
					spaceIdx := strings.Index(line[slashIdx+1:], " ")
					if spaceIdx == -1 {
						line = sty.PanicFile.Render(line)
					} else {
						line = sty.PanicFileAux.Render(line[:slashIdx+1]) + sty.PanicFile.Render(line[slashIdx+1:slashIdx+1+spaceIdx]) + sty.PanicFileAux.Render(line[slashIdx+1+spaceIdx:])
					}
				}
			case strings.HasPrefix(line, "\t"):
				// this is the top header, which seems to be off by one character
				// lets pad this to vertically align it with the other lines
				line = strings.Replace(line, "\t", "\t ", 1)
			}
		}

		lines[i] = sty.PanicGroup.Render("░") + prefix + line
	}
	return strings.Join(lines, "\n")
}

func splitWhitespace(s string) (prefix, content string) {
	for i, char := range s {
		if !strings.ContainsRune(" \t\n\r", char) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func formatFailedTest(s string, st style.Go) string {
	// split into "-- FAIL:" and the rest
	idx := strings.Index(s, ":")

	if idx == -1 {
		return s
	}

	before := s[:idx+1]
	after := s[idx+1:]

	// preserve but partition the line ending(s)
	lnIdx := strings.LastIndex(after, "\n")
	var trailer string
	if lnIdx > -1 {
		trailer = after[lnIdx:]
		after = after[:lnIdx]
	}

	// split off "(0.20s)"
	auxIdx := strings.LastIndex(after, "(")
	var aux string
	if auxIdx > -1 {
		aux = after[auxIdx:]
		after = after[:auxIdx]
	}

	// apply styles to all sections

	before = strings.Replace(before, "--- FAIL:", st.Aux.Render("─── ")+st.Failed.Render("FAIL")+" ", 1)

	if aux != "" {
		aux = st.Aux.Render(aux)
	}

	if after != "" {
		after = st.Bold.Render(after)
	}

	return before + after + aux + trailer
}

func formatLogLine(dir, s string, st style.Go, i ide.Context) string {
	// split into "file":"linenumber" and the rest
	idx := strings.Index(s, ":")
	if idx == -1 {
		return s
	}
	file := s[:idx]

	lineIdx := strings.Index(s[idx+1:], ":")
	if lineIdx == -1 {
		return s
	}

	line := s[idx+1 : idx+1+lineIdx]
	rest := s[idx+1+lineIdx+1:]

	location := fmt.Sprintf("%s:%s", file, line)
	if i != nil && dir != "" {
		lineInt, err := strconv.Atoi(line)
		if err == nil {
			openCmd := i.FileAtLineURL(filepath.Join(dir, strings.TrimSpace(file)), lineInt)
			whitespace, nonWhitespace := splitWhitespace(location)
			if nonWhitespace != "" {
				location = whitespace + termlink.Link(nonWhitespace, openCmd)
			} else {
				location = termlink.Link(whitespace, openCmd)
			}
		}
	}

	// note: we don't want to include newlines in the style render (which would be in "rest")
	return st.Aux.Render(location) + ":" + rest
}

func formatTestExecutionMark(s string, st style.Go) string {
	// preserve but partition the line ending(s)
	lnIdx := strings.LastIndex(s, "\n")
	var trailer string
	var line = s
	if lnIdx > -1 {
		trailer = line[lnIdx:]
		line = line[:lnIdx]
	}

	return st.Aux.Render(line) + trailer

	//// split into "=== RUN" and the rest
	// idx := strings.Index(s, "Test")
	// if idx == -1 {
	//	return s
	//}
	//
	// before := s[:idx]
	// after := s[idx:]
	//
	// return st.Aux.Render(before) + after
}

func formatPassedTest(s string, st style.Go) string {
	// split into "-- PASS:" and the rest
	idx := strings.Index(s, ":")

	if idx == -1 {
		return s
	}

	before := s[:idx+1]
	after := s[idx+1:]

	// preserve but partition the line ending(s)
	lnIdx := strings.LastIndex(after, "\n")
	var trailer string
	if lnIdx > -1 {
		trailer = after[lnIdx:]
		after = after[:lnIdx]
	}

	// split off "(0.20s)"
	auxIdx := strings.LastIndex(after, "(")
	var aux string
	if auxIdx > -1 {
		aux = after[auxIdx:]
		after = after[:auxIdx]
	}

	// apply styles to all sections

	before = strings.Replace(before, "--- PASS:", st.Aux.Render("─── ")+st.Success.Render("PASS")+" ", 1)

	if aux != "" {
		aux = st.Aux.Render(aux)
	}

	return before + after + aux + trailer
}
