package cover

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PackageResult holds per-package coverage data parsed from `go tool covdata percent` output.
type PackageResult struct {
	PackagePath string
	Percent     float64
}

// FunctionResult holds per-function coverage data parsed from `go tool covdata func` output.
type FunctionResult struct {
	FilePath string
	Line     int
	FuncName string
	Percent  float64
}

// PackageCoverage runs `go tool covdata percent` on a binary coverage directory
// and returns per-package coverage results.
//
// The expected output format is tab-separated:
//
//	package/path		coverage: 41.1% of statements
func PackageCoverage(coverDir string) ([]PackageResult, error) {
	cmd := exec.Command("go", "tool", "covdata", "percent", fmt.Sprintf("-i=%s", coverDir))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("covdata percent failed: %w\n%s", err, stderr.String())
	}

	return parsePercentOutput(stdout.String())
}

// FunctionCoverage runs `go tool covdata func` on a binary coverage directory
// and returns per-function coverage results plus the overall percentage from the total line.
//
// The expected output format is tab-separated:
//
//	package/path/file.go:12:	funcName	100.0%
//	total					(statements)	41.1%
func FunctionCoverage(coverDir string) ([]FunctionResult, float64, error) {
	cmd := exec.Command("go", "tool", "covdata", "func", fmt.Sprintf("-i=%s", coverDir))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("covdata func failed: %w\n%s", err, stderr.String())
	}

	return parseFuncOutput(stdout.String())
}

// parsePercentOutput parses the tab-separated output of `go tool covdata percent`.
func parsePercentOutput(output string) ([]PackageResult, error) {
	var results []PackageResult

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// format: "package/path\tcoverage: NN.N% of statements"
		pkgPath, pct, err := parsePercentLine(line)
		if err != nil {
			return nil, fmt.Errorf("parsing percent line %q: %w", line, err)
		}

		results = append(results, PackageResult{
			PackagePath: pkgPath,
			Percent:     pct,
		})
	}

	return results, nil
}

// parsePercentLine parses a single line from covdata percent output.
func parsePercentLine(line string) (string, float64, error) {
	// split on tab to separate package path from coverage info
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) < 2 {
		// try splitting on whitespace if no tabs
		parts = strings.Fields(line)
		if len(parts) < 3 {
			return "", 0, fmt.Errorf("unexpected format: %q", line)
		}
	}

	pkgPath := strings.TrimSpace(parts[0])

	// extract percentage from "coverage: NN.N% of statements"
	rest := parts[len(parts)-1]
	pct, err := extractPercent(rest)
	if err != nil {
		return "", 0, err
	}

	return pkgPath, pct, nil
}

// parseFuncOutput parses the tab-separated output of `go tool covdata func`.
func parseFuncOutput(output string) ([]FunctionResult, float64, error) {
	var results []FunctionResult
	var overallPercent float64

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// the total line has a different format: "total\t(statements)\tNN.N%"
		if strings.HasPrefix(line, "total") || strings.HasPrefix(line, "total:") {
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

// extractPercent pulls a percentage from a string like "coverage: 41.1% of statements".
func extractPercent(s string) (float64, error) {
	idx := strings.Index(s, "%")
	if idx < 0 {
		return 0, fmt.Errorf("no %% found in %q", s)
	}

	// walk backwards from the % to find the start of the number
	start := idx - 1
	for start >= 0 && (s[start] == '.' || (s[start] >= '0' && s[start] <= '9')) {
		start--
	}
	start++

	if start >= idx {
		return 0, fmt.Errorf("no number before %% in %q", s)
	}

	return strconv.ParseFloat(s[start:idx], 64)
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
