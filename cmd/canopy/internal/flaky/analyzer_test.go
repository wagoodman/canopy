package flaky

import (
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

func TestCalculateFlakyScore(t *testing.T) {
	tests := []struct {
		name   string
		passes int
		fails  int
		want   float64
	}{
		{
			name:   "all passes = not flaky",
			passes: 10,
			fails:  0,
			want:   0.0,
		},
		{
			name:   "all fails = not flaky",
			passes: 0,
			fails:  10,
			want:   0.0,
		},
		{
			name:   "50/50 = maximally flaky",
			passes: 5,
			fails:  5,
			want:   1.0,
		},
		{
			name:   "75/25 = moderately flaky",
			passes: 3,
			fails:  1,
			want:   0.75,
		},
		{
			name:   "90/10 = slightly flaky",
			passes: 9,
			fails:  1,
			want:   0.36,
		},
		{
			name:   "no runs = not flaky",
			passes: 0,
			fails:  0,
			want:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateFlakyScore(tt.passes, tt.fails)
			require.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestCalculateFlakyScoreFromRate(t *testing.T) {
	tests := []struct {
		name     string
		passRate float64
		want     float64
	}{
		{
			name:     "0% pass rate",
			passRate: 0.0,
			want:     0.0,
		},
		{
			name:     "100% pass rate",
			passRate: 1.0,
			want:     0.0,
		},
		{
			name:     "50% pass rate",
			passRate: 0.5,
			want:     1.0,
		},
		{
			name:     "25% pass rate",
			passRate: 0.25,
			want:     0.75,
		},
		{
			name:     "negative rate clamped",
			passRate: -0.5,
			want:     0.0,
		},
		{
			name:     "over 100% rate clamped",
			passRate: 1.5,
			want:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateFlakyScoreFromRate(tt.passRate)
			require.InDelta(t, tt.want, got, 0.01)
		})
	}
}

// mockStore implements the Store interface for testing
type mockStore struct {
	sessions []test.SessionInfo
	events   map[uuid.UUID][]gotest.Event
	failures map[uuid.UUID][]db.FailedTestDetails
}

func (m *mockStore) ListSessions() ([]test.SessionInfo, error) {
	return m.sessions, nil
}

func (m *mockStore) GetTestEvents(runID uuid.UUID) ([]gotest.Event, error) {
	return m.events[runID], nil
}

func (m *mockStore) GetFailuresByRun(runID uuid.UUID) ([]db.FailedTestDetails, error) {
	if m.failures == nil {
		return nil, nil
	}
	return m.failures[runID], nil
}

func TestAnalyzer_AnalyzeAll(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	runID3 := uuid.New()

	baseTime := time.Now()

	ref1 := gotest.Reference{Package: "pkg/a", FuncName: "TestStable"}
	ref2 := gotest.Reference{Package: "pkg/a", FuncName: "TestFlaky"}
	ref3 := gotest.Reference{Package: "pkg/b", FuncName: "TestAlwaysFails"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
					{UUID: runID3, Started: baseTime.Add(2 * time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref2, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref3, Action: gotest.FailAction, Time: baseTime, Output: "error A"},
			},
			runID2: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime.Add(time.Hour)},
				{Reference: ref2, Action: gotest.FailAction, Time: baseTime.Add(time.Hour), Output: "error B"},
				{Reference: ref3, Action: gotest.FailAction, Time: baseTime.Add(time.Hour), Output: "error A"},
			},
			runID3: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime.Add(2 * time.Hour)},
				{Reference: ref2, Action: gotest.PassAction, Time: baseTime.Add(2 * time.Hour)},
				{Reference: ref3, Action: gotest.FailAction, Time: baseTime.Add(2 * time.Hour), Output: "error A"},
			},
		},
	}

	analyzer := NewAnalyzer(store, Config{
		MinRuns:   2,
		Threshold: 0,
	})

	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)
	// only flaky tests (with both passes and fails) are returned
	require.Len(t, results, 1)

	// the flaky test should be the only result
	flaky := results[0]
	require.Equal(t, "TestFlaky", flaky.Reference.FuncName)
	require.Equal(t, 2, flaky.PassCount)
	require.Equal(t, 1, flaky.FailCount)
	require.Greater(t, flaky.Score, 0.0)
	require.True(t, flaky.IsFlaky())
	require.NotNil(t, flaky.LastFlip)

	// use Analyze() to verify non-flaky tests are still analyzed correctly
	// but excluded from AnalyzeAll() results
	stable, err := analyzer.Analyze(ref1)
	require.NoError(t, err)
	require.NotNil(t, stable)
	require.Equal(t, 3, stable.PassCount)
	require.Equal(t, 0, stable.FailCount)
	require.Equal(t, 0.0, stable.Score)
	require.False(t, stable.IsFlaky())

	alwaysFails, err := analyzer.Analyze(ref3)
	require.NoError(t, err)
	require.NotNil(t, alwaysFails)
	require.Equal(t, 0, alwaysFails.PassCount)
	require.Equal(t, 3, alwaysFails.FailCount)
	require.Equal(t, 0.0, alwaysFails.Score)
	require.False(t, alwaysFails.IsFlaky())
}

