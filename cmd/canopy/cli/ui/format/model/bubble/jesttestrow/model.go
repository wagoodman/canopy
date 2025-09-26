package jesttestrow

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
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
	common    state.Common
}

func NewModel(ref gotest.Reference, common state.Common, config Config) *Model {
	stRef := config.Style
	if stRef == nil {
		st := style.NewJest(config.Color)
		stRef = &st
	}
	return &Model{
		config:    config,
		ref:       ref,
		style:     *stRef,
		common:    common,
		testsSeen: make(map[gotest.Reference]struct{}),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) ShouldImprint() bool {
	return m.isExpired(m.config.ExpireOnCompletion)
}

func (m Model) isExpired(enabled bool) bool {
	if !enabled {
		return false
	}
	switch m.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (m Model) IsAlive() bool {
	isPkg := m.ref.IsPackage()
	if m.config.ShowPackages && isPkg {
		return true
	}
	if !isPkg {
		if m.config.KeepAllTestOutput {
			return true
		}
		if m.config.KeepFailedTestOutput && m.action == gotest.FailAction {
			return true
		}
	}
	return !m.isExpired(true)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.common.OnMessage(msg)

	e, ok := msg.(partybus.Event)
	if !ok {
		return m, nil
	}

	if e.Type != event.GoTestType {
		return m, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return m, nil
	}

	if m.ref.IsPackage() && gt.Reference.Package == m.ref.Package && !gt.Reference.IsPackage() {
		if _, ok := m.testsSeen[gt.Reference]; !ok {
			m.testsSeen[gt.Reference] = struct{}{}
		}
	}

	if gt.Reference != m.ref {
		return m, nil
	}

	switch gt.Action {
	case gotest.RunAction, gotest.PassAction, gotest.FailAction, gotest.SkipAction, gotest.StartAction:
		m.action = gt.Action

	case gotest.OutputAction:
		m.output = append(m.output, gt.Output)

		if m.ref.IsPackage() && output.HasPackageCoverageMarking(gt.Output) {
			m.coverage = gt.Output
		}
	}
	return m, nil
}

func (m Model) testTitleOutput() (title, output string) {
	if m.config.NestNonPackages && !m.ref.IsPackage() {
		return m.testNestedTitleOutput()
	}
	return m.testTopTitleOutput()
}

func (m Model) testNestedTitleOutput() (title, output string) {
	switch m.action {
	case gotest.RunAction, gotest.StartAction:
		status := m.common.Spinner.View
		if status == "" {
			status = "…"
		}

		title = m.style.Aux.Render(fmt.Sprintf("  %s", status))
		if m.config.ShowIntermediateOutput && len(m.output) > 0 {
			output = m.style.Aux.Render(strings.TrimSpace(m.output[len(m.output)-1]))
		}
	case gotest.PassAction:
		title = m.style.CheckTitle.Render("  ✔")
	case gotest.FailAction:
		title = m.style.XTitle.Render("  ✕") // ✘✕✖

		// if m.ref.IsPackage() {
		//	output = m.s.Aux.Render(strings.TrimSpace(m.coverage))
		// } else {
		//	output = renderOutput(m.output, 0)
		//}

		if !m.ref.IsPackage() {
			output = renderOutput(m.output, 2)
		}

	case gotest.SkipAction:
		title = m.style.Aux.Render("  ►►")
	}
	return title, output
}

func (m Model) testTopTitleOutput() (title, output string) {
	switch m.action {
	case gotest.RunAction, gotest.StartAction:
		title = m.style.RunningTitle.Render(" RUNS ")
		if m.config.ShowIntermediateOutput && len(m.output) > 0 {
			output = m.style.Aux.Render(strings.TrimSpace(m.output[len(m.output)-1]))
		}
	case gotest.PassAction:
		title = m.style.SuccessTitle.Render(" PASS ")
	case gotest.FailAction:
		title = m.style.FailureTitle.Render(" FAIL ")

		// if m.ref.IsPackage() {
		//	output = m.s.Aux.Render(strings.TrimSpace(m.coverage))
		// } else {
		//	output = renderOutput(m.output, 0)
		//}

		if !m.ref.IsPackage() {
			output = renderOutput(m.output, 0)
		}

	case gotest.SkipAction:
		title = m.style.SkipTitle.Render(" SKIP ")
	}
	return title, output
}

func (m Model) View() string {
	var (
		title     string
		output    string
		testCount string
	)

	title, output = m.testTitleOutput()

	output = m.style.Aux.Render(output)

	testPkg, testName := splitTestRef(m.ref)

	if m.config.NestNonPackages {
		if !m.ref.IsPackage() {
			testPkg = ""
		} else {
			testPkg = m.style.Title.Render(testPkg)
		}

		if testName != "" {
			testPkg = m.style.Aux.Render(testPkg)
		}
	} else if testName != "" {
		testPkg = m.style.Aux.Render(testPkg)
		testName = m.style.Title.Render(testName)
	}

	if m.ref.IsPackage() {
		testCount = fmt.Sprintf(" (%d tests) ", len(m.testsSeen))
		testCount = m.style.Aux.Render(testCount)
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
