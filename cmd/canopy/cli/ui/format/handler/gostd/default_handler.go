package gostd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/savioxavier/termlink"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var (
	_ handler.Handler  = (*DefaultPackage)(nil)
	_ partybus.Handler = (*DefaultPackage)(nil)
	_ fmt.Stringer     = (*DefaultPackage)(nil)
)

type DefaultPackageConfig struct {
	Color                       bool
	PackageNameWidth            int
	IDE                         ide.Context
	HidePackagesWithNoTestFiles bool
}

func NewDefaultHandler(writer io.Writer, config DefaultPackageConfig) handler.Handler {
	return newPackageHandler(
		func(ref gotest.Reference, writer io.Writer) handler.Handler {
			return NewDefaultPackage(writer, config, ref)
		}, writer)
}

type DefaultPackage struct {
	writer          io.Writer
	config          DefaultPackageConfig
	style           style.GoStd
	pkg             string
	events          []gotest.Event
	failedRefs      map[gotest.Reference]struct{}
	resultEvent     map[gotest.Reference]gotest.Event
	packageCoverage map[gotest.Reference]string
}

func NewDefaultPackage(writer io.Writer, config DefaultPackageConfig, ref gotest.Reference) *DefaultPackage {
	return &DefaultPackage{
		writer:          writer,
		config:          config,
		style:           style.NewGoStd(config.Color),
		pkg:             ref.Package,
		failedRefs:      make(map[gotest.Reference]struct{}),
		resultEvent:     make(map[gotest.Reference]gotest.Event),
		packageCoverage: make(map[gotest.Reference]string),
	}
}

func (n *DefaultPackage) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		return n.OnGoTestEvent(goTestEvent)
	}
	return nil
}

func (n *DefaultPackage) OnGoTestEvent(e gotest.Event) error {
	if e.Reference.Package != n.pkg {
		return nil
	}

	isPkg := e.Reference.IsPackage()

	if hasFailedTestMarking(e.Output) {
		n.resultEvent[e.Reference] = e
	} else {
		n.events = append(n.events, e)
	}

	switch e.Action {
	case gotest.FailAction:
		n.failedRefs[e.Reference] = struct{}{}
		fallthrough
	case gotest.PassAction, gotest.SkipAction:
		if isPkg {
			n.render(n.writer)
			return ErrPackageComplete
		}
	}
	return nil
}

func (n *DefaultPackage) String() string {
	sb := strings.Builder{}
	n.render(&sb)
	return sb.String()
}

func (n *DefaultPackage) render(writer io.Writer) { //nolint: gocognit
	panicRefs := make(map[gotest.Reference]bool)
	for _, e := range n.events {
		if !e.Reference.IsPackage() && !n.isFailedReference(e.Reference) {
			continue
		}

		if hasPanicMarking(e.Output) {
			panicRefs[e.Reference] = true
		}

		switch e.Action {
		case gotest.RunAction:
			// replace with the eventual result
			resultEvent, ok := n.resultEvent[e.Reference]
			if !ok {
				// TODO, not great
				log.Warnf("no result found for test: %s", e.Reference)
				continue
			}

			if strings.TrimSpace(resultEvent.Output) == "" {
				continue
			}
			fmt.Fprint(writer, n.renderOutput(resultEvent, panicRefs[e.Reference]))
		default:
			if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && n.config.HidePackagesWithNoTestFiles {
				continue
			}
			if hasRunMarking(e.Output) || hasPassMarking(e.Output) {
				// skip the run line
				continue
			}
			if hasPackageCoverageMarking(e.Output) {
				// skip "coverage:" lines
				continue
			}
			if strings.TrimSpace(e.Output) == "" {
				continue
			}

			out := n.renderOutput(e, panicRefs[e.Reference])
			if out != "" {
				fmt.Fprint(writer, out)
			}
		}
	}
}

func (n *DefaultPackage) isFailedReference(ref gotest.Reference) bool {
	_, ok := n.failedRefs[ref]
	return ok
}

func hasPackageCoverageMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "coverage:")
}

func hasPassedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "ok")
}

func hasUnknownPackageMarking(output string) bool {
	return strings.HasPrefix(output, "?") || strings.HasPrefix(output, "\t")
}

func hasPassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "PASS")
}

func hasRunMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== RUN")
}

func hasContinueMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== CONT")
}

func hasPauseMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== PAUSE")
}

func hasFailedTestMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- FAIL:")
}

func hasFailedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "FAIL")
}

func hasPanicMarking(output string) bool {
	return strings.HasPrefix(output, "panic:")
}

