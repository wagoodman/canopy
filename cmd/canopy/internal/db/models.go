package db

import (
	"time"

	"gorm.io/datatypes"
)

// Version is the database schema version identifier.
const Version = "v1"

// models returns all GORM model types for database migration.
func models() []any {
	return []any{
		TestSession{},
		TestRun{},
		TestEvent{},
		Reference{},
		Annotation{},
		FailedTestDetails{},
		CoverageData{},
		FileCoverage{},
		CoverageBlock{},
	}
}

// TestSession represents a complete test session containing multiple test runs.
// A session starts when the user launches canopy and ends when they exit.
type TestSession struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// UUID is the unique identifier exposed externally for this session.
	UUID string `gorm:"column:uuid;index" json:"uuid"`

	// Started is the timestamp when this test session began.
	Started time.Time `gorm:"column:started" json:"started"`

	// Ended is the timestamp when this test session completed (nil if still running).
	Ended *time.Time `gorm:"column:ended" json:"ended"`

	// TestRuns contains all test runs executed within this session.
	TestRuns *[]TestRun `gorm:"foreignKey:SessionID" json:"test_runs"`
}

// TestRun represents a single execution of one or more tests with specific configuration.
type TestRun struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// SessionID links this run to its parent test session.
	SessionID int64 `gorm:"column:session_id" json:"-"`

	// UUID is the unique identifier exposed externally for this run.
	UUID string `gorm:"column:uuid;index" json:"uuid"`

	// Started is the timestamp when this test run began.
	Started time.Time `gorm:"column:started" json:"started"`

	// Ended is the timestamp when this test run completed (nil if still running).
	Ended *time.Time `gorm:"column:ended" json:"ended"`

	// Config is the JSON-encoded test runner configuration used for this run.
	Config datatypes.JSON `gorm:"column:config" json:"config"`

	// Events contains all test events that occurred during this run.
	Events *[]TestEvent `gorm:"foreignKey:RunID" json:"events"`

	// Coverage is the overall code coverage percentage for this run (nil if not calculated).
	Coverage *float64 `gorm:"column:coverage" json:"coverage"`
}

// TestEvent represents a single event from the go test JSON output stream.
type TestEvent struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// RunID links this event to its parent test run.
	RunID int64 `gorm:"column:run_id" json:"-"`

	// Index is the sequential position of this event within the test run.
	Index int64 `gorm:"column:index" json:"index"`

	// ReferenceID links this event to a specific test reference.
	ReferenceID int64 `gorm:"column:reference_id" json:"-"`

	// Reference identifies which test this event belongs to.
	Reference Reference

	// Time is the timestamp when this event occurred during test execution.
	Time time.Time `gorm:"column:time" json:"time"`

	// Action is the test event action (e.g., "run", "pass", "fail", "skip", "output").
	Action string `gorm:"column:action" json:"action"`

	// Output is the test output text associated with this event.
	Output string `gorm:"column:output" json:"output"`

	// Elapsed is the duration in seconds for terminal events (pass/fail/skip).
	// Only populated for events that mark test completion.
	Elapsed *float64 `gorm:"column:elapsed" json:"elapsed,omitempty"`

	// FailedBuild identifies the package that caused a build failure.
	// Present in fail events when the failure was due to a build error in a dependency.
	FailedBuild string `gorm:"column:failed_build" json:"failed_build,omitempty"`

	// Annotations contains metadata tags extracted from test output (e.g., "flaky", "slow").
	Annotations []Annotation `gorm:"many2many:test_event_annotations" json:"annotations"`

	// Error is the error message if this event represents a test failure.
	Error string `gorm:"column:error" json:"error"`

	// Failure contains structured failure data if this event is a failure event.
	Failure *FailedTestDetails `gorm:"foreignKey:EventID" json:"failure,omitempty"`
}

// Reference identifies a specific test or subtest by its package, function, and t.Run name.
// References are deduplicated across test runs to enable historical tracking.
type Reference struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// Package is the Go package path containing this test.
	Package string `gorm:"column:package;index" json:"package"`

	// FuncName is the test function name (e.g., "TestMyFunction").
	FuncName string `gorm:"column:function;index" json:"function"`

	// TRunName is the subtest name from t.Run() calls, empty for top-level tests.
	TRunName string `gorm:"column:t_run_name;index" json:"t_run_name"`
}

// Annotation represents a metadata tag that can be attached to test events.
// Annotations are extracted from test output and used for categorization and filtering.
type Annotation struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// Value is the annotation string (e.g., "flaky", "slow", "integration").
	Value string `gorm:"column:value;uniqueIndex" json:"value"`
}

// FailedTestDetails stores structured failure information parsed from test output.
// This enables rich queries, better visualization, and flaky test detection.
type FailedTestDetails struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// EventID links this failure to its parent test event.
	EventID int64 `gorm:"column:event_id;uniqueIndex" json:"-"`

	// RunID is denormalized from TestEvent for query efficiency.
	RunID int64 `gorm:"column:run_id;index" json:"-"`

	// Type is the failure category (assertion, panic, diff, timeout, unknown).
	Type string `gorm:"column:type;index" json:"type"`

	// Details contains the type-specific failure information as JSON.
	// The structure depends on Type: AssertionInfo, PanicInfo, or DiffInfo.
	Details datatypes.JSON `gorm:"column:details" json:"details,omitempty"`

	// LocationFile is the source file where the failure occurred.
	LocationFile string `gorm:"column:location_file" json:"location_file,omitempty"`

	// LocationLine is the line number where the failure occurred.
	LocationLine int `gorm:"column:location_line" json:"location_line,omitempty"`

	// Fingerprint is a semantic hash for identifying distinct failure modes.
	Fingerprint string `gorm:"column:fingerprint;index" json:"fingerprint"`
}

// CoverageData stores structured coverage information for a test run.
type CoverageData struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// RunID links this coverage data to its parent test run (one coverage data per run).
	RunID int64 `gorm:"column:run_id;uniqueIndex" json:"-"`

	// Mode is the coverage mode from the profile (e.g., "set", "count", "atomic").
	Mode string `gorm:"column:mode" json:"mode"`

	// Files contains per-file coverage data for this run.
	Files []FileCoverage `gorm:"foreignKey:CoverageDataID" json:"files"`
}

// FileCoverage stores coverage data for a single source file.
type FileCoverage struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// CoverageDataID links this file coverage to its parent coverage data.
	CoverageDataID int64 `gorm:"column:coverage_data_id;index" json:"-"`

	// FileName is the package-qualified file path from the coverage profile.
	FileName string `gorm:"column:file_name" json:"file_name"`

	// Blocks contains per-block coverage data for this file.
	Blocks []CoverageBlock `gorm:"foreignKey:FileCoverageID" json:"blocks"`
}

// CoverageBlock stores a single coverage block from the Go coverage profile.
type CoverageBlock struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// FileCoverageID links this block to its parent file coverage.
	FileCoverageID int64 `gorm:"column:file_coverage_id;index" json:"-"`

	// StartLine is the starting line number of the block.
	StartLine int `gorm:"column:start_line" json:"start_line"`

	// StartCol is the starting column number of the block.
	StartCol int `gorm:"column:start_col" json:"start_col"`

	// EndLine is the ending line number of the block.
	EndLine int `gorm:"column:end_line" json:"end_line"`

	// EndCol is the ending column number of the block.
	EndCol int `gorm:"column:end_col" json:"end_col"`

	// NumStmt is the number of statements in the block.
	NumStmt int `gorm:"column:num_stmt" json:"num_stmt"`

	// Count is the number of times the block was executed.
	Count int `gorm:"column:count" json:"count"`
}
