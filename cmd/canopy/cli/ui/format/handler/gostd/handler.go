package gostd

import (
	"fmt"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
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
	Reference        gotest.Reference
	Action           gotest.Action
	Start            time.Time
	End              *time.Time
	InfoOutputEvents []gotest.Event // '===' output lines for this test
	OutputEvents     []gotest.Event
	Children         []*testResult
	Parent           *testResult
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
	verbose      bool
	packages     map[string]*packageState
	packageOrder []string
	panic        map[gotest.Reference]bool
}

func NewHandler(writer io.Writer, verbose bool, config PackageConfig) handler.Handler {
	return &testHandler{
		writer:   writer,
		verbose:  verbose,
		packages: make(map[string]*packageState),
		panic:    make(map[gotest.Reference]bool),
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

func (h *testHandler) OnGoTestEvent(e gotest.Event) error {
	packageName := e.Reference.Package
	pkg := h.packages[packageName]

	if output.HasPanicMarking(e.Output) {
		h.panic[e.Reference] = true
	}

	switch e.Action {
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
		if e.Reference.FuncName != "" {
			test := &testResult{
				Reference: e.Reference,
				Start:     e.Time,
			}
			pkg.Tests[e.Reference] = test

			// determine parent-child relationships
			if !e.Reference.IsSubTest() {
				pkg.TopLevel = append(pkg.TopLevel, test)
			} else {
				parentRef := e.Reference.ParentRef()
				if parentRef != nil {
					if parent, exists := pkg.Tests[*parentRef]; exists {
						test.Parent = parent
						parent.Children = append(parent.Children, test)
					}
				}
			}
		}

	case gotest.OutputAction:
		if e.Reference.FuncName != "" {
			if test, exists := pkg.Tests[e.Reference]; exists {
				if strings.HasPrefix(e.Output, "=== ") {
					// add === event info to the test that is the function reference
					funTest := findTestFunction(test)
					if funTest != nil {
						funTest.InfoOutputEvents = append(funTest.InfoOutputEvents, e)
					}
				} else {
					test.OutputEvents = append(test.OutputEvents, e)
				}
			}
		} else {
			// package-level output (like FAIL line)
			pkg.FinalLines = append(pkg.FinalLines, e.Output)
		}

	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:

		if e.Reference.FuncName != "" {
			if test, exists := pkg.Tests[e.Reference]; exists {
				test.Action = e.Action
				test.End = &e.Time
			}
		} else {
			// package completed
			pkg.Action = e.Action

			// try to output completed packages in start order
			h.render()
		}
	}

	return nil
}

func findTestFunction(cur *testResult) *testResult {
	if cur.Reference.IsPackage() {
		return nil
	}
	if !cur.Reference.IsSubTest() {
		return cur
	}

	var last *testResult
	for cur != nil {
		if !cur.Reference.IsSubTest() {
			return cur
		}
		if cur.Parent == nil {
			return last
		}
		last = cur
		cur = cur.Parent
	}
	return last
}

func (h *testHandler) render() {
	for len(h.packageOrder) > 0 {
		pkgName := h.packageOrder[0]
		pkg := h.packages[pkgName]

		if !pkg.Action.Completed() {
			// this package isn't done yet, so we can't output anything after it
			break
		}

		if h.verbose {
			h.outputPackageVerbose(pkg)
		} else {
			h.outputPackageQuiet(pkg)
		}

		h.packageOrder = h.packageOrder[1:]
		delete(h.packages, pkgName)
	}
}

func (h *testHandler) outputPackageQuiet(pkg *packageState) {
	for _, test := range pkg.TopLevel {
		if h.hasFailure(test) {
			h.outputTestQuiet(test, 0)
		}
	}

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

	// only show failed tests
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

		// output logs only for failed tests
		if test.Action == gotest.FailAction {
			for _, e := range test.OutputEvents {
				if strings.TrimSpace(e.Output) != "" {
					fmt.Fprintf(h.writer, "%s%s", indentStr, e.Output)
				}
			}
		}

		// output failed children
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

func (h *testHandler) outputPackageVerbose(pkg *packageState) {
	// output top-level tests in order
	for _, test := range pkg.TopLevel {
		h.outputTestVerbose(test, 0)
	}

	// output package final lines
	for _, line := range pkg.FinalLines {
		fmt.Fprint(h.writer, line)
	}
}

func (h *testHandler) outputTestVerbose(test *testResult, indent int) {

	for _, infoOutputEvent := range test.InfoOutputEvents {
		fmt.Fprintf(h.writer, "%s", infoOutputEvent.Output)
	}

	indentStr := strings.Repeat("    ", indent)

	// output test's own output (logs, errors)
	for _, e := range test.OutputEvents {
		if strings.TrimSpace(e.Output) != "" {
			fmt.Fprintf(h.writer, "%s%s", indentStr, e.Output)
		}
	}

	// output children recursively
	for _, child := range test.Children {
		h.outputTestVerbose(child, indent+1)
	}
}

func (h *testHandler) String() string {
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
