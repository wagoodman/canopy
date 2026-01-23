package failure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestPanicParser_CanParse(t *testing.T) {
	parser := &panicParser{}

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name:   "panic prefix",
			input:  "panic: something went wrong",
			expect: true,
		},
		{
			name:   "panic in middle of output",
			input:  "some output\npanic: error occurred",
			expect: true,
		},
		{
			name:   "runtime error",
			input:  "runtime error: invalid memory address",
			expect: true,
		},
		{
			name:   "testify output",
			input:  "Error Trace: file.go:10",
			expect: false,
		},
		{
			name:   "normal output",
			input:  "test passed successfully",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.CanParse(tt.input)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestPanicParser_Parse(t *testing.T) {
	parser := &panicParser{}

	tests := []struct {
		name   string
		input  string
		expect *StructuredFailure
	}{
		{
			name: "simple panic",
			input: `panic: something went wrong

goroutine 1 [running]:
main.doSomething()
	/home/user/project/main.go:42 +0x100
main.main()
	/home/user/project/main.go:10 +0x50`,
			expect: &StructuredFailure{
				FailureType: PanicFailure,
				Panic: &PanicInfo{
					Message: "something went wrong",
					Frames: []StackFrame{
						{Function: "main.doSomething", File: "/home/user/project/main.go", Line: 42, IsUser: true},
						{Function: "main.main", File: "/home/user/project/main.go", Line: 10, IsUser: true},
					},
				},
				Location: SourceLocation{
					File: "/home/user/project/main.go",
					Line: 42,
				},
			},
		},
		{
			name: "runtime error nil pointer",
			input: `panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x10d1234]

goroutine 42 [running]:
github.com/example/pkg.HandleRequest(0xc0001234567)
	/home/user/project/handler.go:55 +0x100
runtime.gopanic(0x10a1234, 0xc000123456)
	/usr/local/go/src/runtime/panic.go:1000 +0x200`,
			expect: &StructuredFailure{
				FailureType: PanicFailure,
				Panic: &PanicInfo{
					Message: "runtime error: invalid memory address or nil pointer dereference",
					Frames: []StackFrame{
						{Function: "github.com/example/pkg.HandleRequest", File: "/home/user/project/handler.go", Line: 55, IsUser: true},
						{Function: "runtime.gopanic", File: "/usr/local/go/src/runtime/panic.go", Line: 1000, IsUser: false},
					},
				},
				Location: SourceLocation{
					File: "/home/user/project/handler.go",
					Line: 55,
				},
			},
		},
		{
			name: "multiline panic message",
			input: `panic: error occurred
with multiple lines
in the message

goroutine 1 [running]:
main.test()
	/project/main.go:20 +0x50`,
			expect: &StructuredFailure{
				FailureType: PanicFailure,
				Panic: &PanicInfo{
					Message: "error occurred\nwith multiple lines\nin the message",
					Frames: []StackFrame{
						{Function: "main.test", File: "/project/main.go", Line: 20, IsUser: true},
					},
				},
				Location: SourceLocation{
					File: "/project/main.go",
					Line: 20,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Parse(tt.input)
			require.NotNil(t, got)

			require.Equal(t, tt.expect.FailureType, got.FailureType)
			require.NotNil(t, got.Panic)
			require.Equal(t, tt.expect.Panic.Message, got.Panic.Message)

			// compare frames
			if diff := cmp.Diff(tt.expect.Panic.Frames, got.Panic.Frames); diff != "" {
				t.Errorf("frames mismatch (-want +got):\n%s", diff)
			}

			// check location (first user frame)
			require.Equal(t, tt.expect.Location.File, got.Location.File)
			require.Equal(t, tt.expect.Location.Line, got.Location.Line)

			// verify raw output preserved
			require.Equal(t, tt.input, got.RawOutput)
		})
	}
}

func TestPanicParser_IsUserCode(t *testing.T) {
	parser := &panicParser{}

	tests := []struct {
		name     string
		funcName string
		expect   bool
	}{
		{
			name:     "main package",
			funcName: "main.doSomething",
			expect:   true,
		},
		{
			name:     "user package",
			funcName: "github.com/user/pkg.Handler",
			expect:   true,
		},
		{
			name:     "runtime function",
			funcName: "runtime.gopanic",
			expect:   false,
		},
		{
			name:     "testing function",
			funcName: "testing.tRunner",
			expect:   false,
		},
		{
			name:     "reflect function",
			funcName: "reflect.Value.Call",
			expect:   false,
		},
		{
			name:     "empty function",
			funcName: "",
			expect:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.isUserCode(tt.funcName)
			require.Equal(t, tt.expect, got)
		})
	}
}
