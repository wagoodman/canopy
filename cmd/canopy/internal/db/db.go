// Package db provides SQLite database persistence for test sessions and results.
package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cover"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CoverageInput bundles coverage data for storage when a test run completes.
type CoverageInput struct {
	Percent      float64
	CoverageDir  string
	Packages     []cover.PackageResult
	Functions    []cover.FunctionResult
}

// SourceStateInput bundles source state data for storage.
type SourceStateInput struct {
	Commit     string
	Branch     string
	Dirty      bool
	DirtyFiles []DirtyFileInput
}

// DirtyFileInput represents a single dirty file to be stored.
type DirtyFileInput struct {
	Path        string
	ContentHash string
	ModTime     *time.Time
}

// Store provides database operations for test session persistence using SQLite and GORM.
type Store struct {
	db              *gorm.DB
	failureRegistry *failure.Registry
}

// New creates a new database store instance at the specified file path.
// It automatically runs schema migrations and configures SQLite performance optimizations.
// The dbFilePath can be ":memory:" for an in-memory database or a file path for persistent storage.
func New(dbFilePath string) (*Store, error) {
	db, err := open(dbFilePath)
	if err != nil {
		return nil, err
	}

	db.Exec("PRAGMA foreign_keys = ON")

	ms := models()
	for i := range ms {
		model := ms[i]
		if err := db.AutoMigrate(&model); err != nil {
			// TODO: get model name that failed...
			return nil, fmt.Errorf("unable to migrate: %w", err)
		}
	}

	// drop old coverage tables that are no longer used (replaced by PackageCoverage + FunctionCoverage)
	for _, table := range droppedModels() {
		if db.Migrator().HasTable(table) {
			if err := db.Migrator().DropTable(table); err != nil {
				log.WithFields("table", table, "error", err).Warn("failed to drop old table")
			}
		}
	}

	// probably not safe... should reconsider this
	db.Exec("PRAGMA synchronous = OFF")
	db.Exec("PRAGMA journal_mode = OFF")
	db.Exec("PRAGMA temp_store = MEMORY")
	db.Exec("PRAGMA cache_size = 100000")
	db.Exec("PRAGMA mmap_size = 268435456") // 256 MB
	// db.Exec("PRAGMA auto_vacuum = NONE")

	return &Store{
		db:              db,
		failureRegistry: failure.NewRegistry(),
	}, nil
}

// GetTestSession retrieves a test session by its UUID, preloading all associated test runs.
func (s Store) GetTestSession(sessionID uuid.UUID) (TestSession, error) {
	var session TestSession
	if err := s.db.Preload("TestRuns").Where("uuid = ?", sessionID.String()).First(&session).Error; err != nil {
		return TestSession{}, fmt.Errorf("unable to get test session: %w", err)
	}
	return session, nil
}

// GetTestSessions retrieves all test sessions from the database, preloading their test runs.
func (s Store) GetTestSessions() ([]TestSession, error) {
	var sessions []TestSession
	if err := s.db.Preload("TestRuns").Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("unable to get test sessions: %w", err)
	}
	return sessions, nil
}

// StartTestSession creates a new test session with the current timestamp and returns its UUID.
func (s Store) StartTestSession() (uuid.UUID, error) {
	session := TestSession{
		UUID:    uuid.NewString(),
		Started: time.Now(),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return uuid.Nil, fmt.Errorf("unable to create test session: %w", err)
	}
	return uuid.Parse(session.UUID)
}

// EndTestSession marks a test session as completed by setting its ended timestamp.
func (s Store) EndTestSession(sessionID uuid.UUID) error {
	session, err := s.GetTestSession(sessionID)
	if err != nil {
		return err
	}
	n := time.Now()
	session.Ended = &n
	if err := s.db.Save(&session).Error; err != nil {
		return fmt.Errorf("unable to end test session: %w", err)
	}
	return nil
}

