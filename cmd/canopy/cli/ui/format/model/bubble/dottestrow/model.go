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
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly"
	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ bubbly.EventHandler      = (*Factory)(nil)
	_ bubbly.MessageListener   = (*Factory)(nil)
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
	_ frame.TerminalElement    = (*Model)(nil)
)

type Config struct {
	Color                  bool
	ShowPackages           bool
	KeepFailedTestOutput   bool
	NestNonPackages        bool
	ExpireOnCompletion     bool
	DieOnCompletion        bool
	ShowIntermediateOutput bool
	Style                  *style.Dot
}

type Factory struct {
	config Config
	seen   map[gotest.Reference]struct{}
	common state.Common
}

func NewFactory(config Config) *Factory {
	return &Factory{
		config: config,
		seen:   make(map[gotest.Reference]struct{}),
	}
}

func (j *Factory) OnMessage(msg tea.Msg) {
	j.common.OnMessage(msg)
}

func (j Factory) RespondsTo() []partybus.EventType {
	return []partybus.EventType{event.GoTestType}
}

func (j Factory) Handle(e partybus.Event) ([]tea.Model, tea.Cmd) {
	if e.Type != event.GoTestType {
		return nil, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return nil, nil
	}

	if !j.config.ShowPackages && gt.Reference.IsPackage() {
		return nil, nil
	}

	if _, ok := j.seen[gt.Reference]; ok {
		return nil, nil
	}

	j.seen[gt.Reference] = struct{}{}
	return []tea.Model{NewModel(gt.Reference, j.common, j.config)}, nil
}

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

func (j Model) IsHidden() bool {
	isPkg := j.ref.IsPackage()
	if !isPkg && j.action != gotest.FailAction {
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
		if j.config.KeepFailedTestOutput && j.action == gotest.FailAction {
			return true
		}
	}
	return !j.isExpired(true)
}

func (j Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	j.common.OnMessage(msg)

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
			j.testOrder = append(j.testOrder, gt.Reference)
		}
		j.testsSeen[gt.Reference] = gt.Action
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
	if j.action == gotest.FailAction {
		title = j.style.XTitle.Render("  ✕") // ✘✕✖

		// if j.ref.IsPackage() {
		//	output = j.s.Aux.Render(strings.TrimSpace(j.coverage))
		// } else {
		//	output = renderOutput(j.output, 0)
		//}

		if !j.ref.IsPackage() {
			output = renderOutput(j.output, 2, j.style)
		}
	}

	return title, output
}

func (j Model) testTopTitleOutput() (title, output string) {
	switch j.action {
	// TODO: why is action sometimes empty string?
	case gotest.RunAction, gotest.StartAction, "":
		status := j.common.Spinner.View
		if status == "" {
			status = "…"
		}

		title = j.style.RunningTitle.Render(status)
		if j.config.ShowIntermediateOutput && len(j.output) > 0 {
			output = j.style.Aux.Render(strings.TrimSpace(j.output[len(j.output)-1]))
		}
	case gotest.PassAction:
		title = j.style.SuccessTitle.Render("✔")
	case gotest.FailAction:
		title = j.style.FailureTitle.Render("✕")

		// if j.ref.IsPackage() {
		//	output = j.s.Aux.Render(strings.TrimSpace(j.coverage))
		// } else {
		//	output = renderOutput(j.output, 0)
		//}

		if !j.ref.IsPackage() {
			output = renderOutput(j.output, 0, j.style)
		}

	case gotest.SkipAction:
		title = j.style.SkipTitle.Render(" ")
	}
	return title, output
}

func (j Model) View() string { //nolint: gocognit, funlen
	var (
		title     string
		output    string
		testCount string
	)

	title, output = j.testTitleOutput()

	testPkg, testName := splitTestRef(j.ref)

	if j.config.NestNonPackages {
		if !j.ref.IsPackage() {
			testPkg = ""
		} else {
			switch j.action {
			case gotest.RunAction, gotest.StartAction:
				testPkg = j.style.RunningTitle.Render(testPkg)
			case gotest.FailAction:
				testPkg = j.style.FailureTitle.Render(testPkg)
			}
		}
	} else {
		if testName != "" {
			testPkg = j.style.Nested.Render(testPkg)
			testName = j.style.Nested.Render(testName)
		} else {
			switch j.action {
			case gotest.RunAction, gotest.StartAction:
				testPkg = j.style.RunningTitle.Render(testPkg)
			case gotest.FailAction:
				testPkg = j.style.FailureTitle.Render(testPkg)
			}
		}
	}

	var hasFailed, hasSkipped bool
	if j.ref.IsPackage() {
		var sections []string
		var currentSection string
		var lastAction *gotest.Action
		for i, ref := range j.testOrder {
			currentAction := j.testsSeen[ref]

			if lastAction == nil {
				lastAction = &currentAction
			}

			if *lastAction != currentAction || i == len(j.testOrder)-1 {
				switch *lastAction {
				case gotest.FailAction:
					currentSection = j.style.FailureTitle.Render(currentSection)
					hasFailed = true
				case gotest.SkipAction:
					currentSection = j.style.SkipTitle.Render(currentSection)
					hasSkipped = true
				default:
					currentSection = j.style.Dot.Render(currentSection)
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
		// testCount = " " + strings.Repeat("•", len(j.testsSeen)) // .•·●⏺⦿
		// testCount = j.style.Dot.Render(testCount)
	}

	rendered := fmt.Sprintf("%-1s %s%s%s %s", title, testPkg, testName, testCount, output)
	if lipgloss.Width(rendered) > j.common.Window.Width && j.common.Window.Width > 0 {
		trailer := fmt.Sprintf("[%d]", len(j.testsSeen))
		if hasFailed {
			trailer = j.style.FailureTitle.Render(trailer)
		} else if hasSkipped {
			trailer = j.style.SkipTitle.Render(trailer)
		}

		rendered = ansi.Truncate(rendered, j.common.Window.Width, trailer)
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