func TestAnalyzer_MinRunsFilter(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	runID3 := uuid.New()
	baseTime := time.Now()

	ref := gotest.Reference{Package: "pkg/a", FuncName: "TestFlaky"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
					{UUID: runID3, Started: baseTime.Add(2 * time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				{Reference: ref, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
			runID3: {
				{Reference: ref, Action: gotest.PassAction, Time: baseTime.Add(2 * time.Hour)},
			},
		},
	}

	// with MinRuns=5, the test with only 3 runs should be excluded
	analyzer := NewAnalyzer(store, Config{MinRuns: 5})
	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Empty(t, results)

	// with MinRuns=3, it should be included (test has 3 runs and is flaky)
	analyzer = NewAnalyzer(store, Config{MinRuns: 3})
	results, err = analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestAnalyzer_ThresholdFilter(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()

	ref1 := gotest.Reference{Package: "pkg/a", FuncName: "TestSlightlyFlaky"}
	ref2 := gotest.Reference{Package: "pkg/a", FuncName: "TestVeryFlaky"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref2, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime.Add(time.Hour)}, // still passes
				{Reference: ref2, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)}, // fails
			},
		},
	}

	// ref1: 2 pass, 0 fail -> score 0
	// ref2: 1 pass, 1 fail -> score 1.0

	// threshold of 0.5 should only include ref2
	analyzer := NewAnalyzer(store, Config{MinRuns: 1, Threshold: 0.5})
	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "TestVeryFlaky", results[0].Reference.FuncName)
}

func TestAnalyzer_WindowFilter(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()
	oldTime := baseTime.Add(-48 * time.Hour) // 2 days ago

	ref := gotest.Reference{Package: "pkg/a", FuncName: "TestOld"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: oldTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: oldTime},
					{UUID: runID2, Started: baseTime},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref, Action: gotest.FailAction, Time: oldTime, Output: "old error"},
			},
			runID2: {
				{Reference: ref, Action: gotest.PassAction, Time: baseTime},
			},
		},
	}

	// with 24h window, old run should be excluded
	// only the recent pass is counted -> 1 pass, 0 fail -> not flaky, empty results
	analyzer := NewAnalyzer(store, Config{MinRuns: 1, Window: 24 * time.Hour})
	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Empty(t, results, "with window filter, test should not be flaky (only recent pass counted)")

	// without window, both runs counted -> 1 pass, 1 fail -> score 1.0, is flaky
	analyzer = NewAnalyzer(store, Config{MinRuns: 1, Window: 0})
	results, err = analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, 1, results[0].PassCount)
	require.Equal(t, 1, results[0].FailCount)
	require.Equal(t, 1.0, results[0].Score)
}