// StartTestRun creates a new test run within a session with the given configuration.
// The configuration is JSON-encoded and stored for future reference.
func (s Store) StartTestRun(sessionID uuid.UUID, cfg gotest.RunnerConfig) (uuid.UUID, error) {
	session, err := s.GetTestSession(sessionID)
	if err != nil {
		return uuid.Nil, err
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to marshal runner config: %w", err)
	}

	run := TestRun{
		SessionID: session.ID,
		UUID:      uuid.NewString(),
		Started:   time.Now(),
		Config:    cfgBytes,
	}

	if err := s.db.Create(&run).Error; err != nil {
		return uuid.Nil, fmt.Errorf("unable to create test run: %w", err)
	}

	return uuid.Parse(run.UUID)
}

// EndTestRun marks a test run as completed, setting its ended timestamp and optional coverage data.
// When coverage profiles are provided, structured coverage data is stored alongside the aggregate percentage.
func (s Store) EndTestRun(runID uuid.UUID, coverage *CoverageInput) error {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return err
	}
	n := time.Now()
	run.Ended = &n

	if coverage != nil {
		run.Coverage = &coverage.Percent
		run.CoverageDir = coverage.CoverageDir
	}

	if err := s.db.Save(&run).Error; err != nil {
		return fmt.Errorf("unable to end test run: %w", err)
	}

	if coverage != nil {
		if len(coverage.Packages) > 0 {
			if err := s.addPackageCoverage(run.ID, coverage.Packages); err != nil {
				log.WithFields("error", err).Warn("failed to store package coverage")
			}
		}
		if len(coverage.Functions) > 0 {
			if err := s.addFunctionCoverage(run.ID, coverage.Functions); err != nil {
				log.WithFields("error", err).Warn("failed to store function coverage")
			}
		}
	}

	return nil
}

// GetTestRun retrieves a test run by its UUID without loading associated events.
func (s Store) GetTestRun(runID uuid.UUID) (TestRun, error) {
	var run TestRun
	if err := s.db.Where("uuid = ?", runID.String()).First(&run).Error; err != nil {
		return TestRun{}, fmt.Errorf("unable to get test run: %w", err)
	}
	return run, nil
}

// GetTestEvents retrieves all test events for a run, preloading references and annotations.
func (s Store) GetTestEvents(runID uuid.UUID) ([]TestEvent, error) {
	var run TestRun
	if err := s.db.Preload("Events.Reference").Preload("Events.Annotations").Where("uuid = ?", runID.String()).First(&run).Error; err != nil {
		return nil, fmt.Errorf("unable to get test run: %w", err)
	}

	if run.Events == nil {
		return nil, fmt.Errorf("did not attempt to fetch events")
	}

	return *run.Events, nil
}

// GetTestEventsBatch retrieves a batch of test events for a run with pagination support.
// Events are ordered by index for consistent streaming. Returns events, whether there are more, and any error.
func (s Store) GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]TestEvent, bool, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return nil, false, err
	}

	var events []TestEvent
	if err := s.db.Preload("Reference").Preload("Annotations").
		Where("run_id = ?", run.ID).
		Order("\"index\" ASC").
		Offset(offset).
		Limit(limit + 1). // fetch one extra to check if there are more
		Find(&events).Error; err != nil {
		return nil, false, fmt.Errorf("unable to get test events batch: %w", err)
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit] // trim the extra
	}

	return events, hasMore, nil
}

// GetTestEventCount returns the total number of events for a run.
func (s Store) GetTestEventCount(runID uuid.UUID) (int64, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := s.db.Model(&TestEvent{}).Where("run_id = ?", run.ID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("unable to count test events: %w", err)
	}

	return count, nil
}

// GetSessionTestRuns retrieves all test runs for a session.
// If infoOnly is true, events and other associations are omitted for faster queries.
func (s Store) GetSessionTestRuns(sessionID uuid.UUID, infoOnly bool) ([]TestRun, error) {
	var runs []TestRun

	db := s.db
	if infoOnly {
		db = db.Omit(clause.Associations)
	}

	if err := db.Where("session_id = ?", sessionID.String()).Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("unable to get test runs: %w", err)
	}
	return runs, nil
}

