package cover

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/cover"
)

// PackageResult holds per-package coverage calculated from a text coverage profile.
type PackageResult struct {
	PackagePath string
	Percent     float64
}

// FunctionResult holds per-function coverage parsed from `go tool cover -func` output.
type FunctionResult struct {
	FilePath string
	Line     int
	FuncName string
	Percent  float64
}

// PackageCoverage calculates per-package statement coverage from a text profile written by
// `go test -coverprofile`. This is the same statements-covered over statements-total math
// `go test` uses for its per-package lines, applied to the merged profile.
func PackageCoverage(profile string) ([]PackageResult, error) {
	profiles, err := cover.ParseProfiles(profile)
	if err != nil {
		return nil, fmt.Errorf("unable to parse coverage profile: %w", err)
	}

	type counts struct{ covered, total int64 }
	byPkg := make(map[string]*counts)
	for _, p := range profiles {
		pkg := path.Dir(p.FileName)
		c, ok := byPkg[pkg]
		if !ok {
			c = &counts{}
			byPkg[pkg] = c
		}
		for _, b := range p.Blocks {
			c.total += int64(b.NumStmt)
			if b.Count > 0 {
				c.covered += int64(b.NumStmt)
			}
		}
	}

	results := make([]PackageResult, 0, len(byPkg))
	for pkg, c := range byPkg {
		pct := 0.0
		if c.total > 0 {
			pct = 100 * float64(c.covered) / float64(c.total)
		}
		results = append(results, PackageResult{PackagePath: pkg, Percent: pct})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].PackagePath < results[j].PackagePath })

	return results, nil
}

// FunctionCoverage runs `go tool cover -func` on a text profile and returns per-function
// coverage plus the overall percentage from the total line. Using the go tool directly means
// the total is exactly what `go tool cover -func` reports for the same run.
//
// The expected output format is tab-separated:
//
//	package/path/file.go:12:	funcName	100.0%
//	total:					(statements)	41.1%
func FunctionCoverage(profile string) ([]FunctionResult, float64, error) {
	cmd := exec.Command("go", "tool", "cover", fmt.Sprintf("-func=%s", profile)) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("go tool cover -func failed: %w\n%s", err, stderr.String())
	}

	return parseFuncOutput(stdout.String())
}

// parseFuncOutput parses the tab-separated output of `go tool cover -func`.
func parseFuncOutput(output string) ([]FunctionResult, float64, error) {
	var results []FunctionResult
	var overallPercent float64

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// the total line has a different format: "total:\t(statements)\tNN.N%". Match the colon
		// so a module path starting with "total" isn't mistaken for it.
		if strings.HasPrefix(line, "total:") {
			pct, err := extractTrailingPercent(line)
			if err != nil {
				return nil, 0, fmt.Errorf("parsing total line %q: %w", line, err)
			}
			overallPercent = pct
			continue
		}

		fr, err := parseFuncLine(line)
		if err != nil {
			return nil, 0, fmt.Errorf("parsing func line %q: %w", line, err)
		}
		results = append(results, fr)
	}

	return results, overallPercent, nil
}

// parseFuncLine parses a single function coverage line.
// format: "package/path/file.go:12:\tfuncName\tNN.N%"
func parseFuncLine(line string) (FunctionResult, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return FunctionResult{}, fmt.Errorf("unexpected format: %q", line)
	}

	// first field is "file:line:" — parse file path and line number
	location := strings.TrimSuffix(fields[0], ":")

	lastColon := strings.LastIndex(location, ":")
	if lastColon < 0 {
		return FunctionResult{}, fmt.Errorf("no line number in %q", fields[0])
	}

	filePath := location[:lastColon]
	lineStr := location[lastColon+1:]
	lineNum, err := strconv.Atoi(lineStr)
	if err != nil {
		return FunctionResult{}, fmt.Errorf("invalid line number %q: %w", lineStr, err)
	}

	// second field is function name
	funcName := fields[1]

	// last field is percentage
	pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return FunctionResult{}, fmt.Errorf("invalid percent %q: %w", fields[len(fields)-1], err)
	}

	return FunctionResult{
		FilePath: filePath,
		Line:     lineNum,
		FuncName: funcName,
		Percent:  pct,
	}, nil
}

// extractTrailingPercent extracts a percentage from the last field of a line.
func extractTrailingPercent(line string) (float64, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty line")
	}

	last := fields[len(fields)-1]
	pctStr := strings.TrimSuffix(last, "%")
	return strconv.ParseFloat(pctStr, 64)
}
