package gostd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestVerboseHandler_CIGrouping(t *testing.T) {
	tests := []struct {
		name           string
		groupConfig    group.Config
		events         []gotest.Event
		wantGroupStart bool
		wantGroupEnd   bool
	}{
		{
			name: "grouping enabled for passed package",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- PASS: TestFoo (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.01s\n"},
			},
			wantGroupStart: true,
			wantGroupEnd:   true,
		},
		{
			name: "grouping disabled",
			groupConfig: group.Config{
				Formatter:   nil,
				GroupPassed: true,
				GroupFailed: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- PASS: TestFoo (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.01s\n"},
			},
			wantGroupStart: false,
			wantGroupEnd:   false,
		},
		{
			name: "grouping enabled but failed package not grouped",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- FAIL: TestFoo (0.01s)\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "FAIL\texample.com/pkg\t0.01s\n"},
			},
			wantGroupStart: false,
			wantGroupEnd:   false,
		},
		{
			name: "grouping enabled for failed package",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: false,
				GroupFailed: true,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- FAIL: TestFoo (0.01s)\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "FAIL\texample.com/pkg\t0.01s\n"},
			},
			wantGroupStart: true,
			wantGroupEnd:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewVerboseHandler(&buf, PackageConfig{
				Grouping: tt.groupConfig,
			})

			// process all events
			for _, e := range tt.events {
				err := h.(*verboseHandler).OnGoTestEvent(e)
				assert.NoError(t, err)
			}

			output := buf.String()

			if tt.wantGroupStart {
				assert.True(t, strings.Contains(output, "::group::"), "expected ::group:: marker in output:\n%s", output)
			} else {
				assert.False(t, strings.Contains(output, "::group::"), "did not expect ::group:: marker in output:\n%s", output)
			}

			if tt.wantGroupEnd {
				assert.True(t, strings.Contains(output, "::endgroup::"), "expected ::endgroup:: marker in output:\n%s", output)
			} else {
				assert.False(t, strings.Contains(output, "::endgroup::"), "did not expect ::endgroup:: marker in output:\n%s", output)
			}
		})
	}
}

func TestVerboseHandler_AcrossTestsGrouping(t *testing.T) {
	tests := []struct {
		name                    string
		groupConfig             group.Config
		events                  []gotest.Event
		wantGroupCount          int    // number of ::group:: markers expected
		wantPassingTestsGrouped bool   // whether "passing tests" group is expected
		wantGroupTitle          string // expected group title if wantPassingTestsGrouped
	}{
		{
			name: "consecutive passing tests grouped when AcrossTests enabled",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
				AcrossTests: true,
			},
			events: []gotest.Event{
				// RUN events
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "=== RUN   Test2\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "=== RUN   Test3\n"},
				// PASS events (3 consecutive passes)
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "--- PASS: Test2 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "--- PASS: Test3 (0.01s)\n"},
				// package pass
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.03s\n"},
			},
			// package passes → package is grouped. AcrossTests grouping is skipped to avoid nesting.
			wantGroupCount:          1, // only package group (test grouping skipped to avoid nesting)
			wantPassingTestsGrouped: false,
		},
		{
			name: "consecutive passing tests with failure in middle - two groups",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
				AcrossTests: true,
			},
			events: []gotest.Event{
				// RUN events
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "=== RUN   Test2\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "=== RUN   Test3\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test4"}, Output: "=== RUN   Test4\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test5"}, Output: "=== RUN   Test5\n"},
				// PASS, PASS, FAIL, PASS, PASS
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "--- PASS: Test2 (0.01s)\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "--- FAIL: Test3 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test4"}, Output: "--- PASS: Test4 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test5"}, Output: "--- PASS: Test5 (0.01s)\n"},
				// package fail
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "FAIL\texample.com/pkg\t0.05s\n"},
			},
			wantGroupCount:          2, // two groups of consecutive passed tests (no package group since failed)
			wantPassingTestsGrouped: true,
			wantGroupTitle:          "passed tests",
		},
		{
			name: "single passing test not grouped",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
				AcrossTests: true,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.01s\n"},
			},
			wantGroupCount:          1, // only package group, single test not grouped
			wantPassingTestsGrouped: false,
		},
		{
			name: "AcrossTests disabled - no consecutive grouping",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
				AcrossTests: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "=== RUN   Test2\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "=== RUN   Test3\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test2"}, Output: "--- PASS: Test2 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "Test3"}, Output: "--- PASS: Test3 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.03s\n"},
			},
			wantGroupCount:          1, // only package group
			wantPassingTestsGrouped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewVerboseHandler(&buf, PackageConfig{
				Grouping: tt.groupConfig,
			})

			// process all events
			for _, e := range tt.events {
				err := h.(*verboseHandler).OnGoTestEvent(e)
				assert.NoError(t, err)
			}

			output := buf.String()

			// count ::group:: markers
			groupCount := strings.Count(output, "::group::")
			assert.Equal(t, tt.wantGroupCount, groupCount, "unexpected group count in output:\n%s", output)

			if tt.wantPassingTestsGrouped {
				assert.Contains(t, output, tt.wantGroupTitle, "expected group title %q in output:\n%s", tt.wantGroupTitle, output)
			} else {
				assert.NotContains(t, output, "passed tests", "did not expect 'passed tests' group in output:\n%s", output)
			}
		})
	}
}

