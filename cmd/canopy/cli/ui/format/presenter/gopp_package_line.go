package presenter

import (
	"fmt"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"io"
	"strings"
)

type Package struct {
	Status         string
	Name           string
	TestsCompleted int
	Aux            []string
	Trailer        string
	Style          style.Go
	FormatStatus   bool
	MaxTestName    int
}

func (p Package) Present(stdout, _ io.Writer) error {
	if _, err := fmt.Fprint(stdout, p.String()); err != nil {
		return fmt.Errorf("failed to write go test package output to stdout: %w", err)
	}
	return nil
}

// func FormatPackageLine(status, pkgName string, testsCompleted int, aux []string, trailer string, st style.Go, formatStatus bool, maxTestName int) string {
func (p Package) String() string {

	var status = p.Status
	var aux = p.Aux
	if p.FormatStatus {
		switch {
		case output.HasPassMarking(status):
			status = p.Style.Success.Render(status)
		case output.HasPassedPackageMarking(status):
			status = p.Style.Success.Render(status)
		case output.HasUnknownPackageMarking(status):
			status = p.Style.Aux.Render(status)
		case output.HasFailedPackageMarking(status):
			status = p.Style.Failed.Render(status)
		}
	} else if p.TestsCompleted > 0 {
		runStr := fmt.Sprintf("%d tests", p.TestsCompleted)
		aux = append(aux, runStr)
	}

	if p.Name != "" {
		// make all test names the same width
		p.Name = fmt.Sprintf("%-*s", p.MaxTestName, p.Name)
	}

	for i, a := range aux {
		switch {
		case output.HasTimeMarker(a):
			break

		case strings.ContainsAny(a, "(["):
			// already formatted
			break
		case output.HasPackageCoverageMarking(a):
			a = strings.ReplaceAll(strings.ReplaceAll(a, "coverage: ", "[")+"]", "of statements", "coverage")

		default:
			a = "[" + a + "]"
		}

		aux[i] = p.Style.Aux.Render(a)
	}

	return strings.Join(append([]string{status, p.Name}, aux...), "\t") + p.Trailer
}
