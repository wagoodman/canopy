package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func createRunWithData(t *testing.T, store *Store, sessionID int64, started time.Time) int64 {
	t.Helper()

	run := TestRun{
		SessionID: sessionID,
		UUID:      "run-" + started.Format(time.RFC3339Nano),
		Started:   started,
	}
	require.NoError(t, store.db.Create(&run).Error)

	// add a reference and annotation (shared/deduplicated)
	ref := Reference{Package: "pkg/foo", FuncName: "TestFoo", TRunName: ""}
	require.NoError(t, store.GetOrCreateReference(&ref))

	ann := Annotation{Value: "flaky"}
	require.NoError(t, store.GetOrCreateAnnotation(&ann))

	// add test events
	event := TestEvent{
		RunID:       run.ID,
		Index:       0,
		ReferenceID: ref.ID,
		Time:        started,
		Action:      "pass",
		Annotations: []Annotation{ann},
	}
	require.NoError(t, store.db.Create(&event).Error)

	// add failed test details
	failure := FailedTestDetails{
		EventID:      event.ID,
		RunID:        run.ID,
		Type:         "assertion",
		Fingerprint:  "abc123",
		LocationFile: "foo_test.go",
		LocationLine: 42,
	}
	require.NoError(t, store.db.Create(&failure).Error)

	// add coverage data (new schema)
	pkgCov := PackageCoverage{RunID: run.ID, PackagePath: "pkg/foo", Percent: 75.0}
	require.NoError(t, store.db.Create(&pkgCov).Error)

	funcCov := FunctionCoverage{
		RunID:    run.ID,
		FilePath: "foo.go",
		Line:     10,
		FuncName: "TestFoo",
		Percent:  100.0,
	}
	require.NoError(t, store.db.Create(&funcCov).Error)

	return run.ID
}

func createSession(t *testing.T, store *Store) int64 {
	t.Helper()
	session := TestSession{
		UUID:    "session-" + time.Now().Format(time.RFC3339Nano),
		Started: time.Now(),
	}
	require.NoError(t, store.db.Create(&session).Error)
	return session.ID
}

func countRows(t *testing.T, store *Store, model interface{}) int64 {
	t.Helper()
	var count int64
	require.NoError(t, store.db.Model(model).Count(&count).Error)
	return count
}

func countJoinTableRows(t *testing.T, store *Store) int64 {
	t.Helper()
	var count int64
	require.NoError(t, store.db.Raw("SELECT COUNT(*) FROM test_event_annotations").Scan(&count).Error)
	return count
}

func TestDeleteRun_CascadesAllData(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	runID := createRunWithData(t, store, sessionID, time.Now())

	// verify data exists
	require.Equal(t, int64(1), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(1), countRows(t, store, &TestEvent{}))
	require.Equal(t, int64(1), countRows(t, store, &FailedTestDetails{}))
	require.Equal(t, int64(1), countRows(t, store, &PackageCoverage{}))
	require.Equal(t, int64(1), countRows(t, store, &FunctionCoverage{}))
	require.Equal(t, int64(1), countJoinTableRows(t, store))

	// delete the run
	deleted, err := store.DeleteRuns([]int64{runID})
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	// verify all child data is gone
	require.Equal(t, int64(0), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(0), countRows(t, store, &TestEvent{}))
	require.Equal(t, int64(0), countRows(t, store, &FailedTestDetails{}))
	require.Equal(t, int64(0), countRows(t, store, &PackageCoverage{}))
	require.Equal(t, int64(0), countRows(t, store, &FunctionCoverage{}))
	require.Equal(t, int64(0), countJoinTableRows(t, store))
}

func TestDeleteRun_PreservesSharedReferences(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	runID := createRunWithData(t, store, sessionID, time.Now())

	// verify references and annotations exist
	refsBefore := countRows(t, store, &Reference{})
	annsBefore := countRows(t, store, &Annotation{})
	require.True(t, refsBefore > 0)
	require.True(t, annsBefore > 0)

	_, err = store.DeleteRuns([]int64{runID})
	require.NoError(t, err)

	// references and annotations should still exist (they're shared lookup tables)
	require.Equal(t, refsBefore, countRows(t, store, &Reference{}))
	require.Equal(t, annsBefore, countRows(t, store, &Annotation{}))
}

