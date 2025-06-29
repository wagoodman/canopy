package gopp

import (
	"fmt"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
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
	HideExecutionTestEvents     bool
}

func NewVerboseHandler(writer io.Writer, config VerbosePackageConfig) handler.Handler {
	return handler.NewPackageHandler(
		func(ref gotest.Reference, writer io.Writer) handler.Handler {
			return NewVerbosePackage(writer, config, ref)
		}, writer)
}

type VerbosePackage struct {
	writer          io.Writer
	config          VerbosePackageConfig
	style           style.Go
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
		style:           style.NewGo(config.Color),
		pkg:             ref.Package,
		buffer:          &strings.Builder{},
		funcConcluded:   make(map[gotest.Reference]struct{}),
		packageCoverage: make(map[gotest.Reference]string),
		panic:           make(map[gotest.Reference]bool),
	}
}

func (h *VerbosePackage) Handle(e partybus.Event) error {
	switch e.Type {
	case event.GoTestType:
		goTestEvent, err := parser.ParseGoTestType(e)
		if err != nil {
			log.Warnf("unable to parse go test event: %+v", err)
			return nil
		}

		return h.OnGoTestEvent(goTestEvent)
	}
	return nil
}

func (h *VerbosePackage) String() string {
	return h.buffer.String()
}

func (h *VerbosePackage) OnGoTestEvent(e gotest.Event) error {
	if e.Reference.Package != h.pkg {
		return nil
	}

	if e.HasAnnotation(gotest.NoTestFiles, gotest.NoTestsToRun) && h.config.HidePackagesWithNoTestFiles {
		return nil
	}

	if output.HasPanicMarking(e.Output) {
		h.panic[e.Reference] = true
	}

	if e.Action == gotest.OutputAction {
		if !e.Reference.IsPackage() {
			trimmedOutput := strings.TrimSpace(e.Output)
			if h.lastOutputRef == nil || *h.lastOutputRef != e.Reference {
				hasEqualMarker := strings.HasPrefix(trimmedOutput, "===")
				hasDashMarker := strings.HasPrefix(trimmedOutput, "---")
				if !hasEqualMarker && !hasDashMarker {
					out := h.style.Aux.Render(fmt.Sprintf("%s  %s", "═══ NAME", e.Reference.TestName(true)))
					_, err := fmt.Fprint(h.writer, out+"\n")
					if err != nil {
						return err
					}
				}
				h.lastOutputRef = &e.Reference
			}
		}

		var writer io.Writer = h.buffer
		if !h.funcRefConcluded(e.Reference) {
			writer = h.writer
		}
		_, err := fmt.Fprint(writer, h.renderOutput(e))

		return err
	}

	if !e.Reference.IsPackage() {
		return nil
	}

	switch e.Action {
	case gotest.PassAction, gotest.SkipAction, gotest.FailAction:
		funcRef := e.Reference.FuncRef()
		if funcRef != nil {
			h.funcConcluded[*funcRef] = struct{}{}
		}
		_, err := fmt.Fprint(h.writer, h.buffer.String())
		if err != nil {
			return err
		}
		return handler.ErrPackageComplete
	}

	return nil
}

func (h *VerbosePackage) funcRefConcluded(ref gotest.Reference) bool {
	funcRef := ref.FuncRef()
	if funcRef != nil {
		if _, exists := h.funcConcluded[*funcRef]; exists {
			return true
		}
	}
	return false
}

func (h *VerbosePackage) renderOutput(e gotest.Event) string {
	if e.Reference.IsPackage() {
		return h.formatPackage(e)
	}
	return h.format(e)
}

func (h *VerbosePackage) formatPackage(e gotest.Event) string {
	if output.HasFailedPackageMarking(e.Output) || output.HasPassedPackageMarking(e.Output) || output.HasUnknownPackageMarking(e.Output) || output.HasPassMarking(e.Output) {
		return parseAndFormatPackageLine(e.Output, h.style, h.config.PackageNameWidth)
	}
	if output.HasPackageCoverageMarking(e.Output) {
		// withhold this until you are showing the final package output
		h.packageCoverage[e.Reference] = e.Output
		return ""
	}
	return e.Output
}

func (h *VerbosePackage) format(e gotest.Event) string {
	if h.panic[e.Reference] {
		return formatPanic(e.Output, h.style)
	}
	if output.HasFailedTestMarking(e.Output) {
		return formatFailedTest(e.Output, h.style)
	}
	if output.HasTestPassMarking(e.Output) {
		return formatPassedTest(e.Output, h.style)
	}
	if output.HasTestStartMarking(e.Output) || output.HasContinueMarking(e.Output) || output.HasPauseMarking(e.Output) {
		if h.config.HideExecutionTestEvents {
			return ""
		}
		return formatTestExecutionMark(e.Output, h.style)
	}
	if output.IsLogLine(e.Output) {
		return formatLogLine(e.PackageDirPath, e.Output, h.style, h.config.IDE)
	}
	return e.Output
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
