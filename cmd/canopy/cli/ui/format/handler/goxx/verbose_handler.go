package goxx

import (
	"fmt"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
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
	writer        io.Writer
	config        VerbosePackageConfig
	style         style.Go
	lastOutputRef *gotest.Reference
	pkg           string
	buffer        *strings.Builder
	funcConcluded map[gotest.Reference]struct{}
	formatter     func(gotest.Event, bool) fmt.Stringer
	panic         map[gotest.Reference]bool
}

func NewVerbosePackage(writer io.Writer, config VerbosePackageConfig, ref gotest.Reference) *VerbosePackage {
	if !ref.IsPackage() {
		ref = ref.PackageRef()
	}
	st := style.NewGo(config.Color)
	return &VerbosePackage{
		writer:        writer,
		config:        config,
		style:         st,
		pkg:           ref.Package,
		buffer:        &strings.Builder{},
		funcConcluded: make(map[gotest.Reference]struct{}),
		panic:         make(map[gotest.Reference]bool),
		formatter: presenter.NewGoVerboseEventFactory(
			st,
			config.IDE,
			config.HideExecutionTestEvents,
			config.PackageNameWidth,
		).NewEvent,
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
		_, err := fmt.Fprint(writer, h.formatter(e, h.panic[e.Reference]))

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
