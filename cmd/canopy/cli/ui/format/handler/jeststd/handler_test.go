package jeststd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestHandler_CIGrouping(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		events         []gotest.Event
		wantGroupStart bool
		wantGroupEnd   bool
	}{
		{
			name: "grouping enabled for passed package",
			config: Config{
				Grouping: group.Config{
					Formatter:   group.GitHub,
					GroupPassed: true,
					GroupFailed: false,
				},
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
			config: Config{
				Grouping: group.Config{
					Formatter:   nil,
					GroupPassed: true,
					GroupFailed: false,
				},
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
			config: Config{
				Grouping: group.Config{
					Formatter:   group.GitHub,
					GroupPassed: true,
					GroupFailed: false,
				},
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
			config: Config{
				Grouping: group.Config{
					Formatter:   group.GitHub,
					GroupPassed: false,
					GroupFailed: true,
				},
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
			h := NewHandler(&buf, tt.config)

			// process all events
			for _, e := range tt.events {
				err := h.(*jestHandler).OnGoTestEvent(e)
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

func TestHandler_JestStyleOutput(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		events      []gotest.Event
		wantStrings []string
	}{
		{
			name: "passed package shows PASS header",
			config: Config{
				Color: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}},
			},
			wantStrings: []string{" PASS ", "example.com/pkg"},
		},
		{
			name: "failed package shows FAIL header",
			config: Config{
				Color: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg"}},
			},
			wantStrings: []string{" FAIL ", "example.com/pkg"},
		},
		{
			name: "verbose mode shows test names with checkmarks",
			config: Config{
				Color:   false,
				Verbose: true,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.PassAction, Reference: gotest.Reference{Package: "example.com/pkg"}},
			},
			wantStrings: []string{"✔", "TestFoo"},
		},
		{
			name: "failed tests show X mark",
			config: Config{
				Color: false,
			},
			events: []gotest.Event{
				{Action: gotest.RunAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg", FuncName: "TestFoo"}},
				{Action: gotest.FailAction, Reference: gotest.Reference{Package: "example.com/pkg"}},
			},
			wantStrings: []string{"✕", "TestFoo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewHandler(&buf, tt.config)

			for _, e := range tt.events {
				err := h.(*jestHandler).OnGoTestEvent(e)
				assert.NoError(t, err)
			}

			output := buf.String()
			for _, want := range tt.wantStrings {
				assert.Contains(t, output, want, "expected %q in output:\n%s", want, output)
			}
		})
	}
}
