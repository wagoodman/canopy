package trend

import (
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// Store is the subset of test.Manager needed to read historical run data.
type Store interface {
	// ListSessions returns all stored test sessions, most recent first.
	ListSessions() ([]test.SessionInfo, error)
	// GetTestEvents retrieves all events for a single test run.
	GetTestEvents(runID uuid.UUID) ([]gotest.Event, error)
}

// Record is a single terminal observation of a reference within one run.
//
// Cached is only ever true for package-level references: go test emits a lone
// "ok pkg (cached)" event for a cached package and no per-test events, so
// individual test records are always fresh executions.
type Record struct {
	RunID   uuid.UUID
	Time    time.Time
	Action  gotest.Action // pass, fail, or skip
	Elapsed float64       // seconds; 0 when unknown (e.g. cached packages)
	Cached  bool          // the result was served from the go test cache
}

// Scope filters which runs and references the collector gathers.
type Scope struct {
	// Last limits analysis to the most recent N runs across all sessions (0 = no limit).
	Last int
	// Window limits analysis to runs within this duration from now (0 = no limit).
	Window time.Duration
	// SessionIDs limits analysis to these sessions (empty = all sessions).
	SessionIDs []uuid.UUID
	// PackagePatterns are glob patterns matched against package paths (empty = all).
	PackagePatterns []string
	// ExcludePatterns are glob patterns of package paths to exclude.
	ExcludePatterns []string
	// TestPattern filters test function names (nil = all). Not applied to package refs.
	TestPattern *regexp.Regexp
}