// GetOrCreateReference finds an existing test reference or creates a new one if it doesn't exist.
// This ensures references are deduplicated across test runs for historical tracking.
func (s Store) GetOrCreateReference(ref *Reference) error {
	if err := s.db.Where("package = ? AND function = ? AND t_run_name = ?", ref.Package, ref.FuncName, ref.TRunName).FirstOrCreate(ref).Error; err != nil {
		return fmt.Errorf("unable to get or create reference: %w", err)
	}
	return nil
}

// GetOrCreateAnnotation finds an existing annotation or creates a new one if it doesn't exist.
// This ensures annotations are deduplicated across test events.
func (s Store) GetOrCreateAnnotation(annotation *Annotation) error {
	if err := s.db.Where("value = ?", annotation.Value).FirstOrCreate(annotation).Error; err != nil {
		return fmt.Errorf("unable to get or create annotation: %w", err)
	}
	return nil
}

// AddTestEvent creates a new test event record from a gotest.Event.
// It automatically creates or reuses references and converts annotations.
func (s Store) AddTestEvent(runID uuid.UUID, event gotest.Event) error {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return err
	}

	ref := Reference{
		Package:  event.Reference.Package,
		FuncName: event.Reference.FuncName,
		TRunName: event.Reference.TRunName,
	}
	if err := s.GetOrCreateReference(&ref); err != nil {
		return err
	}

	var annotations []Annotation
	for _, a := range event.Annotations {
		annotation := Annotation{Value: string(a)}
		if err := s.GetOrCreateAnnotation(&annotation); err != nil {
			return err
		}
		annotations = append(annotations, annotation)
	}

	var errStr string
	if event.Error != nil {
		errStr = event.Error.Error()
	}

	testEvent := TestEvent{
		Index:       event.Index,
		RunID:       run.ID,
		Reference:   ref,
		Time:        event.Time,
		Action:      string(event.Action),
		Output:      event.Output,
		Elapsed:     event.Elapsed,
		FailedBuild: event.FailedBuild,
		Annotations: annotations,
		Error:       errStr,
	}

	if err := s.db.Create(&testEvent).Error; err != nil {
		return fmt.Errorf("unable to create test event: %w", err)
	}

	// parse and store failure data for fail events
	if event.Action == gotest.FailAction {
		// aggregate output from all output events for this test reference in this run
		aggregatedOutput := s.aggregateTestOutput(run.ID, ref.ID)
		if aggregatedOutput != "" {
			if err := s.addFailureData(testEvent.ID, run.ID, aggregatedOutput); err != nil {
				// log but don't fail the event creation
				log.WithFields("error", err).Debug("failed to parse failure data")
			}
		}
	}

	return nil
}

// aggregateTestOutput collects all output from output events for a specific test reference within a run.
func (s Store) aggregateTestOutput(runID, refID int64) string {
	var events []TestEvent
	if err := s.db.Where("run_id = ? AND reference_id = ? AND action = ?", runID, refID, "output").
		Order("id ASC").Find(&events).Error; err != nil {
		return ""
	}

	var output strings.Builder
	for _, e := range events {
		output.WriteString(e.Output)
	}
	return output.String()
}

// addFailureData parses failure output and stores structured failure data.
func (s Store) addFailureData(eventID, runID int64, output string) error {
	sf := s.failureRegistry.Parse(output)
	if sf == nil {
		return nil
	}

	// marshal the type-specific details
	var detailsJSON []byte
	var err error

	switch sf.FailureType {
	case failure.AssertionFailure:
		if sf.Assertion != nil {
			detailsJSON, err = json.Marshal(sf.Assertion)
		}
	case failure.PanicFailure:
		if sf.Panic != nil {
			detailsJSON, err = json.Marshal(sf.Panic)
		}
	case failure.DiffFailure:
		if sf.Diff != nil {
			detailsJSON, err = json.Marshal(sf.Diff)
		}
	}

	if err != nil {
		return fmt.Errorf("unable to marshal failure details: %w", err)
	}

	failureData := FailedTestDetails{
		EventID:      eventID,
		RunID:        runID,
		Type:         string(sf.FailureType),
		Details:      detailsJSON,
		LocationFile: sf.Location.File,
		LocationLine: sf.Location.Line,
		Fingerprint:  sf.Fingerprint,
	}

	if err := s.db.Create(&failureData).Error; err != nil {
		return fmt.Errorf("unable to create failure data: %w", err)
	}

	return nil
}