func TestAnalyzer_DistinctFailures(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	runID3 := uuid.New()
	runID4 := uuid.New()
	baseTime := time.Now()

	ref := gotest.Reference{Package: "pkg/a", FuncName: "TestMultipleFailureModes"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
					{UUID: runID3, Started: baseTime.Add(2 * time.Hour)},
					{UUID: runID4, Started: baseTime.Add(3 * time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref, Action: gotest.FailAction, Time: baseTime},
			},
			runID2: {
				{Reference: ref, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
			runID3: {
				{Reference: ref, Action: gotest.FailAction, Time: baseTime.Add(2 * time.Hour)},
			},
			runID4: {
				// add a pass to make this test flaky
				{Reference: ref, Action: gotest.PassAction, Time: baseTime.Add(3 * time.Hour)},
			},
		},
		failures: map[uuid.UUID][]db.FailedTestDetails{
			runID1: {
				{Fingerprint: "fp-nil-pointer", Type: "panic"},
			},
			runID2: {
				{Fingerprint: "fp-timeout", Type: "timeout"},
			},
			runID3: {
				{Fingerprint: "fp-nil-pointer", Type: "panic"}, // same as first
			},
		},
	}

	analyzer := NewAnalyzer(store, Config{MinRuns: 1})
	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].IsFlaky())

	// should detect 2 distinct failure modes (nil pointer and timeout)
	require.Len(t, results[0].FailureModes, 2)
	require.True(t, results[0].HasDistinctFailures())

	// verify the failure modes contain the expected fingerprints
	fingerprints := make(map[string]int)
	for _, mode := range results[0].FailureModes {
		fingerprints[mode.Fingerprint] = mode.Count()
	}
	require.Equal(t, 2, fingerprints["fp-nil-pointer"]) // occurred twice
	require.Equal(t, 1, fingerprints["fp-timeout"])     // occurred once
}

func TestAnalysis_Methods(t *testing.T) {
	analysis := Analysis{
		PassCount: 7,
		FailCount: 3,
		SkipCount: 2,
		TotalRuns: 12,
	}

	require.InDelta(t, 0.7, analysis.PassRate(), 0.01)
	require.InDelta(t, 0.3, analysis.FailRate(), 0.01)
	require.True(t, analysis.IsFlaky())

	// not flaky if no failures
	analysis.FailCount = 0
	require.False(t, analysis.IsFlaky())

	// not flaky if no passes
	analysis.PassCount = 0
	analysis.FailCount = 10
	require.False(t, analysis.IsFlaky())

	// edge case: no data
	empty := Analysis{}
	require.Equal(t, 0.0, empty.PassRate())
	require.Equal(t, 0.0, empty.FailRate())
	require.False(t, empty.IsFlaky())
}