func TestVerboseHandler_AcrossPackagesGrouping(t *testing.T) {
	tests := []struct {
		name                       string
		groupConfig                group.Config
		events                     []gotest.Event
		wantGroupCount             int    // number of ::group:: markers expected
		wantPassingPackagesGrouped bool   // whether "passing packages" group is expected
		wantGroupTitle             string // expected group title if wantPassingPackagesGrouped
	}{
		{
			name: "consecutive passing packages grouped when AcrossPackages enabled",
			groupConfig: group.Config{
				Formatter:      group.GitHub,
				GroupPassed:    true,
				GroupFailed:    false,
				AcrossPackages: true,
			},
			// packages complete in reverse alphabetical order, so they accumulate
			// before being flushed when the first (alphabetically) package completes
			events: []gotest.Event{
				// packages start (adds them to h.packages in alphabetical order)
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg2"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg3"}, Output: ""},
				// all tests run
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				// tests pass
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				// packages complete in reverse order: pkg3, pkg2, pkg1
				// when pkg3 completes, render blocks on pkg1 (not yet complete)
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg3"}, Output: "ok  \texample.com/pkg3\t0.01s\n"},
				// when pkg2 completes, render still blocks on pkg1
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2"}, Output: "ok  \texample.com/pkg2\t0.01s\n"},
				// when pkg1 completes, all 3 packages are done and grouped together
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: "ok  \texample.com/pkg1\t0.01s\n"},
			},
			wantGroupCount:             1, // all 3 packages grouped together
			wantPassingPackagesGrouped: true,
			wantGroupTitle:             "passed packages", // streaming uses count-less title
		},
		{
			name: "consecutive passing packages with failure in middle",
			groupConfig: group.Config{
				Formatter:      group.GitHub,
				GroupPassed:    true,
				GroupFailed:    false,
				AcrossPackages: true,
			},
			// packages complete in reverse alphabetical order within each group
			// pkg1, pkg2 (pass), pkg3 (fail), pkg4, pkg5 (pass)
			events: []gotest.Event{
				// packages start (adds them to h.packages in alphabetical order)
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg2"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg3"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg4"}, Output: ""},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg5"}, Output: ""},
				// all tests run
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg4", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg5", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				// tests complete
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "--- FAIL: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg5", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg4", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				// packages complete: pkg2 (blocked by pkg1), pkg1 (flush pkg1+pkg2), pkg3 (fail, flush, no group), pkg5 (blocked), pkg4 (flush pkg4+pkg5)
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2"}, Output: "ok  \texample.com/pkg2\t0.01s\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: "ok  \texample.com/pkg1\t0.01s\n"},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg3"}, Output: "FAIL\texample.com/pkg3\t0.01s\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg5"}, Output: "ok  \texample.com/pkg5\t0.01s\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg4"}, Output: "ok  \texample.com/pkg4\t0.01s\n"},
			},
			wantGroupCount:             2, // two groups of consecutive passed packages
			wantPassingPackagesGrouped: true,
			wantGroupTitle:             "passed packages", // streaming uses count-less title
		},
		{
			name: "single passing package uses streaming group",
			groupConfig: group.Config{
				Formatter:      group.GitHub,
				GroupPassed:    true,
				GroupFailed:    false,
				AcrossPackages: true,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: "ok  \texample.com/pkg1\t0.01s\n"},
			},
			// streaming groups single packages the same way as multiple - consistent format
			wantGroupCount:             1,
			wantPassingPackagesGrouped: true,
			wantGroupTitle:             "passed packages",
		},
		{
			name: "AcrossPackages disabled - individual package grouping",
			groupConfig: group.Config{
				Formatter:      group.GitHub,
				GroupPassed:    true,
				GroupFailed:    false,
				AcrossPackages: false,
			},
			events: []gotest.Event{
				// package 1
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg1"}, Output: "ok  \texample.com/pkg1\t0.01s\n"},
				// package 2
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg2"}, Output: "ok  \texample.com/pkg2\t0.01s\n"},
				// package 3
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "=== RUN   Test1\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg3", FuncName: "Test1"}, Output: "--- PASS: Test1 (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg3"}, Output: "ok  \texample.com/pkg3\t0.01s\n"},
			},
			wantGroupCount:             3, // each package gets its own group
			wantPassingPackagesGrouped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewVerboseHandler(&buf, PackageConfig{
				Grouping: tt.groupConfig,
			})

			// process all events
			for _, e := range tt.events {
				err := h.(*verboseHandler).OnGoTestEvent(e)
				assert.NoError(t, err)
			}

			output := buf.String()

			// count ::group:: markers
			groupCount := strings.Count(output, "::group::")
			assert.Equal(t, tt.wantGroupCount, groupCount, "unexpected group count in output:\n%s", output)

			if tt.wantPassingPackagesGrouped {
				assert.Contains(t, output, tt.wantGroupTitle, "expected group title %q in output:\n%s", tt.wantGroupTitle, output)
			} else {
				assert.NotContains(t, output, "passed packages", "did not expect 'passed packages' group in output:\n%s", output)
			}
		})
	}
}