// GetFailuresByRun retrieves all failure data for a specific test run.
func (s Store) GetFailuresByRun(runID uuid.UUID) ([]FailedTestDetails, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return nil, err
	}

	var failures []FailedTestDetails
	if err := s.db.Where("run_id = ?", run.ID).Find(&failures).Error; err != nil {
		return nil, fmt.Errorf("unable to get failures by run: %w", err)
	}
	return failures, nil
}

// GetFailuresByFingerprint retrieves all failure data with a specific fingerprint.
// This is useful for finding similar failures across runs for flaky test detection.
func (s Store) GetFailuresByFingerprint(fingerprint string) ([]FailedTestDetails, error) {
	var failures []FailedTestDetails
	if err := s.db.Where("fingerprint = ?", fingerprint).Find(&failures).Error; err != nil {
		return nil, fmt.Errorf("unable to get failures by fingerprint: %w", err)
	}
	return failures, nil
}

// addPackageCoverage stores per-package coverage data for a test run.
func (s Store) addPackageCoverage(runID int64, pkgs []cover.PackageResult) error {
	if len(pkgs) == 0 {
		return nil
	}

	records := make([]PackageCoverage, len(pkgs))
	for i, p := range pkgs {
		records[i] = PackageCoverage{
			RunID:       runID,
			PackagePath: p.PackagePath,
			Percent:     p.Percent,
		}
	}

	if err := s.db.CreateInBatches(records, 500).Error; err != nil {
		return fmt.Errorf("unable to create package coverage: %w", err)
	}

	return nil
}

// addFunctionCoverage stores per-function coverage data for a test run.
func (s Store) addFunctionCoverage(runID int64, funcs []cover.FunctionResult) error {
	if len(funcs) == 0 {
		return nil
	}

	records := make([]FunctionCoverage, len(funcs))
	for i, f := range funcs {
		records[i] = FunctionCoverage{
			RunID:    runID,
			FilePath: f.FilePath,
			Line:     f.Line,
			FuncName: f.FuncName,
			Percent:  f.Percent,
		}
	}

	if err := s.db.CreateInBatches(records, 500).Error; err != nil {
		return fmt.Errorf("unable to create function coverage: %w", err)
	}

	return nil
}

// GetPackageCoverage retrieves per-package coverage data for a test run.
func (s Store) GetPackageCoverage(runID uuid.UUID) ([]PackageCoverage, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return nil, err
	}

	var pkgs []PackageCoverage
	if err := s.db.Where("run_id = ?", run.ID).Find(&pkgs).Error; err != nil {
		return nil, fmt.Errorf("unable to get package coverage: %w", err)
	}
	return pkgs, nil
}

// GetFunctionCoverage retrieves per-function coverage data for a test run.
func (s Store) GetFunctionCoverage(runID uuid.UUID) ([]FunctionCoverage, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return nil, err
	}

	var funcs []FunctionCoverage
	if err := s.db.Where("run_id = ?", run.ID).Find(&funcs).Error; err != nil {
		return nil, fmt.Errorf("unable to get function coverage: %w", err)
	}
	return funcs, nil
}

// AddSourceState stores source state data for a test run.
func (s Store) AddSourceState(runID uuid.UUID, state *SourceStateInput) error {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return err
	}

	ss := SourceState{
		RunID:  run.ID,
		Commit: state.Commit,
		Branch: state.Branch,
		Dirty:  state.Dirty,
	}

	if err := s.db.Create(&ss).Error; err != nil {
		return fmt.Errorf("unable to create source state: %w", err)
	}

	if len(state.DirtyFiles) > 0 {
		files := make([]FileState, len(state.DirtyFiles))
		for i, f := range state.DirtyFiles {
			files[i] = FileState{
				SourceStateID: ss.ID,
				Path:          f.Path,
				ContentHash:   f.ContentHash,
				ModTime:       f.ModTime,
			}
		}

		if err := s.db.CreateInBatches(files, 500).Error; err != nil {
			return fmt.Errorf("unable to create file states: %w", err)
		}
	}

	return nil
}