func TestDeleteRun_PreservesOtherRuns(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	run1ID := createRunWithData(t, store, sessionID, time.Now().Add(-2*time.Hour))
	_ = createRunWithData(t, store, sessionID, time.Now().Add(-1*time.Hour))

	// verify we have 2 of everything
	require.Equal(t, int64(2), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(2), countRows(t, store, &TestEvent{}))
	require.Equal(t, int64(2), countRows(t, store, &FailedTestDetails{}))
	require.Equal(t, int64(2), countRows(t, store, &PackageCoverage{}))

	// delete only the first run
	deleted, err := store.DeleteRuns([]int64{run1ID})
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	// the second run's data should still be intact
	require.Equal(t, int64(1), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(1), countRows(t, store, &TestEvent{}))
	require.Equal(t, int64(1), countRows(t, store, &FailedTestDetails{}))
	require.Equal(t, int64(1), countRows(t, store, &PackageCoverage{}))
	require.Equal(t, int64(1), countRows(t, store, &FunctionCoverage{}))
}

func TestDeleteRunsByAge(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)

	now := time.Now()
	_ = createRunWithData(t, store, sessionID, now.Add(-48*time.Hour)) // 2 days old
	_ = createRunWithData(t, store, sessionID, now.Add(-24*time.Hour)) // 1 day old
	_ = createRunWithData(t, store, sessionID, now.Add(-1*time.Hour))  // 1 hour old

	require.Equal(t, int64(3), countRows(t, store, &TestRun{}))

	// delete runs older than 36 hours (should remove the 2-day-old run)
	deleted, err := store.DeleteRunsByAge(36 * time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, int64(2), countRows(t, store, &TestRun{}))

	// delete runs older than 2 hours (should remove the 1-day-old run)
	deleted, err = store.DeleteRunsByAge(2 * time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, int64(1), countRows(t, store, &TestRun{}))
}

func TestDeleteRunsKeepingLast(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)

	now := time.Now()
	_ = createRunWithData(t, store, sessionID, now.Add(-3*time.Hour))
	_ = createRunWithData(t, store, sessionID, now.Add(-2*time.Hour))
	_ = createRunWithData(t, store, sessionID, now.Add(-1*time.Hour))

	require.Equal(t, int64(3), countRows(t, store, &TestRun{}))

	// keep last 2
	deleted, err := store.DeleteRunsKeepingLast(2)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, int64(2), countRows(t, store, &TestRun{}))

	// verify the remaining runs are the 2 most recent
	var remaining []TestRun
	require.NoError(t, store.db.Order("started DESC").Find(&remaining).Error)
	require.Len(t, remaining, 2)
	// most recent should be ~1 hour old, next ~2 hours old
	require.True(t, remaining[0].Started.After(remaining[1].Started))
}

func TestDeleteRunsKeepingLast_KeepMoreThanExist(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	_ = createRunWithData(t, store, sessionID, time.Now())

	deleted, err := store.DeleteRunsKeepingLast(10)
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
	require.Equal(t, int64(1), countRows(t, store, &TestRun{}))
}

func TestDeleteAllRuns(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	session1ID := createSession(t, store)
	session2ID := createSession(t, store)

	_ = createRunWithData(t, store, session1ID, time.Now().Add(-2*time.Hour))
	_ = createRunWithData(t, store, session1ID, time.Now().Add(-1*time.Hour))
	_ = createRunWithData(t, store, session2ID, time.Now())

	require.Equal(t, int64(3), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(2), countRows(t, store, &TestSession{}))

	deleted, err := store.DeleteAllRuns()
	require.NoError(t, err)
	require.Equal(t, 3, deleted)
	require.Equal(t, int64(0), countRows(t, store, &TestRun{}))
	require.Equal(t, int64(0), countRows(t, store, &TestEvent{}))
	require.Equal(t, int64(0), countRows(t, store, &PackageCoverage{}))

	// sessions should also be gone (orphaned)
	require.Equal(t, int64(0), countRows(t, store, &TestSession{}))
}

