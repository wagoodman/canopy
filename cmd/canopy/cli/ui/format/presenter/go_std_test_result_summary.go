package presenter

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ Presenter = (*JestTestResultSummary)(nil)

type GoStdTestResultSummaryConfig struct {
	Color            bool
	WriteToStderr    bool
	PackageNameWidth int
	PackageCount     int
	HidePackageCount bool
	RunningState     string
}

func (c GoStdTestResultSummaryConfig) New(run gotest.Run) Presenter {
	return GoStdTestResultSummary{
		config: c,
		style:  style.NewGoStd(c.Color),
		run:    run,
	}
}

type GoStdTestResultSummary struct {
	config GoStdTestResultSummaryConfig
	style  style.GoStd
	run    gotest.Run
}

func (s GoStdTestResultSummary) Present(stdout, stderr io.Writer) error {
	var w = stdout
	if s.config.WriteToStderr {
		w = stderr
	}

	passed, isRunning := s.run.Result.Passed()

	var result string

	// refStr := refCompactString(s.config.RunningState, s.run.Result.ReferencesByAction(gotest.RunAction), s.run.Result.ReferencesByAction(gotest.OutputAction))
	//
	//if refStr != "" {
	//	result += refStr + "\n"
	//}

	switch {
	case isRunning:
		if s.config.RunningState != "" {
			result += s.style.Running.Render(s.config.RunningState)
		} else {
			result += s.style.Running.Render("RUNNING")
		}
	case !passed:
		result += s.style.Failed.Render("FAIL")
	default:
		result += s.style.Success.Render("PASS")
	}

	stats := s.run.Result.TestStats()

	var tests []string

	if stats.Passed > 0 {
		tests = append(tests, s.style.Success.Render(fmt.Sprintf("%d passed", stats.Passed)))
	}

	if stats.Failed > 0 {
		tests = append(tests, s.style.Failed.Render(fmt.Sprintf("%d failed", stats.Failed)))
	}

	if stats.Skipped > 0 {
		tests = append(tests, s.style.Skipped.Render(fmt.Sprintf("%d skipped", stats.Skipped)))
	}

	total := stats.Total()
	if total != stats.Passed || total == 0 {
		tests = append(tests, fmt.Sprintf("%d total", stats.Total()))
	}

	testSummaryCount := strings.Join(tests, " / ")

	var sections []string

	if !s.config.HidePackageCount {
		sections = append(sections, fmt.Sprintf("%d packages", s.config.PackageCount))
	}

	sections = append(sections, fmt.Sprintf("%s tests", testSummaryCount))

	summary := strings.Join(sections, ", ")
	wideSummary := lipgloss.NewStyle().Width(s.config.PackageNameWidth).Render(summary)

	result += "\t" + wideSummary

	result += "\t" + s.style.Aux.Render(s.run.Elapsed().Round(time.Millisecond).String())

	if coverage, ok := s.run.Result.Coverage(); ok {
		result += "\t" + s.style.Aux.Render(fmt.Sprintf("coverage: %0.1f%% of statements", coverage))
	}

	if _, err := fmt.Fprintln(w, result); err != nil {
		return fmt.Errorf("failed to write final report: %w", err)
	}

	return nil
}

// func refCompactString(runningState string, refss ...gotest.References) string {
//	var refs gotest.References
//	for _, r := range refss {
//		refs = append(refs, r...)
//	}
//	sort.Sort(refs)
//	refsByPkg := make(map[string][]gotest.Reference)
//
//	var lastPkg string
//	var pkgs []string
//	for _, ref := range refs {
//		if ref.Package != lastPkg {
//			pkgs = append(pkgs, ref.Package)
//			lastPkg = ref.Package
//		}
//		refsByPkg[ref.Package] = append(refsByPkg[ref.Package], ref)
//	}
//
//	// show std status line for each in progress
//
//	var result strings.Builder
//	for _, pkg := range pkgs {
//		result.WriteString(fmt.Sprintf("%s\t%s\t%d running tests\n", runningState, pkg, len(refsByPkg[pkg])))
//	}
//
//	return result.String()
//}
