package gostd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cienv"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestVerboseHandler_CIGrouping(t *testing.T) {
	tests := []struct {
		name           string
		ciGrouping     cienv.GroupConfig
		events         []gotest.Event
		wantGroupStart bool
		wantGroupEnd   bool
	}{
		{
			name: "grouping enabled for passed package",
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOn,
				GroupPassedPackages: true,
				GroupFailedPackages: false,
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
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOff,
				GroupPassedPackages: true,
				GroupFailedPackages: false,
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
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOn,
				GroupPassedPackages: true,
				GroupFailedPackages: false,
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
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOn,
				GroupPassedPackages: false,
				GroupFailedPackages: true,
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
				CIGrouping: tt.ciGrouping,
			})

			// Process all events
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
		ciGrouping     cienv.GroupConfig
		events         []gotest.Event
		wantGroupStart bool
		wantGroupEnd   bool
	}{
		{
			name: "grouping enabled for passed package",
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOn,
				GroupPassedPackages: true,
				GroupFailedPackages: false,
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
			ciGrouping: cienv.GroupConfig{
				Enabled:             cienv.ToggleOff,
				GroupPassedPackages: true,
				GroupFailedPackages: false,
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
				CIGrouping: tt.ciGrouping,
			})

			// Process all events
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
