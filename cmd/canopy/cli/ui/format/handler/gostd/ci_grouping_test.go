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