func TestAnalyzer_Analyze(t *testing.T) {
	runID := uuid.New()
	baseTime := time.Now()

	ref1 := gotest.Reference{Package: "pkg/a", FuncName: "TestA"}
	ref2 := gotest.Reference{Package: "pkg/b", FuncName: "TestB"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs:    []test.RunInfo{{UUID: runID, Started: baseTime}},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref2, Action: gotest.FailAction, Time: baseTime},
			},
		},
	}

	analyzer := NewAnalyzer(store, DefaultConfig())

	// analyze existing reference
	result, err := analyzer.Analyze(ref1)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, ref1, result.Reference)
	require.Equal(t, 1, result.PassCount)

	// analyze non-existent reference
	nonExistent := gotest.Reference{Package: "pkg/c", FuncName: "TestNotFound"}
	result, err = analyzer.Analyze(nonExistent)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestAnalyzer_SkipsPackageReferences(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()

	pkgRef := gotest.Reference{Package: "pkg/a"} // package-level
	funcRef := gotest.Reference{Package: "pkg/a", FuncName: "TestFunc"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: pkgRef, Action: gotest.PassAction, Time: baseTime},
				{Reference: funcRef, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				{Reference: pkgRef, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
				{Reference: funcRef, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
		},
	}

	analyzer := NewAnalyzer(store, Config{MinRuns: 1})
	results, err := analyzer.AnalyzeAll()
	require.NoError(t, err)

	// should only include the function reference, not the package reference
	// the function reference is flaky (1 pass, 1 fail)
	require.Len(t, results, 1)
	require.Equal(t, "TestFunc", results[0].Reference.FuncName)
	require.True(t, results[0].IsFlaky())
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	expected := Config{
		Window:    0,
		MinRuns:   2,
		Threshold: 0,
	}

	if diff := cmp.Diff(expected, cfg, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("DefaultConfig() mismatch (-want +got):\n%s", diff)
	}
}

func TestAnalyzer_PackagePatternFilter(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()

	ref1 := gotest.Reference{Package: "github.com/foo/bar/cmd", FuncName: "TestCmd"}
	ref2 := gotest.Reference{Package: "github.com/foo/bar/internal/pkg", FuncName: "TestInternal"}
	ref3 := gotest.Reference{Package: "github.com/foo/bar/api", FuncName: "TestAPI"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref2, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref3, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				// all fail in second run to make them flaky
				{Reference: ref1, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
				{Reference: ref2, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
				{Reference: ref3, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
		},
	}

	tests := []struct {
		name            string
		packagePatterns []string
		excludePatterns []string
		wantPackages    []string
	}{
		{
			name:            "no patterns includes all",
			packagePatterns: nil,
			excludePatterns: nil,
			wantPackages:    []string{"github.com/foo/bar/cmd", "github.com/foo/bar/internal/pkg", "github.com/foo/bar/api"},
		},
		{
			name:            "include pattern filters to matching",
			packagePatterns: []string{"github.com/foo/bar/cmd"},
			wantPackages:    []string{"github.com/foo/bar/cmd"},
		},
		{
			name:            "include pattern with ... suffix",
			packagePatterns: []string{"github.com/foo/bar/..."},
			wantPackages:    []string{"github.com/foo/bar/cmd", "github.com/foo/bar/internal/pkg", "github.com/foo/bar/api"},
		},
		{
			name:            "exclude pattern removes matching",
			excludePatterns: []string{"github.com/foo/bar/internal/pkg"},
			wantPackages:    []string{"github.com/foo/bar/cmd", "github.com/foo/bar/api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(store, Config{
				MinRuns:         1,
				PackagePatterns: tt.packagePatterns,
				ExcludePatterns: tt.excludePatterns,
			})

			results, err := analyzer.AnalyzeAll()
			require.NoError(t, err)

			var gotPackages []string
			for _, r := range results {
				gotPackages = append(gotPackages, r.Reference.Package)
			}

			require.ElementsMatch(t, tt.wantPackages, gotPackages)
		})
	}
}

func TestAnalyzer_TestPatternFilter(t *testing.T) {
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()

	ref1 := gotest.Reference{Package: "pkg/a", FuncName: "TestUser"}
	ref2 := gotest.Reference{Package: "pkg/a", FuncName: "TestUserLogin"}
	ref3 := gotest.Reference{Package: "pkg/a", FuncName: "TestAdmin"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    uuid.New(),
				Started: baseTime,
				Runs: []test.RunInfo{
					{UUID: runID1, Started: baseTime},
					{UUID: runID2, Started: baseTime.Add(time.Hour)},
				},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref1, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref2, Action: gotest.PassAction, Time: baseTime},
				{Reference: ref3, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				// all fail in second run to make them flaky
				{Reference: ref1, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
				{Reference: ref2, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
				{Reference: ref3, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
		},
	}

	tests := []struct {
		name      string
		pattern   string
		wantTests []string
	}{
		{
			name:      "no pattern includes all",
			pattern:   "",
			wantTests: []string{"TestUser", "TestUserLogin", "TestAdmin"},
		},
		{
			name:      "exact match",
			pattern:   "^TestUser$",
			wantTests: []string{"TestUser"},
		},
		{
			name:      "prefix match",
			pattern:   "^TestUser",
			wantTests: []string{"TestUser", "TestUserLogin"},
		},
		{
			name:      "contains match",
			pattern:   "User",
			wantTests: []string{"TestUser", "TestUserLogin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{MinRuns: 1}
			if tt.pattern != "" {
				cfg.TestPattern = regexp.MustCompile(tt.pattern)
			}

			analyzer := NewAnalyzer(store, cfg)
			results, err := analyzer.AnalyzeAll()
			require.NoError(t, err)

			var gotTests []string
			for _, r := range results {
				gotTests = append(gotTests, r.Reference.FuncName)
			}

			require.ElementsMatch(t, tt.wantTests, gotTests)
		})
	}
}

func TestAnalyzer_SessionFilter(t *testing.T) {
	session1 := uuid.New()
	session2 := uuid.New()
	runID1 := uuid.New()
	runID2 := uuid.New()
	baseTime := time.Now()

	ref := gotest.Reference{Package: "pkg/a", FuncName: "TestFlaky"}

	store := &mockStore{
		sessions: []test.SessionInfo{
			{
				UUID:    session1,
				Started: baseTime,
				Runs:    []test.RunInfo{{UUID: runID1, Started: baseTime}},
			},
			{
				UUID:    session2,
				Started: baseTime.Add(time.Hour),
				Runs:    []test.RunInfo{{UUID: runID2, Started: baseTime.Add(time.Hour)}},
			},
		},
		events: map[uuid.UUID][]gotest.Event{
			runID1: {
				{Reference: ref, Action: gotest.PassAction, Time: baseTime},
			},
			runID2: {
				{Reference: ref, Action: gotest.FailAction, Time: baseTime.Add(time.Hour)},
			},
		},
	}

	tests := []struct {
		name       string
		sessionIDs []uuid.UUID
		wantLen    int
		wantFlaky  bool
	}{
		{
			name:       "no session filter includes all - test is flaky",
			sessionIDs: nil,
			wantLen:    1,
			wantFlaky:  true,
		},
		{
			name:       "filter to session with pass only - not flaky",
			sessionIDs: []uuid.UUID{session1},
			wantLen:    0, // not flaky (only passes), so not returned
			wantFlaky:  false,
		},
		{
			name:       "filter to session with fail only - not flaky",
			sessionIDs: []uuid.UUID{session2},
			wantLen:    0, // not flaky (only fails), so not returned
			wantFlaky:  false,
		},
		{
			name:       "filter to both sessions - test is flaky",
			sessionIDs: []uuid.UUID{session1, session2},
			wantLen:    1,
			wantFlaky:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(store, Config{
				MinRuns:    1,
				SessionIDs: tt.sessionIDs,
			})

			results, err := analyzer.AnalyzeAll()
			require.NoError(t, err)
			require.Len(t, results, tt.wantLen)
			if tt.wantFlaky {
				require.True(t, results[0].IsFlaky())
			}
		})
	}
}

func TestMatchGlobPrefix(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		pkg     string
		want    bool
	}{
		{
			name:    "trailing /... matches prefix",
			pattern: "github.com/foo/bar/...",
			pkg:     "github.com/foo/bar/cmd",
			want:    true,
		},
		{
			name:    "trailing /... matches exact",
			pattern: "github.com/foo/bar/...",
			pkg:     "github.com/foo/bar",
			want:    true,
		},
		{
			name:    "trailing /... does not match different prefix",
			pattern: "github.com/foo/bar/...",
			pkg:     "github.com/foo/baz/cmd",
			want:    false,
		},
		{
			name:    "exact pattern does not match with glob",
			pattern: "github.com/foo/bar",
			pkg:     "github.com/foo/bar/cmd",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlobPrefix(tt.pattern, tt.pkg)
			require.Equal(t, tt.want, got)
		})
	}
}
