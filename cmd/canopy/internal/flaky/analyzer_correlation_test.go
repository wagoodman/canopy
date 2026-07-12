package flaky

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// TestAnalyzer_PositionalFailureCorrelation proves that when a SINGLE run has two
// failing tests with DISTINCT fingerprints, each test gets its OWN fingerprint rather
// than both inheriting the first failure's (the pre-fix behavior).
func TestAnalyzer_PositionalFailureCorrelation(t *testing.T) {
	runFail := uuid.New()
	runPass := uuid.New()
	baseTime := time.Now()

	// two distinct tests that both fail in the same run with different failure modes
	refA := gotest.Reference{Package: "pkg/a", FuncName: "TestAlpha"}
	refB := gotest.Reference{Package: "pkg/a", FuncName: "TestBeta"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runFail, Started: baseTime},
					{UUID: runPass, Started: baseTime.Add(time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runFail: {
				// a package-level fail event precedes the test-level ones; the store
				// writes a failure row for it too, so the cursor must skip past it.
				{Reference: gotest.Reference{Package: "pkg/a"}, Action: gotest.FailAction, Time: baseTime, Output: "FAIL pkg/a"},
				{Reference: refA, Action: gotest.FailAction, Time: baseTime, Output: "boom A"},
				{Reference: refB, Action: gotest.FailAction, Time: baseTime, Output: "boom B"},
			},
			// a later passing run so both tests register as flaky
			runPass: {
				{Reference: refA, Action: gotest.PassAction, Time: baseTime.Add(time.Hour)},
				{Reference: refB, Action: gotest.PassAction, Time: baseTime.Add(time.Hour)},
			},
		},
		failures: map[uuid.UUID][]db.FailedTestDetails{
			// one row per fail event, in event order: package, then refA, then refB
			runFail: {
				{Fingerprint: "fp-pkg", Type: "unknown"},
				{Fingerprint: "fp-alpha", Type: "assertion"},
				{Fingerprint: "fp-beta", Type: "panic"},
			},
		},
	}

	analyzer := NewAnalyzer(store, Config{MinRuns: 1})

	alpha, err := analyzer.Analyze(refA)
	require.NoError(t, err)
	require.NotNil(t, alpha)
	require.Len(t, alpha.FailureModes, 1)
	require.Equal(t, "fp-alpha", alpha.FailureModes[0].Fingerprint)

	beta, err := analyzer.Analyze(refB)
	require.NoError(t, err)
	require.NotNil(t, beta)
	require.Len(t, beta.FailureModes, 1)
	// the bug: beta would inherit "fp-alpha" (the first failure). it must be its own.
	require.Equal(t, "fp-beta", beta.FailureModes[0].Fingerprint)
}
