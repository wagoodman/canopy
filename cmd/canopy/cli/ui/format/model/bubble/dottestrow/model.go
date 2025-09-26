package dottestrow

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

type Model struct {
	config    Config
	ref       gotest.Reference
	action    gotest.Action
	coverage  string
	output    []string
	testsSeen map[gotest.Reference]gotest.Action
	testOrder []gotest.Reference
	style     style.Dot

	common state.Common
}

func NewModel(ref gotest.Reference, common state.Common, config Config) *Model {
	stRef := config.Style
	if stRef == nil {
		st := style.NewDot(config.Color)
		stRef = &st
	}
	return &Model{
		config:    config,
		ref:       ref,
		style:     *stRef,
		common:    common,
		testsSeen: make(map[gotest.Reference]gotest.Action),
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

func (m Model) IsHidden() bool {
	isPkg := m.ref.IsPackage()
	if !isPkg && m.action != gotest.FailAction {
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
			m.testOrder = append(m.testOrder, gt.Reference)
		}
		m.testsSeen[gt.Reference] = gt.Action
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
	if m.action == gotest.FailAction {
		title = m.style.XTitle.Render("  ✕") // ✘✕✖

		// if m.ref.IsPackage() {
		//	output = m.s.Aux.Render(strings.TrimSpace(m.coverage))
		// } else {
		//	output = renderOutput(m.output, 0)
		//}

		if !m.ref.IsPackage() {
			output = renderOutput(m.output, 2, m.style)
		}
	}

	return title, output
}

func (m Model) testTopTitleOutput() (title, output string) {
	switch m.action {
	// TODO: why is action sometimes empty string?
	case gotest.RunAction, gotest.StartAction, "":
		status := m.common.Spinner.View
		if status == "" {
			status = "…"
		}

		title = m.style.RunningTitle.Render(status)
		if m.config.ShowIntermediateOutput && len(m.output) > 0 {
			output = m.style.Aux.Render(strings.TrimSpace(m.output[len(m.output)-1]))
		}
	case gotest.PassAction:
		title = m.style.SuccessTitle.Render("✔")
	case gotest.FailAction:
		title = m.style.FailureTitle.Render("✕")

		// if m.ref.IsPackage() {
		//	output = m.s.Aux.Render(strings.TrimSpace(m.coverage))
		// } else {
		//	output = renderOutput(m.output, 0)
		//}

		if !m.ref.IsPackage() {
			output = renderOutput(m.output, 0, m.style)
		}

	case gotest.SkipAction:
		title = m.style.SkipTitle.Render(" ")
	}
	return title, output
}

func (m Model) View() string { //nolint: gocognit, funlen
	var (
		title     string
		output    string
		testCount string
	)

	title, output = m.testTitleOutput()

	testPkg, testName := splitTestRef(m.ref)

	if m.config.NestNonPackages {
		if !m.ref.IsPackage() {
			testPkg = ""
		} else {
			switch m.action {
			case gotest.RunAction, gotest.StartAction:
				testPkg = m.style.RunningTitle.Render(testPkg)
			case gotest.FailAction:
				testPkg = m.style.FailureTitle.Render(testPkg)
			}
		}
	} else {
		if testName != "" {
			testPkg = m.style.Nested.Render(testPkg)
			testName = m.style.Nested.Render(testName)
		} else {
			switch m.action {
			case gotest.RunAction, gotest.StartAction:
				testPkg = m.style.RunningTitle.Render(testPkg)
			case gotest.FailAction:
				testPkg = m.style.FailureTitle.Render(testPkg)
			}
		}
	}

	var hasFailed, hasSkipped bool
	if m.ref.IsPackage() {
		var sections []string
		var currentSection string
		var lastAction *gotest.Action
		for i, ref := range m.testOrder {
			currentAction := m.testsSeen[ref]

			if lastAction == nil {
				lastAction = &currentAction
			}

			if *lastAction != currentAction || i == len(m.testOrder)-1 {
				switch *lastAction {
				case gotest.FailAction:
					currentSection = m.style.FailureTitle.Render(currentSection)
					hasFailed = true
				case gotest.SkipAction:
					currentSection = m.style.SkipTitle.Render(currentSection)
					hasSkipped = true
				default:
					currentSection = m.style.Dot.Render(currentSection)
				}
				sections = append(sections, currentSection)
				currentSection = ""
				lastAction = &currentAction
			}

			switch currentAction {
			case gotest.FailAction:
				currentSection += "✕"
			default:
				currentSection += "•"
			}
		}

		testCount = " " + strings.Join(sections, "")

		// very simple...
		// testCount = " " + strings.Repeat("•", len(m.testsSeen)) // .•·●⏺⦿
		// testCount = m.style.Dot.Render(testCount)
	}

	rendered := fmt.Sprintf("%-1s %s%s%s %s", title, testPkg, testName, testCount, output)
	if lipgloss.Width(rendered) > m.common.Window.Width && m.common.Window.Width > 0 {
		trailer := fmt.Sprintf("[%d]", len(m.testsSeen))
		if hasFailed {
			trailer = m.style.FailureTitle.Render(trailer)
		} else if hasSkipped {
			trailer = m.style.SkipTitle.Render(trailer)
		}

		rendered = ansi.Truncate(rendered, m.common.Window.Width, trailer)
	}
	return rendered
}

func renderOutput(lines []string, n int, st style.Dot) string {
	sb := strings.Builder{}
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if strings.HasPrefix(clean, "=== RUN") || strings.HasPrefix(clean, "--- ") {
			continue
		}
		sb.WriteString(strings.Repeat(" ", n) + formatLogLine(line, st))
	}
	ret := sb.String()
	if strings.TrimSpace(ret) == "" {
		return ""
	}
	return "\n" + ret
}

func formatLogLine(s string, st style.Dot) string {
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

	// format the fields

	return st.Aux.Render(file+":"+line) + ":" + rest
}

func splitTestRef(ref gotest.Reference) (string, string) {
	name := ref.TestName(false)
	suffix := ""
	if name != "" {
		suffix = "/"
	}
	return ref.Package + suffix, ref.TestName(false)
}
