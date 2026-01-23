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

	// Annotations contains metadata tags extracted from test output (e.g., "flaky", "slow").
	Annotations []Annotation `gorm:"many2many:test_event_annotations" json:"annotations"`

	// Error is the error message if this event represents a test failure.
	Error string `gorm:"column:error" json:"error"`
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
