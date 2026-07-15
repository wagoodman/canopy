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
		PackageCoverage{},
		FunctionCoverage{},
		SourceState{},
		FileState{},
	}
}

// droppedModels returns old model types that should be removed during migration.
func droppedModels() []string {
	return []string{
		"coverage_data",
		"file_coverages",
		"coverage_blocks",
	}
}

// TestSession represents a complete test session containing multiple test runs.
// A session starts when the user launches canopy and ends when they exit.
type TestSession struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// UUID is the unique identifier exposed externally for this session.
	UUID string `gorm:"column:uuid;index" json:"uuid"`

	// Name is an optional label used to find-or-create a durable session (empty for TUI-launched sessions).
	Name string `gorm:"column:name;index" json:"name"`

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

	// CoverageDir is the absolute path to the persistent binary coverage directory for this run.
	CoverageDir string `gorm:"column:coverage_dir" json:"coverage_dir,omitempty"`
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

	// the tuple (package, function, t_run_name) is the test's identity and must be unique so
	// GetOrCreateReference's FirstOrCreate is atomic and concurrent writers can't create
	// duplicate references that split a test's history across two IDs.
	// Package is the Go package path containing this test.
	Package string `gorm:"column:package;uniqueIndex:idx_ref_identity,priority:1" json:"package"`

	// FuncName is the test function name (e.g., "TestMyFunction").
	FuncName string `gorm:"column:function;uniqueIndex:idx_ref_identity,priority:2" json:"function"`

	// TRunName is the subtest name from t.Run() calls, empty for top-level tests.
	TRunName string `gorm:"column:t_run_name;uniqueIndex:idx_ref_identity,priority:3" json:"t_run_name"`
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

// PackageCoverage stores per-package coverage data from `go tool covdata percent`.
type PackageCoverage struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// RunID links this coverage data to its parent test run.
	RunID int64 `gorm:"column:run_id;uniqueIndex:idx_pkg_cov_run_pkg" json:"-"`

	// PackagePath is the Go package import path (e.g., "github.com/org/repo/pkg").
	PackagePath string `gorm:"column:package_path;uniqueIndex:idx_pkg_cov_run_pkg" json:"package_path"`

	// Percent is the coverage percentage for this package.
	Percent float64 `gorm:"column:percent" json:"percent"`
}

// FunctionCoverage stores per-function coverage data from `go tool covdata func`.
type FunctionCoverage struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// RunID links this coverage data to its parent test run.
	RunID int64 `gorm:"column:run_id;index" json:"-"`

	// FilePath is the source file path containing the function.
	FilePath string `gorm:"column:file_path" json:"file_path"`

	// Line is the line number where the function is defined.
	Line int `gorm:"column:line" json:"line"`

	// FuncName is the function name.
	FuncName string `gorm:"column:func_name" json:"func_name"`

	// Percent is the coverage percentage for this function.
	Percent float64 `gorm:"column:percent" json:"percent"`
}

// SourceState captures git repository state at test run time.
type SourceState struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// RunID links this source state to its parent test run (one source state per run).
	RunID int64 `gorm:"column:run_id;uniqueIndex" json:"-"`

	// Commit is the HEAD commit hash (full SHA).
	Commit string `gorm:"column:commit;index" json:"commit"`

	// Branch is the current branch name, "HEAD" if detached.
	Branch string `gorm:"column:branch" json:"branch"`

	// Dirty indicates whether there are uncommitted .go file changes.
	Dirty bool `gorm:"column:dirty" json:"dirty"`

	// DirtyFiles contains state for each dirty .go file.
	DirtyFiles []FileState `gorm:"foreignKey:SourceStateID" json:"dirty_files,omitempty"`
}

// FileState represents a single dirty .go file's state at test run time.
type FileState struct {
	// ID is the primary key for database relationships.
	ID int64 `gorm:"primaryKey" json:"-"`

	// SourceStateID links this file state to its parent source state.
	SourceStateID int64 `gorm:"column:source_state_id;index" json:"-"`

	// Path is the file path relative to the repository root.
	Path string `gorm:"column:path" json:"path"`

	// ContentHash is the xxhash64 hex digest of the file content (empty if deleted).
	ContentHash string `gorm:"column:content_hash" json:"content_hash"`

	// ModTime is the file modification time (nil if deleted).
	ModTime *time.Time `gorm:"column:mod_time" json:"mod_time,omitempty"`
}
