package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasTestPassMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"--- PASS: TestExample", true},
		{"   --- PASS: TestExample", true},
		{"--- FAIL: TestExample", false},
		{"PASS: TestExample", false},
		{"", false},
		{"--- PASS:TestExample", true}, // Even without a space, it should detect
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasTestPassMarking(tt.output)
			assert.Equal(t, tt.expected, result, "Output: %q", tt.output)
		})
	}
}

func TestHasShuffleSeedMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"-test.shuffle 42", true},
		{"-test.shuffle 1784084924287870000\n", true},
		{"   -test.shuffle 42\n", true},
		{"-test.shufflefoo", false}, // no space delimiter, not the seed line
		{"ok      cmd/canopy/cli/commands   0.978s", false},
		{"--- FAIL: TestExample", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasShuffleSeedMarking(tt.output)
			assert.Equal(t, tt.expected, result, "Output: %q", tt.output)
		})
	}
}

func TestHasPackageCoverageMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"coverage: 75.0% of statements", true},
		{" coverage: 75.0% of statements", true},
		{"PASS coverage: 75.0% of statements", false},
		{"", false},
	}

	for _, tt := range tests {

		t.Run(tt.output, func(t *testing.T) {
			result := HasPackageCoverageMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasPassedPackageMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"ok  	github.com/example/repo	0.002s", true},
		{"ok\tgithub.com/example/repo\t0.002s", true},
		{"FAIL\tgithub.com/example/repo\t0.002s", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasPackageOKMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasUnknownPackageMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"?   	github.com/example/repo", true},
		{"\tgithub.com/example/repo", true},
		{"ok  	github.com/example/repo	0.002s", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasUnknownPackageMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasPackagePassMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"PASS", true},
		{"PASS some test", true},
		{"PASS  coverage: 75.0% of statements", true},
		{"FAIL", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasPackagePassMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasRunMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"=== RUN   TestExample", true},
		{"=== RUN", true},
		{"=== ", false},
		{"RUN TestExample", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasRunMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasFailedTestMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"--- FAIL: TestExample", true},
		{"--- FAIL", false},
		{"FAIL: TestExample", false},
		{"FAIL", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasFailedTestMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasFailedPackageMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"FAIL	github.com/example/repo	0.002s", true},
		{"FAIL	", true},
		{"ok  	github.com/example/repo	0.002s", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := HasFailedPackageMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLogLine(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"palindrome_test.go:51: this is a log line", true},
		{"  main.go:123: another log line", true},
		{"not_a_log_line", false},
		{" random text palindrome_test.go:51: not a log line", false},
		{"", false},
		{"file.go: no line number", false},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := IsLogLine(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldShowStateMarker(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		mode     string
		expected bool
	}{
		// non-state markers should always be shown
		{
			name:     "non-state marker with none mode",
			line:     "some regular output",
			mode:     ExecutionMarkersNone,
			expected: true,
		},
		{
			name:     "non-state marker with all mode",
			line:     "some regular output",
			mode:     ExecutionMarkersAll,
			expected: true,
		},
		// mode: none - hide all state markers
		{
			name:     "RUN marker with none mode",
			line:     "=== RUN   TestExample",
			mode:     ExecutionMarkersNone,
			expected: false,
		},
		{
			name:     "PAUSE marker with none mode",
			line:     "=== PAUSE TestExample",
			mode:     ExecutionMarkersNone,
			expected: false,
		},
		{
			name:     "CONT marker with none mode",
			line:     "=== CONT  TestExample",
			mode:     ExecutionMarkersNone,
			expected: false,
		},
		// mode: all - show all state markers
		{
			name:     "RUN marker with all mode",
			line:     "=== RUN   TestExample",
			mode:     ExecutionMarkersAll,
			expected: true,
		},
		{
			name:     "PAUSE marker with all mode",
			line:     "=== PAUSE TestExample",
			mode:     ExecutionMarkersAll,
			expected: true,
		},
		{
			name:     "CONT marker with all mode",
			line:     "=== CONT  TestExample",
			mode:     ExecutionMarkersAll,
			expected: true,
		},
		// mode: parallel-only - show only PAUSE/CONT markers
		{
			name:     "RUN marker with parallel-only mode",
			line:     "=== RUN   TestExample",
			mode:     ExecutionMarkersParallelOnly,
			expected: false,
		},
		{
			name:     "PAUSE marker with parallel-only mode",
			line:     "=== PAUSE TestExample",
			mode:     ExecutionMarkersParallelOnly,
			expected: true,
		},
		{
			name:     "CONT marker with parallel-only mode",
			line:     "=== CONT  TestExample",
			mode:     ExecutionMarkersParallelOnly,
			expected: true,
		},
		// unknown mode defaults to hiding
		{
			name:     "RUN marker with unknown mode",
			line:     "=== RUN   TestExample",
			mode:     "invalid",
			expected: false,
		},
		{
			name:     "empty mode defaults to hiding",
			line:     "=== RUN   TestExample",
			mode:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldShowStateMarker(tt.line, tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}
