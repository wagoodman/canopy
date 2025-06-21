package gostd

import (
	"fmt"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
	"io"
	"sort"
	"strings"
	"time"
)

var (
	_ handler.Handler  = (*testHandler)(nil)
	_ partybus.Handler = (*testHandler)(nil)
	_ fmt.Stringer     = (*testHandler)(nil)
)

type testResult struct {
	Reference gotest.Reference
	Action    gotest.Action
	Start     time.Time
	End       *time.Time
	Output    []string
	Children  []*testResult
	Parent    *testResult
}

type packageState struct {
	Name       string
	Action     gotest.Action
	Start      time.Time
	End        *time.Time
	Tests      map[gotest.Reference]*testResult
	TopLevel   []*testResult
	FinalLines []string
}

type PackageConfig struct {
	Color                       bool
	PackageNameWidth            int
	IDE                         ide.Context
	HidePackagesWithNoTestFiles bool
}

type testHandler struct {
	writer       io.Writer
	packages     map[string]*packageState
	packageOrder []string
}

func NewHandler(writer io.Writer, config PackageConfig) handler.Handler {
	return &testHandler{
		writer:   writer,
		packages: make(map[string]*packageState),
	}
}

func (h *testHandler) Handle(e partybus.Event) error {
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

func (h *testHandler) OnGoTestEvent(event gotest.Event) error {
	packageName := event.Reference.Package
	pkg := h.packages[packageName]

	switch event.Action {
	case gotest.StartAction:
		if _, exists := h.packages[packageName]; !exists {
			h.packages[packageName] = &packageState{
				Name:     packageName,
				Tests:    make(map[gotest.Reference]*testResult),
				TopLevel: []*testResult{},
			}
			h.packageOrder = append(h.packageOrder, packageName)
		}

	case gotest.RunAction:

		if event.Reference.FuncName != "" {
			test := &testResult{
				Reference: event.Reference,
				Output:    []string{},
				Start:     event.Time,
			}
			pkg.Tests[event.Reference] = test

			// determine parent-child relationships
			if !event.Reference.IsSubTest() {
				pkg.TopLevel = append(pkg.TopLevel, test)
			} else {
				parentRef := event.Reference.ParentRef()
				if parentRef != nil {
					if parent, exists := pkg.Tests[*parentRef]; exists {
						test.Parent = parent
						parent.Children = append(parent.Children, test)
					}
				}
			}
		}

	case gotest.OutputAction:
		if event.Reference.FuncName != "" {
			if test, exists := pkg.Tests[event.Reference]; exists {
				// only store non-RUN output lines
				if !strings.Contains(event.Output, "=== RUN") {
					test.Output = append(test.Output, event.Output)
				}
			}
		} else {
			// package-level output (like FAIL line)
			pkg.FinalLines = append(pkg.FinalLines, event.Output)
		}

	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:

		if event.Reference.FuncName != "" {
			if test, exists := pkg.Tests[event.Reference]; exists {
				test.Action = event.Action
				test.End = &event.Time
			}
		} else {
			// package completed
			pkg.Action = event.Action

			// try to output completed packages in start order
			h.render()
		}
	}

	return nil
}

func (h *testHandler) render() {
	// Find the first uncompleted package in start order
	for len(h.packageOrder) > 0 {
		pkgName := h.packageOrder[0]
		pkg := h.packages[pkgName]

		if !pkg.Action.Completed() {
			// This package isn't done yet, so we can't output anything after it
			break
		}

		// Output this package
		h.outputPackageQuiet(pkg)

		// Remove from the front of the slice to mark as output
		h.packageOrder = h.packageOrder[1:]
		delete(h.packages, pkgName)
	}
}

func (h *testHandler) outputPackageQuiet(pkg *packageState) {
	// Only output failed tests and their hierarchy
	for _, test := range pkg.TopLevel {
		if h.hasFailure(test) {
			h.outputTestQuiet(test, 0)
		}
	}

	// Output package final lines
	for _, line := range pkg.FinalLines {
		fmt.Fprint(h.writer, line)
	}
}

func (h *testHandler) hasFailure(test *testResult) bool {
	if test.Action == gotest.FailAction {
		return true
	}
	for _, child := range test.Children {
		if h.hasFailure(child) {
			return true
		}
	}
	return false
}

func (h *testHandler) outputTestQuiet(test *testResult, indent int) {
	indentStr := strings.Repeat("    ", indent)

	// Only show failed tests
	if test.Action == gotest.FailAction || h.hasFailedChildren(test) {
		if test.Action != gotest.UnknownAction {
			status := "PASS"
			if test.Action == gotest.FailAction {
				status = "FAIL"
			} else if test.Action == gotest.SkipAction {
				status = "SKIP"
			}
			fmt.Fprintf(h.writer, "%s--- %s: %s (%s)\n", indentStr, status, test.Reference.TestName(true), test.End.Sub(test.Start).Truncate(time.Millisecond))
		}

		// Output logs only for failed tests
		if test.Action == gotest.FailAction {
			for _, output := range test.Output {
				if strings.TrimSpace(output) != "" {
					fmt.Fprintf(h.writer, "%s%s", indentStr, output)
				}
			}
		}

		// Output failed children
		for _, child := range test.Children {
			if child.Action == gotest.FailAction || h.hasFailedChildren(child) {
				h.outputTestQuiet(child, indent+1)
			}
		}
	}
}

func (h *testHandler) hasFailedChildren(test *testResult) bool {
	for _, child := range test.Children {
		if child.Action == gotest.FailAction || h.hasFailedChildren(child) {
			return true
		}
	}
	return false
}

func (h *testHandler) String() string {
	// Return any buffered content from incomplete packages
	var pkgs []string
	for pkg := range h.packages {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	sb := strings.Builder{}
	for _, pkg := range pkgs {
		if state := h.packages[pkg]; state != nil && !state.Action.Completed() {
			// Could include partial output here if needed
			sb.WriteString(fmt.Sprintf("# Package %s still running...\n", pkg))
		}
	}
	return sb.String()
}