func TestQuietHandler_CIGrouping(t *testing.T) {
	tests := []struct {
		name           string
		groupConfig    group.Config
		events         []gotest.Event
		wantGroupStart bool
		wantGroupEnd   bool
	}{
		{
			name: "grouping enabled for passed package",
			groupConfig: group.Config{
				Formatter:   group.GitHub,
				GroupPassed: true,
				GroupFailed: false,
			},
			// quiet handler only outputs package summary, so single-line output is not grouped.
			// This test verifies that single-line output is passed through without group markers.
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- PASS: TestFoo (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.01s\n"},
			},
			// single-line output skips grouping (not worth collapsing)
			wantGroupStart: false,
			wantGroupEnd:   false,
		},
		{
			name: "grouping disabled",
			groupConfig: group.Config{
				Formatter:   nil,
				GroupPassed: true,
				GroupFailed: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "=== RUN   TestFoo\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}, Output: "--- PASS: TestFoo (0.01s)\n"},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}, Output: "ok  \texample.com/pkg\t0.01s\n"},
			},
			wantGroupStart: false,
			wantGroupEnd:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewQuietHandler(&buf, PackageConfig{
				Grouping: tt.groupConfig,
			})

			// process all events
			for _, e := range tt.events {
				err := h.(*quietHandler).OnGoTestEvent(e)
				assert.NoError(t, err)
			}

			output := buf.String()

			if tt.wantGroupStart {
				assert.True(t, strings.Contains(output, "::group::"), "expected ::group:: marker in output:\n%s", output)
			} else {
				assert.False(t, strings.Contains(output, "::group::"), "did not expect ::group:: marker in output:\n%s", output)
			}

			if tt.wantGroupEnd {
				assert.True(t, strings.Contains(output, "::endgroup::"), "expected ::endgroup:: marker in output:\n%s", output)
			} else {
				assert.False(t, strings.Contains(output, "::endgroup::"), "did not expect ::endgroup:: marker in output:\n%s", output)
			}
		})
	}
}
