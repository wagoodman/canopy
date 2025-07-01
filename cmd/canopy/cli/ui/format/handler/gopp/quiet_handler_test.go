package gopp

import (
	"errors"
	"fmt"
	error2 "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestDefaultHandler(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{
			name:    "go1.21.3",
			fixture: "full/go1.21.3-default.jsonl",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := QuietPackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}

			for _, b := range []bool{true, false} {
				t.Run(fmt.Sprintf("hide-no-tests=%v", b), func(t *testing.T) {
					cfg.HidePackagesWithNoTestFiles = b

					subject := NewQuietHandler(&sb, cfg)
					events := fixtureEvents(t, tt.fixture)
					for e := range events {
						err := subject.OnGoTestEvent(e)
						if errors.Is(err, error2.ErrPackageComplete) {
							// this one is OK to ignore
							continue
						}
						require.NoError(t, err)
					}

					snaps.MatchSnapshot(t, sb.String())
				})
			}
		})
	}
}

func TestDefaultPackage(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		ref     gotest.Reference
	}{
		{
			name:    "failing package",
			fixture: "mixed-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/weird.d",
				FuncName: "TestAddFailingSubtest",
				TRunName: "Test_weird_numbers_(oops)/offset=2",
			},
		},
		{
			name:    "passing package",
			fixture: "mixed-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/cmd/canopy/internal/gotest",
				FuncName: "Test_dfsTreeIterator_Next",
				TRunName: "duplicate_case",
			},
		},
		{
			name:    "panic package",
			fixture: "panic-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/panic",
				FuncName: "TestPanic",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := QuietPackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}
			subject := NewQuietPackage(&sb, cfg, tt.ref)
			events := fixtureEvents(t, tt.fixture)
			for e := range events {
				err := subject.OnGoTestEvent(e)
				if errors.Is(err, error2.ErrPackageComplete) {
					// this one is OK to ignore
					continue
				}
				require.NoError(t, err)
			}

			output := sb.String() // usecase: to stdout
			snaps.MatchSnapshot(t, output)
			assert.Equal(t, output, subject.String()) // usecase: studio UI

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
			result := output.HasPackageCoverageMarking(tt.output)
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
			result := output.HasPassedPackageMarking(tt.output)
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
			result := output.HasUnknownPackageMarking(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasPassMarking(t *testing.T) {
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
			result := output.HasPassMarking(tt.output)
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
			result := output.HasRunMarking(tt.output)
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
			result := output.HasFailedTestMarking(tt.output)
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
			result := output.HasFailedPackageMarking(tt.output)
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
			result := output.IsLogLine(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitWhitespace(t *testing.T) {
	tests := []struct {
		input           string
		expectedPrefix  string
		expectedContent string
	}{
		{"   leading spaces", "   ", "leading spaces"},
		{"\t\ttabbed content", "\t\t", "tabbed content"},
		{"\n\rnewlines", "\n\r", "newlines"},
		{"noWhitespace", "", "noWhitespace"},
		{"  mixed whitespace\n\tand content", "  ", "mixed whitespace\n\tand content"},
		{"", "", ""},
		{"   ", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			prefix, content := splitWhitespace(tt.input)
			assert.Equal(t, tt.expectedPrefix, prefix, "Prefix should match expected")
			assert.Equal(t, tt.expectedContent, content, "Content should match expected")
		})
	}
}
