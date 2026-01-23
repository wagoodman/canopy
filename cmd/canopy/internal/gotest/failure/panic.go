package failure

import (
	"regexp"
	"strconv"
	"strings"
)

// panicParser parses panic stack traces from test output.
type panicParser struct{}

var (
	// goroutinePattern matches "goroutine N [state]:" headers.
	goroutinePattern = regexp.MustCompile(`^goroutine \d+`)
	// stackFilePattern matches stack file lines like "\t/path/file.go:123 +0x..."
	stackFilePattern = regexp.MustCompile(`^\t(.+\.go):(\d+)`)
	// stackFuncPattern matches function lines like "package.func(args)" and extracts just the function name
	stackFuncPattern = regexp.MustCompile(`^([^\t]+)\(.*\)$`)
)

// runtime and test framework package prefixes to identify non-user code
var runtimePrefixes = []string{
	"runtime.",
	"runtime/",
	"testing.",
	"reflect.",
	"sync.",
	"syscall.",
	"internal/",
}

func (p *panicParser) Name() string {
	return "panic"
}

func (p *panicParser) CanParse(output string) bool {
	return strings.HasPrefix(output, "panic:") ||
		strings.Contains(output, "\npanic:") ||
		strings.Contains(output, "runtime error:")
}

func (p *panicParser) Parse(output string) *StructuredFailure {
	sf := &StructuredFailure{
		FailureType: PanicFailure,
		RawOutput:   output,
		Panic: &PanicInfo{
			Version: PanicInfoVersion,
		},
	}

	lines := strings.Split(output, "\n")

	// extract panic message
	sf.Panic.Message = p.extractPanicMessage(lines)

	// extract stack frames
	sf.Panic.Frames = p.extractStackFrames(lines)

	// set location from first user frame if available
	for _, frame := range sf.Panic.Frames {
		if frame.IsUser {
			sf.Location = SourceLocation{
				File: frame.File,
				Line: frame.Line,
			}
			break
		}
	}

	return sf
}

// extractPanicMessage extracts the panic message from the output lines.
func (p *panicParser) extractPanicMessage(lines []string) string {
	var message strings.Builder
	var inPanicMessage bool

	for _, line := range lines {
		if strings.HasPrefix(line, "panic:") {
			inPanicMessage = true
			msg := strings.TrimPrefix(line, "panic:")
			msg = strings.TrimSpace(msg)
			message.WriteString(msg)
			continue
		}

		if inPanicMessage {
			// panic messages can span multiple lines until we hit a goroutine line, stack frame, or signal line
			if goroutinePattern.MatchString(line) ||
				strings.HasPrefix(line, "\t") ||
				stackFuncPattern.MatchString(line) ||
				strings.HasPrefix(line, "[signal ") {
				break
			}
			// append continuation lines
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if message.Len() > 0 {
					message.WriteString("\n")
				}
				message.WriteString(trimmed)
			}
		}
	}

	return message.String()
}

// extractStackFrames parses stack trace lines into structured frames.
func (p *panicParser) extractStackFrames(lines []string) []StackFrame {
	var frames []StackFrame
	var currentFunc string

	// find the start of the stack trace (goroutine line)
	var inStack bool
	for _, line := range lines {
		if goroutinePattern.MatchString(line) {
			inStack = true
			continue
		}

		if !inStack {
			continue
		}

		// check if this is a function line
		if matches := stackFuncPattern.FindStringSubmatch(line); len(matches) >= 2 {
			currentFunc = matches[1]
			continue
		}

		// check if this is a file line
		if matches := stackFilePattern.FindStringSubmatch(line); len(matches) >= 3 {
			lineNum, _ := strconv.Atoi(matches[2])
			frame := StackFrame{
				Function: currentFunc,
				File:     matches[1],
				Line:     lineNum,
				IsUser:   p.isUserCode(currentFunc),
			}
			frames = append(frames, frame)
			currentFunc = ""
		}
	}

	return frames
}

// isUserCode determines if a function is user code vs runtime/framework code.
func (p *panicParser) isUserCode(funcName string) bool {
	if funcName == "" {
		return false
	}

	for _, prefix := range runtimePrefixes {
		if strings.HasPrefix(funcName, prefix) {
			return false
		}
	}

	// also check for common test framework functions
	if strings.Contains(funcName, "testing.tRunner") ||
		strings.Contains(funcName, "testing.(*T)") ||
		strings.Contains(funcName, "testing.(*M)") {
		return false
	}

	return true
}
