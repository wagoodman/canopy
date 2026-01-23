// Package failure provides parsing of test failure output into structured data.
// This enables rich queries, better visualization, and AI-friendly failure context
// by transforming raw test output into queryable structures.
package failure

// Type categorizes the kind of test failure that occurred.
type Type string

const (
	// AssertionFailure indicates a failed assertion (testify, stdlib t.Error, etc.).
	AssertionFailure Type = "assertion"
	// PanicFailure indicates a panic or runtime error occurred during the test.
	PanicFailure Type = "panic"
	// DiffFailure indicates a cmp.Diff or similar diff-based comparison failure.
	DiffFailure Type = "diff"
	// TimeoutFailure indicates the test timed out.
	TimeoutFailure Type = "timeout"
	// UnknownFailure is used when the failure type cannot be determined.
	UnknownFailure Type = "unknown"
)

// StructuredFailure is the unified representation of any parsed test failure.
// It contains type-specific information based on the failure category.
type StructuredFailure struct {
	// FailureType categorizes the failure (assertion, panic, diff, timeout, unknown).
	FailureType Type `json:"type"`
	// Assertion contains details for assertion failures (nil for other types).
	Assertion *AssertionInfo `json:"assertion,omitempty"`
	// Panic contains details for panic/runtime errors (nil for other types).
	Panic *PanicInfo `json:"panic,omitempty"`
	// Diff contains details for diff-based failures (nil for other types).
	Diff *DiffInfo `json:"diff,omitempty"`
	// Location is the source file location where the failure occurred.
	Location SourceLocation `json:"location"`
	// RawOutput preserves the original failure output text.
	RawOutput string `json:"raw_output"`
	// Fingerprint is a semantic hash for identifying distinct failure modes.
	// This enables flaky test detection by grouping similar failures.
	Fingerprint string `json:"fingerprint"`
}

// AssertionInfo contains details extracted from assertion library failures.
type AssertionInfo struct {
	// Version is the schema version for migration support.
	Version int `json:"version"`
	// Library identifies the assertion library (e.g., "testify", "stdlib").
	Library string `json:"library"`
	// Function is the assertion function name (e.g., "Equal", "Contains", "Error").
	Function string `json:"function"`
	// Expected is the expected value as a string representation.
	Expected string `json:"expected"`
	// Actual is the actual value that was received.
	Actual string `json:"actual"`
	// Message is the custom message provided with the assertion, if any.
	Message string `json:"message,omitempty"`
}

const AssertionInfoVersion = 1

// PanicInfo contains details extracted from panic stack traces.
type PanicInfo struct {
	// Version is the schema version for migration support.
	Version int `json:"version"`
	// Message is the panic message or runtime error description.
	Message string `json:"message"`
	// Frames is the stack trace, ordered from most recent to oldest.
	Frames []StackFrame `json:"frames"`
}

const PanicInfoVersion = 1

// StackFrame represents a single frame in a panic stack trace.
type StackFrame struct {
	// Function is the fully qualified function name.
	Function string `json:"function"`
	// File is the source file path.
	File string `json:"file"`
	// Line is the line number in the source file.
	Line int `json:"line"`
	// IsUser is true if this is user code, false for runtime/stdlib/testing code.
	IsUser bool `json:"is_user"`
}

// DiffInfo contains details extracted from cmp.Diff or similar diff output.
type DiffInfo struct {
	// Version is the schema version for migration support.
	Version int `json:"version"`
	// Chunks is a sequence of diff lines preserving order and interleaving.
	Chunks []DiffChunks `json:"chunks"`
}

const DiffInfoVersion = 1

// DiffChunkType categorizes a line or set of lines in a diff.
type DiffChunkType string

const (
	// DiffChunkContext indicates an unchanged line providing context.
	DiffChunkContext DiffChunkType = "context"
	// DiffChunkAddition indicates a line present in actual but not expected.
	DiffChunkAddition DiffChunkType = "addition"
	// DiffChunkRemoval indicates a line present in expected but not actual.
	DiffChunkRemoval DiffChunkType = "removal"
)

// DiffChunks represents a single line in a diff output.
type DiffChunks struct {
	// Type categorizes the line (context, addition, removal).
	Type DiffChunkType `json:"type"`
	// Content is the line content without the +/- prefix.
	Content string `json:"content"`
}

// SourceLocation identifies a position in source code.
type SourceLocation struct {
	// File is the source file path (may be relative or absolute).
	File string `json:"file"`
	// Line is the line number (1-indexed).
	Line int `json:"line"`
}

// IsZero returns true if the location has not been set.
func (s SourceLocation) IsZero() bool {
	return s.File == "" && s.Line == 0
}