func TestDeleteOrphanedSessions(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	// session with runs
	session1ID := createSession(t, store)
	_ = createRunWithData(t, store, session1ID, time.Now())

	// session without runs (orphaned)
	_ = createSession(t, store)

	require.Equal(t, int64(2), countRows(t, store, &TestSession{}))

	deleted, err := store.DeleteOrphanedSessions()
	require.NoError(t, err)
	require.Equal(t, 1, deleted)
	require.Equal(t, int64(1), countRows(t, store, &TestSession{}))
}

func TestDeleteRuns_EmptySlice(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	deleted, err := store.DeleteRuns(nil)
	require.NoError(t, err)
	require.Equal(t, 0, deleted)
}

func TestCountRuns(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	now := time.Now()

	_ = createRunWithData(t, store, sessionID, now.Add(-48*time.Hour))
	_ = createRunWithData(t, store, sessionID, now.Add(-24*time.Hour))
	_ = createRunWithData(t, store, sessionID, now.Add(-1*time.Hour))

	count, err := store.CountRuns()
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	byAge, err := store.CountRunsByAge(36 * time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), byAge)

	beyondKeep, err := store.CountRunsBeyondKeep(2)
	require.NoError(t, err)
	require.Equal(t, int64(1), beyondKeep)
}

func TestVacuum(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	// vacuum on an empty DB should not error
	require.NoError(t, store.Vacuum())
}

func TestDeleteRun_CascadesAllData_StructComparison(t *testing.T) {
	// verify that after deletion, all table counts match expected zeros
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID := createSession(t, store)
	runID := createRunWithData(t, store, sessionID, time.Now())

	_, err = store.DeleteRuns([]int64{runID})
	require.NoError(t, err)

	type tableCounts struct {
		Runs              int64
		Events            int64
		Failures          int64
		PackageCoverages  int64
		FunctionCoverages int64
		JoinTableRows     int64
	}

	got := tableCounts{
		Runs:              countRows(t, store, &TestRun{}),
		Events:            countRows(t, store, &TestEvent{}),
		Failures:          countRows(t, store, &FailedTestDetails{}),
		PackageCoverages:  countRows(t, store, &PackageCoverage{}),
		FunctionCoverages: countRows(t, store, &FunctionCoverage{}),
		JoinTableRows:     countJoinTableRows(t, store),
	}

	want := tableCounts{}

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("table counts after deletion mismatch (-want +got):\n%s", d)
	}
}

// TestSetRunCoverageDir_OrphanCleanup covers the orphan scenario: a coverage-enabled run
// creates its on-disk dir but covdata produces no data (so EndTestRun gets nil coverage).
// The dir must still be tracked and removed by DeleteRuns.
func TestSetRunCoverageDir_OrphanCleanup(t *testing.T) {
	store, err := New(":memory:")
	require.NoError(t, err)

	sessionID, err := store.StartTestSession()
	require.NoError(t, err)

	runID, err := store.StartTestRun(sessionID, gotest.RunnerConfig{Coverage: true})
	require.NoError(t, err)

	// simulate the dir being created on disk at run start
	covDir := filepath.Join(t.TempDir(), "coverage", runID.String())
	require.NoError(t, os.MkdirAll(covDir, 0o755))

	// persist the dir immediately (as manager.go now does)
	require.NoError(t, store.SetRunCoverageDir(runID, covDir))

	// end the run with NO coverage data (the orphan-producing path)
	require.NoError(t, store.EndTestRun(runID, nil))

	// column must still hold the dir despite no coverage data at end
	run, err := store.GetTestRun(runID)
	require.NoError(t, err)
	require.Equal(t, covDir, run.CoverageDir)

	// DeleteRuns must now find and remove the dir
	_, err = store.DeleteRuns([]int64{run.ID})
	require.NoError(t, err)

	_, statErr := os.Stat(covDir)
	require.True(t, os.IsNotExist(statErr), "expected coverage dir to be removed")
}
