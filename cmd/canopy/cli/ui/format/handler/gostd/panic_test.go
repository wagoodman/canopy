package gostd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// a panicking test binary reaches canopy in one of two shapes, and only the first one has a failing
// *test* to hang the failure off of:
//
//	direct     the test body panics, testing recovers long enough to print "--- FAIL: TestX" and emit
//	           a fail action for the test, then the process dies
//	goroutine  the panic is on another goroutine, so testing never gets to conclude anything: the test
//	           gets a run action, the panic text, and nothing else. Only the package fails. A timeout
//	           or a fatal runtime error (concurrent map write, stack exhaustion) lands here too.
func directPanicEvents() []gotest.Event {
	pkg := gotest.NewReference("example.com/m/direct", "")
	test := gotest.NewReference("example.com/m/direct", "TestDirectPanic")
	return []gotest.Event{
		{Action: gotest.RunAction, Reference: test, Output: "=== RUN   TestDirectPanic\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "--- FAIL: TestDirectPanic (0.00s)\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "panic: boom in the test body [recovered, repanicked]\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "goroutine 34 [running]:\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "example.com/m/direct.TestDirectPanic(0x14000102340)\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "\td/d_test.go:5 +0x2c\n"},
		{Action: gotest.FailAction, Reference: test},
		{Action: gotest.OutputAction, Reference: pkg, Output: "FAIL\texample.com/m/direct\t0.29s\n"},
		{Action: gotest.FailAction, Reference: pkg},
	}
}

func goroutinePanicEvents() []gotest.Event {
	pkg := gotest.NewReference("example.com/m/goroutine", "")
	test := gotest.NewReference("example.com/m/goroutine", "TestGoroutinePanic")
	return []gotest.Event{
		{Action: gotest.RunAction, Reference: test, Output: "=== RUN   TestGoroutinePanic\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "panic: boom off the test goroutine\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "goroutine 5 [running]:\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "example.com/m/goroutine.TestGoroutinePanic.func1()\n"},
		{Action: gotest.OutputAction, Reference: test, Output: "\tg/g_test.go:6 +0x2c\n"},
		// no conclusion for the test: the binary died under it
		{Action: gotest.OutputAction, Reference: pkg, Output: "FAIL\texample.com/m/goroutine\t0.35s\n"},
		{Action: gotest.FailAction, Reference: pkg},
	}
}

// TestResult_Passed_Panics is the guard on the exit code: canopy exits on Passed(), so a panicking
// package reading as a pass makes a dead test suite look green to CI.
func TestResult_Passed_Panics(t *testing.T) {
	for _, tt := range []struct {
		name   string
		events []gotest.Event
	}{
		{"panic in the test body", directPanicEvents()},
		{"panic on another goroutine", goroutinePanicEvents()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := gotest.NewResult(gotest.ResultConfig{})

			// a healthy package alongside it, so the passing tests have a chance to mask the failure
			result.Update(gotest.Event{Action: gotest.PassAction, Reference: gotest.NewReference("example.com/m/good", "TestGood")})
			result.Update(gotest.Event{Action: gotest.PassAction, Reference: gotest.NewReference("example.com/m/good", "")})
			require.True(t, result.Passed())

			for _, e := range tt.events {
				result.Update(e)
			}

			assert.False(t, result.Passed())
		})
	}
}

// TestHandlers_PanicOutput is the guard on the diagnosis: the trace is the only thing explaining why
// the run died, so dropping it leaves a bare "FAIL <pkg>" and nothing to act on.
func TestHandlers_PanicOutput(t *testing.T) {
	for _, tt := range []struct {
		name    string
		events  []gotest.Event
		message string
		trace   string
	}{
		{"panic in the test body", directPanicEvents(), "boom in the test body", "example.com/m/direct.TestDirectPanic"},
		{"panic on another goroutine", goroutinePanicEvents(), "boom off the test goroutine", "example.com/m/goroutine.TestGoroutinePanic.func1()"},
	} {
		for _, h := range []struct {
			name string
			new  func(*bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error }
		}{
			{"quiet", func(b *bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error } {
				return NewQuietHandler(b, PackageConfig{}).(*quietHandler)
			}},
			{"verbose", func(b *bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error } {
				return NewVerboseHandler(b, PackageConfig{}).(*verboseHandler)
			}},
		} {
			t.Run(tt.name+"/"+h.name, func(t *testing.T) {
				var buf bytes.Buffer
				handler := h.new(&buf)
				for _, e := range tt.events {
					require.NoError(t, handler.OnGoTestEvent(e))
				}
				out := buf.String()

				// the handler restyles the "panic:" marker itself, so match on the message it carries
				assert.Contains(t, out, tt.message, "the panic message is missing:\n%s", out)
				assert.Contains(t, out, tt.trace, "the stack frame naming the culprit is missing:\n%s", out)
				assert.True(t, strings.Contains(out, "FAIL"), "the package failure is missing:\n%s", out)
			})
		}
	}
}