// GetSourceState retrieves source state data for a test run, including dirty files.
// Returns nil if no source state exists for the run.
func (s Store) GetSourceState(runID uuid.UUID) (*SourceState, error) {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return nil, err
	}

	var ss SourceState
	if err := s.db.Preload("DirtyFiles").Where("run_id = ?", run.ID).First(&ss).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get source state: %w", err)
	}
	return &ss, nil
}

// open creates a new connection to a SQLite database file.
// It creates parent directories if necessary and applies custom logger configuration.
func open(path string) (*gorm.DB, error) {
	log.WithFields("path", path).Debug("opening db connection")

	connStr, err := connectionString(path)
	if err != nil {
		return nil, err
	}

	if path != ":memory:" {
		parentDir := filepath.Dir(path)
		err = os.MkdirAll(parentDir, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("unable to create db directory: %w", err)
		}
	}

	dbObj, err := gorm.Open(sqlite.Open(connStr), &gorm.Config{Logger: newLogger()})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to DB: %w", err)
	}

	return dbObj, nil
}

// deleteRun removes a single test run and all its associated data within an existing transaction.
// The caller is responsible for managing the transaction boundary.
// When adding new run-related tables, add a corresponding DELETE here.
func (s Store) deleteRun(tx *gorm.DB, runID int64) error {
	// 1. package coverage
	if err := tx.Where("run_id = ?", runID).Delete(&PackageCoverage{}).Error; err != nil {
		return fmt.Errorf("unable to delete package coverage for run %d: %w", runID, err)
	}

	// 2. function coverage
	if err := tx.Where("run_id = ?", runID).Delete(&FunctionCoverage{}).Error; err != nil {
		return fmt.Errorf("unable to delete function coverage for run %d: %w", runID, err)
	}

	// 3. failed test details
	if err := tx.Where("run_id = ?", runID).Delete(&FailedTestDetails{}).Error; err != nil {
		return fmt.Errorf("unable to delete failed test details for run %d: %w", runID, err)
	}

	// 4. test_event_annotations join table (many2many between TestEvent and Annotation).
	// This is an implicit GORM join table with no model, so we reference the table name directly
	// but still use a GORM subquery for the IN clause.
	eventIDs := tx.Model(&TestEvent{}).Select("id").Where("run_id = ?", runID)
	if err := tx.Exec("DELETE FROM test_event_annotations WHERE test_event_id IN (?)", eventIDs).Error; err != nil {
		return fmt.Errorf("unable to delete test event annotations for run %d: %w", runID, err)
	}

	// 5. test events
	if err := tx.Where("run_id = ?", runID).Delete(&TestEvent{}).Error; err != nil {
		return fmt.Errorf("unable to delete test events for run %d: %w", runID, err)
	}

	// 6. the test run itself
	if err := tx.Where("id = ?", runID).Delete(&TestRun{}).Error; err != nil {
		return fmt.Errorf("unable to delete test run %d: %w", runID, err)
	}

	return nil
}

// DeleteRuns removes test runs by internal IDs and cascades to all child data.
// Also removes persistent coverage directories from disk.
func (s Store) DeleteRuns(runIDs []int64) (int, error) {
	if len(runIDs) == 0 {
		return 0, nil
	}

	// collect coverage directory paths before deleting the DB records
	var coverageDirs []string
	var runs []TestRun
	if err := s.db.Where("id IN ?", runIDs).Find(&runs).Error; err == nil {
		for _, r := range runs {
			if r.CoverageDir != "" {
				coverageDirs = append(coverageDirs, r.CoverageDir)
			}
		}
	}

	deleted := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range runIDs {
			if err := s.deleteRun(tx, id); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return deleted, err
	}

	// clean up persistent coverage directories from disk
	for _, dir := range coverageDirs {
		if err := os.RemoveAll(dir); err != nil {
			log.WithFields("dir", dir, "error", err).Debug("failed to remove coverage directory")
		}
	}

	if _, err := s.DeleteOrphanedSessions(); err != nil {
		log.WithFields("error", err).Warn("failed to clean up orphaned sessions")
	}

	return deleted, nil
}

