package gostd

import (
	"fmt"
	"io"
	"strings"

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
	_ handler.Handler  = (*VerbosePackage)(nil)
	_ partybus.Handler = (*VerbosePackage)(nil)
	_ fmt.Stringer     = (*VerbosePackage)(nil)
)

type VerbosePackageConfig struct {
	Color                       bool
	PackageNameWidth            int
	IDE                         ide.Context
	HidePackagesWithNoTestFiles bool
}

func NewVerboseHandler(writer io.Writer, config VerbosePackageConfig) handler.Handler {
	return newPackageHandler(
		func(ref gotest.Reference, writer io.Writer) handler.Handler {
			return NewVerbosePackage(writer, config, ref)
		}, writer)
}

type VerbosePackage struct {
	writer          io.Writer
	config          VerbosePackageConfig
	style           style.GoStd
	lastOutputRef   *gotest.Reference
	pkg             string
	buffer          *strings.Builder
	funcConcluded   map[gotest.Reference]struct{}
	packageCoverage map[gotest.Reference]string
	panic           map[gotest.Reference]bool
}

func NewVerbosePackage(writer io.Writer, config VerbosePackageConfig, ref gotest.Reference) *VerbosePackage {
	if !ref.IsPackage() {
		ref = ref.PackageRef()
	}
	return &VerbosePackage{
		writer:          writer,
		config:          config,
		style:           style.NewGoStd(config.Color),
		pkg:             ref.Package,
		buffer:          &strings.Builder{},
		funcConcluded:   make(map[gotest.Reference]struct{}),
		packageCoverage: make(map[gotest.Reference]string),
		panic:           make(map[gotest.Reference]bool),
	}
}

func (n *VerbosePackage) Handle(e partybus.Event) error {
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

func (n *VerbosePackage) String() string {
	return n.buffer.String()
}

func (n *VerbosePackage) OnGoTestEvent(e gotest.Event) error {
	if e.Reference.Package != n.pkg {
		return nil
	}

	if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && n.config.HidePackagesWithNoTestFiles {
		return nil
	}

	if hasPanicMarking(e.Output) {
		n.panic[e.Reference] = true
	}

	if e.Action == gotest.OutputAction {
		if !e.Reference.IsPackage() {
			trimmedOutput := strings.TrimSpace(e.Output)
			if n.lastOutputRef == nil || *n.lastOutputRef != e.Reference {
				hasEqualMarker := strings.HasPrefix(trimmedOutput, "===")
				hasDashMarker := strings.HasPrefix(trimmedOutput, "---")
				if !hasEqualMarker && !hasDashMarker {
					out := n.style.Aux.Render(fmt.Sprintf("%s  %s", "=== NAME", e.Reference.TestName(true)))
					_, err := fmt.Fprint(n.writer, out+"\n")
					if err != nil {
						return err
					}
				}
				n.lastOutputRef = &e.Reference
			}
		}

		var writer io.Writer = n.buffer
		if !n.funcRefConcluded(e.Reference) {
			writer = n.writer
		}
		_, err := fmt.Fprint(writer, n.renderOutput(e))

		return err
	}

	if !e.Reference.IsPackage() {
		return nil
	}

	switch e.Action {
	case gotest.PassAction, gotest.SkipAction, gotest.FailAction:
		funcRef := e.Reference.FuncRef()
		if funcRef != nil {
			n.funcConcluded[*funcRef] = struct{}{}
		}
		_, err := fmt.Fprint(n.writer, n.buffer.String())
		if err != nil {
			return err
		}
		return ErrPackageComplete
	}

	return nil
}

func (n *VerbosePackage) funcRefConcluded(ref gotest.Reference) bool {
	funcRef := ref.FuncRef()
	if funcRef != nil {
		if _, exists := n.funcConcluded[*funcRef]; exists {
			return true
		}
	}
	return false
}

func (n *VerbosePackage) renderOutput(e gotest.Event) string {
	if e.Reference.IsPackage() {
		return n.formatPackage(e)
	}
	return n.format(e)
}

func (n *VerbosePackage) formatPackage(e gotest.Event) string {
	if hasFailedPackageMarking(e.Output) || hasPassedPackageMarking(e.Output) || hasUnknownPackageMarking(e.Output) || hasPassMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, n.style, n.config.PackageNameWidth)
	}
	if hasPackageCoverageMarking(e.Output) {
		// withhold this until you are showing the final package output
		n.packageCoverage[e.Reference] = e.Output
		return ""
	}
	return e.Output
}

func (n *VerbosePackage) format(e gotest.Event) string {
	if n.panic[e.Reference] {
		return formatPanic(e.Output, n.style)
	}
	if hasFailedTestMarking(e.Output) {
		return formatFailedTest(e.Output, n.style)
	}
	if hasTestPassMarking(e.Output) {
		return formatPassedTest(e.Output, n.style)
	}
	if hasTestStartMarking(e.Output) {
		return formatTestStart(e.Output, n.style)
	}
	if isLogLine(e.Output) {
		return formatLogLine(e.PackageDirPath, e.Output, n.style, n.config.IDE)
	}
	return e.Output
}

func hasTestPassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- PASS:")
}

func hasTestStartMarking(output string) bool {
	return strings.HasPrefix(output, "=== RUN")
}

func formatTestStart(s string, st style.GoStd) string {
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

func formatPassedTest(s string, st style.GoStd) string {
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

	before = strings.Replace(before, "--- PASS:", "--- "+st.Success.Render("PASS")+":", 1)

	if aux != "" {
		aux = st.Aux.Render(aux)
	}

	return before + after + aux + trailer
}
