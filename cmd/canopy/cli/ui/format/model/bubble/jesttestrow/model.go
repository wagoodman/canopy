package jesttestrow

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
	_ frame.TerminalElement    = (*Model)(nil)
)

type Config struct {
	Color                       bool
	ShowPackages                bool
	KeepAllTestOutput           bool
	KeepFailedTestOutput        bool
	NestNonPackages             bool
	ExpireOnCompletion          bool
	DieOnCompletion             bool
	ShowIntermediateOutput      bool
	HidePackagesWithNoTestFiles bool

	Style *style.Jest
}

type Model struct {
	config    Config
	ref       gotest.Reference
	action    gotest.Action
	coverage  string
	output    []string
	testsSeen map[gotest.Reference]struct{}
	style     style.Jest
	ws        tea.WindowSizeMsg
}

func NewModel(ref gotest.Reference, ws tea.WindowSizeMsg, config Config) *Model {
	stRef := config.Style
	if stRef == nil {
		st := style.NewJest(config.Color)
		stRef = &st
	}
	return &Model{
		config:    config,
		ref:       ref,
		style:     *stRef,
		ws:        ws,
		testsSeen: make(map[gotest.Reference]struct{}),
	}
}

func (j Model) Init() tea.Cmd {
	return nil
}

func (j Model) ShouldImprint() bool {
	return j.isExpired(j.config.ExpireOnCompletion)
}

func (j Model) isExpired(enabled bool) bool {
	if !enabled {
		return false
	}
	switch j.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (j Model) IsAlive() bool {
	isPkg := j.ref.IsPackage()
	if j.config.ShowPackages && isPkg {
		return true
	}
	if !isPkg {
		if j.config.KeepAllTestOutput {
			return true
		}
		if j.config.KeepFailedTestOutput && j.action == gotest.FailAction {
			return true
		}
	}
	return !j.isExpired(true)
}

func (j Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	e, ok := msg.(partybus.Event)
	if !ok {
		return j, nil
	}

	if e.Type != event.GoTestType {
		return j, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return j, nil
	}

	if j.ref.IsPackage() && gt.Reference.Package == j.ref.Package && !gt.Reference.IsPackage() {
		if _, ok := j.testsSeen[gt.Reference]; !ok {
			j.testsSeen[gt.Reference] = struct{}{}
		}
	}

	if gt.Reference != j.ref {
		return j, nil
	}

	switch gt.Action {
	case gotest.RunAction, gotest.PassAction, gotest.FailAction, gotest.SkipAction, gotest.StartAction:
		j.action = gt.Action

	case gotest.OutputAction:
		j.output = append(j.output, gt.Output)

		if j.ref.IsPackage() && strings.HasPrefix(gt.Output, "coverage:") {
			j.coverage = gt.Output
		}
	}
	return j, nil
}

func (j Model) testTitleOutput() (title, output string) {
	if j.config.NestNonPackages && !j.ref.IsPackage() {
		return j.testNestedTitleOutput()
	}
	return j.testTopTitleOutput()
}

func (j Model) testNestedTitleOutput() (title, output string) {
	switch j.action {
	case gotest.RunAction, gotest.StartAction:

		title = j.style.Aux.Render("  …")
		if j.config.ShowIntermediateOutput && len(j.output) > 0 {
			output = j.style.Aux.Render(strings.TrimSpace(j.output[len(j.output)-1]))
		}
	case gotest.PassAction:
		title = j.style.CheckTitle.Render("  ✔")
	case gotest.FailAction:
		title = j.style.XTitle.Render("  ✕") // ✘✕✖

		// if j.ref.IsPackage() {
		//	output = j.s.Aux.Render(strings.TrimSpace(j.coverage))
		// } else {
		//	output = renderOutput(j.output, 0)
		//}

		if !j.ref.IsPackage() {
			output = renderOutput(j.output, 2)
		}

	case gotest.SkipAction:
		title = j.style.Aux.Render("  ►►")
	}
	return title, output
}

func (j Model) testTopTitleOutput() (title, output string) {
	switch j.action {
	case gotest.RunAction, gotest.StartAction:
		title = j.style.RunningTitle.Render(" RUNS ")
		if j.config.ShowIntermediateOutput && len(j.output) > 0 {
			output = j.style.Aux.Render(strings.TrimSpace(j.output[len(j.output)-1]))
		}
	case gotest.PassAction:
		title = j.style.SuccessTitle.Render(" PASS ")
	case gotest.FailAction:
		title = j.style.FailureTitle.Render(" FAIL ")

		// if j.ref.IsPackage() {
		//	output = j.s.Aux.Render(strings.TrimSpace(j.coverage))
		// } else {
		//	output = renderOutput(j.output, 0)
		//}

		if !j.ref.IsPackage() {
			output = renderOutput(j.output, 0)
		}

	case gotest.SkipAction:
		title = j.style.SkipTitle.Render(" SKIP ")
	}
	return title, output
}

func (j Model) View() string {
	var (
		title     string
		output    string
		testCount string
	)

	title, output = j.testTitleOutput()

	output = j.style.Aux.Render(output)

	testPkg, testName := splitTestRef(j.ref)

	if j.config.NestNonPackages {
		if !j.ref.IsPackage() {
			testPkg = ""
		} else {
			testPkg = j.style.Title.Render(testPkg)
		}

		if testName != "" {
			testPkg = j.style.Aux.Render(testPkg)
		}
	} else if testName != "" {
		testPkg = j.style.Aux.Render(testPkg)
		testName = j.style.Title.Render(testName)
	}

	if j.ref.IsPackage() {
		testCount = fmt.Sprintf(" (%d tests) ", len(j.testsSeen))
		testCount = j.style.Aux.Render(testCount)
	}

	return fmt.Sprintf("%-5s %s%s%s %s", title, testPkg, testName, testCount, output)
}

func renderOutput(lines []string, n int) string {
	sb := strings.Builder{}
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if strings.HasPrefix(clean, "=== RUN") || strings.HasPrefix(clean, "--- ") {
			continue
		}
		sb.WriteString(strings.Repeat(" ", n) + line)
	}
	ret := sb.String()
	if strings.TrimSpace(ret) == "" {
		return ""
	}
	return "\n" + ret
}

func splitTestRef(ref gotest.Reference) (string, string) {
	name := ref.TestName(false)
	suffix := ""
	if name != "" {
		suffix = "/"
	}
	return ref.Package + suffix, ref.TestName(false)
}