// DeleteRunsByAge removes test runs older than the given duration.
// Returns the number of runs deleted.
func (s Store) DeleteRunsByAge(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge)

	var runs []TestRun
	if err := s.db.Where("started < ?", cutoff).Find(&runs).Error; err != nil {
		return 0, fmt.Errorf("unable to find runs older than %s: %w", maxAge, err)
	}

	if len(runs) == 0 {
		return 0, nil
	}

	ids := make([]int64, len(runs))
	for i, r := range runs {
		ids[i] = r.ID
	}

	return s.DeleteRuns(ids)
}

// DeleteRunsKeepingLast removes all but the N most recent test runs (by start time).
// Returns the number of runs deleted.
func (s Store) DeleteRunsKeepingLast(keep int) (int, error) {
	if keep < 0 {
		keep = 0
	}

	var runs []TestRun
	if err := s.db.Order("started DESC").Find(&runs).Error; err != nil {
		return 0, fmt.Errorf("unable to list runs for pruning: %w", err)
	}

	if len(runs) <= keep {
		return 0, nil
	}

	toDelete := runs[keep:]
	ids := make([]int64, len(toDelete))
	for i, r := range toDelete {
		ids[i] = r.ID
	}

	return s.DeleteRuns(ids)
}

// DeleteAllRuns removes all test runs and associated data.
// Returns the number of runs deleted.
func (s Store) DeleteAllRuns() (int, error) {
	var runs []TestRun
	if err := s.db.Find(&runs).Error; err != nil {
		return 0, fmt.Errorf("unable to list all runs: %w", err)
	}

	if len(runs) == 0 {
		return 0, nil
	}

	ids := make([]int64, len(runs))
	for i, r := range runs {
		ids[i] = r.ID
	}

	return s.DeleteRuns(ids)
}

// DeleteOrphanedSessions removes sessions that have no remaining test runs.
func (s Store) DeleteOrphanedSessions() (int, error) {
	activeSessionIDs := s.db.Model(&TestRun{}).Select("DISTINCT session_id")
	result := s.db.Where("id NOT IN (?)", activeSessionIDs).Delete(&TestSession{})
	if result.Error != nil {
		return 0, fmt.Errorf("unable to delete orphaned sessions: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

// CountRuns returns the total number of test runs in the database.
func (s Store) CountRuns() (int64, error) {
	var count int64
	if err := s.db.Model(&TestRun{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("unable to count runs: %w", err)
	}
	return count, nil
}

// CountRunsByAge returns the number of test runs older than the given duration.
func (s Store) CountRunsByAge(maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	var count int64
	if err := s.db.Model(&TestRun{}).Where("started < ?", cutoff).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("unable to count runs by age: %w", err)
	}
	return count, nil
}

// CountRunsBeyondKeep returns the number of test runs that would be removed to keep only N most recent.
func (s Store) CountRunsBeyondKeep(keep int) (int64, error) {
	var total int64
	if err := s.db.Model(&TestRun{}).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("unable to count runs: %w", err)
	}
	excess := total - int64(keep)
	if excess < 0 {
		return 0, nil
	}
	return excess, nil
}

// Vacuum reclaims disk space after deletions.
func (s Store) Vacuum() error {
	if err := s.db.Exec("VACUUM").Error; err != nil {
		return fmt.Errorf("unable to vacuum database: %w", err)
	}
	return nil
}

// connectionString creates a SQLite connection string from a file path.
// It supports ":memory:" for in-memory databases and file paths with shared cache mode.
func connectionString(path string) (string, error) {
	if path == ":memory:" {
		return path, nil
	}
	if path == "" {
		return "", fmt.Errorf("no db filepath given")
	}
	return fmt.Sprintf("file:%s?cache=shared", path), nil
}