func hasTimeMarker(output string) bool {
	return timePattern.MatchString(strings.TrimSpace(output))
}

var (
	logLinePattern = regexp.MustCompile(`^\s*\S+.go:\d+:`)
	timePattern    = regexp.MustCompile(`^\d+\.?\d*\S+$`)
)

func isLogLine(output string) bool {
	// match regex for a line like this:
	//    palindrome_test.go:51: th
	return logLinePattern.MatchString(output)
}

func (n *DefaultPackage) renderOutput(e gotest.Event, isPanic bool) string {
	if e.Reference.IsPackage() {
		return n.formatPackage(e)
	}

	var out string
	if isPanic {
		out = formatPanic(e.Output, n.style)
	} else {
		out = n.format(e)
	}

	// indent
	return strings.Repeat("    ", strings.Count(e.Reference.TestName(false), "/")) + out
}

func (n *DefaultPackage) formatPackage(e gotest.Event) string {
	if hasFailedPackageMarking(e.Output) || hasPassedPackageMarking(e.Output) || hasUnknownPackageMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, n.style, n.config.PackageNameWidth)
	}
	return e.Output
}

func (n *DefaultPackage) format(e gotest.Event) string {
	if hasFailedTestMarking(e.Output) {
		return formatFailedTest(e.Output, n.style)
	}
	if isLogLine(e.Output) {
		return formatLogLine(e.PackageDirPath, e.Output, n.style, n.config.IDE)
	}
	return e.Output
}

func formatPanic(in string, sty style.GoStd) string {
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		prefix := " "
		switch {
		case hasPanicMarking(line):

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
			case isPanicGoRoutineLine(line):
				line = sty.PanicFile.Render(line)
			case isPanicFuncLine(line):
				// format everything after the last slash, if no slash, format the entire line
				slashIdx := strings.LastIndex(line, "/")
				if slashIdx == -1 {
					line = sty.PanicFunc.Render(line)
				} else {
					line = sty.PanicFuncAux.Render(line[:slashIdx+1]) + sty.PanicFunc.Render(line[slashIdx+1:])
				}
			case isPanicFileLine(line):
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

var goroutinePattern = regexp.MustCompile(`^goroutine \d+`)

func isPanicGoRoutineLine(s string) bool {
	return goroutinePattern.MatchString(s)
}

func isPanicFuncLine(s string) bool {
	return !strings.HasPrefix(s, "\t") && strings.Contains(s, "(") && strings.Contains(s, ")")
}

func isPanicFileLine(s string) bool {
	return strings.HasPrefix(s, "\t"+string(os.PathSeparator))
}

func parseAndFormatPackageLine(s string, st style.GoStd, maxTestName int) string {
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

	return FormatPackageLine(status, pkgName, 0, aux, trailer, st, true, maxTestName)
}

func FormatPackageLine(status, pkgName string, testsCompleted int, aux []string, trailer string, st style.GoStd, formatStatus bool, maxTestName int) string {
	if formatStatus {
		switch {
		case hasPassMarking(status):
			status = st.Success.Render(status)
		case hasPassedPackageMarking(status):
			status = st.Success.Render(status)
		case hasUnknownPackageMarking(status):
			status = st.Aux.Render(status)
		case hasFailedPackageMarking(status):
			status = st.Failed.Render(status)
		}
	} else if testsCompleted > 0 {
		runStr := fmt.Sprintf("%d tests", testsCompleted)
		aux = append(aux, runStr)
	}

	if pkgName != "" {
		// make all test names the same width
		pkgName = fmt.Sprintf("%-*s", maxTestName, pkgName)
	}

	for i, a := range aux {
		switch {
		case hasTimeMarker(a):
			break

		case strings.ContainsAny(a, "(["):
			// already formatted
			break
		case hasPackageCoverageMarking(a):
			a = strings.ReplaceAll(strings.ReplaceAll(a, "coverage: ", "[")+"]", "of statements", "coverage")

		default:
			a = "[" + a + "]"
		}

		aux[i] = st.Aux.Render(a)
	}

	return strings.Join(append([]string{status, pkgName}, aux...), "\t") + trailer
}

func formatLogLine(dir, s string, st style.GoStd, i ide.Context) string {
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

func splitWhitespace(s string) (prefix, content string) {
	for i, char := range s {
		if !strings.ContainsRune(" \t\n\r", char) {
			return s[:i], s[i:]
		}
	}
	return s, ""
}

func formatFailedTest(s string, st style.GoStd) string {
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
