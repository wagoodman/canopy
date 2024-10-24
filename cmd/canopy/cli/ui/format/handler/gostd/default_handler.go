package gostd

import (
	"fmt"
	"io"
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

func (n *DefaultPackage) render(writer io.Writer) {
	for _, e := range n.events {
		if !e.Reference.IsPackage() && !n.isFailedReference(e.Reference) {
			continue
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
			fmt.Fprint(writer, n.renderOutput(resultEvent))
		default:
			if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && n.config.HidePackagesWithNoTestFiles {
				continue
			}
			if hasRunMarking(e.Output) || hasPassMarking(e.Output) {
				// skip the run line
				continue
			}
			if !n.isFailedReference(e.Reference) && hasPackageCoverageMarking(e.Output) {
				// skip "coverage:" lines for passing tests
				continue
			}
			if strings.TrimSpace(e.Output) == "" {
				continue
			}

			out := n.renderOutput(e)
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

func hasFailedTestMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- FAIL:")
}

func hasFailedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "FAIL")
}

// func hasTimeMarker(output string) bool {
//	return timePattern.MatchString(strings.TrimSpace(output))
//}

var (
	logLinePattern = regexp.MustCompile(`^\s*\S+.go:\d+:`)
	// coveragePattern = regexp.MustCompile(`coverage:\s*\d+\.\d+%\sof\sstatements\s*$`)
	// timePattern = regexp.MustCompile(`^\d+\.\d+\S$`)
)

func isLogLine(output string) bool {
	// match regex for a line like this:
	//    palindrome_test.go:51: th
	return logLinePattern.MatchString(output)
}

func (n *DefaultPackage) renderOutput(e gotest.Event) string {
	if e.Reference.IsPackage() {
		return n.formatPackage(e)
	}
	// indent
	return strings.Repeat("    ", strings.Count(e.Reference.TestName(false), "/")) + n.format(e)
}

func (n *DefaultPackage) formatPackage(e gotest.Event) string {
	if hasFailedPackageMarking(e.Output) || hasPassedPackageMarking(e.Output) || hasUnknownPackageMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, n.style, n.config.PackageNameWidth)
	}
	if hasPackageCoverageMarking(e.Output) {
		// withhold this until you are showing the final package output
		n.packageCoverage[e.Reference] = e.Output
		return ""
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

	return FormatPackageLine(status, pkgName, aux, trailer, st, true, maxTestName)
}

func FormatPackageLine(status, pkgName string, aux []string, trailer string, st style.GoStd, formatStatus bool, maxTestName int) string {

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
	}

	if pkgName != "" {
		// make all test names the same width
		pkgName = fmt.Sprintf("%-*s", maxTestName, pkgName)
	}

	for i, a := range aux {
		// if hasTimeMarker(fields[idx]) {
		//	aux[i] = "(" + a + ")"
		//}

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

	before = strings.Replace(before, "--- FAIL:", "--- "+st.Failed.Render("FAIL")+":", 1)

	if aux != "" {
		aux = st.Aux.Render(aux)
	}

	if after != "" {
		after = st.Bold.Render(after)
	}

	return before + after + aux + trailer
}
