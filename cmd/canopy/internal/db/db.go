package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

// New creates a new instance of the store.
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

	// probably not safe... should reconsider this
	db.Exec("PRAGMA synchronous = OFF")
	db.Exec("PRAGMA journal_mode = OFF")
	db.Exec("PRAGMA temp_store = MEMORY")
	db.Exec("PRAGMA cache_size = 100000")
	db.Exec("PRAGMA mmap_size = 268435456") // 256 MB
	// db.Exec("PRAGMA auto_vacuum = NONE")

	return &Store{
		db: db,
	}, nil
}

func (s Store) GetTestSession(sessionID uuid.UUID) (TestSession, error) {
	var session TestSession
	if err := s.db.Preload("TestRuns").Where("uuid = ?", sessionID.String()).First(&session).Error; err != nil {
		return TestSession{}, fmt.Errorf("unable to get test session: %w", err)
	}
	return session, nil
}

func (s Store) GetTestSessions() ([]TestSession, error) {
	var sessions []TestSession
	if err := s.db.Preload("TestRuns").Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("unable to get test sessions: %w", err)
	}
	return sessions, nil
}

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

func (s Store) EndTestRun(runID uuid.UUID, coverage *float64) error {
	run, err := s.GetTestRun(runID)
	if err != nil {
		return err
	}
	n := time.Now()
	run.Ended = &n
	run.Coverage = coverage
	if err := s.db.Save(&run).Error; err != nil {
		return fmt.Errorf("unable to end test run: %w", err)
	}
	return nil
}

func (s Store) GetTestRun(runID uuid.UUID) (TestRun, error) {
	var run TestRun
	if err := s.db.Where("uuid = ?", runID.String()).First(&run).Error; err != nil {
		return TestRun{}, fmt.Errorf("unable to get test run: %w", err)
	}
	return run, nil
}

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

func (s Store) GetOrCreateReference(ref *Reference) error {
	if err := s.db.Where("package = ? AND function = ? AND t_run_name = ?", ref.Package, ref.FuncName, ref.TRunName).FirstOrCreate(ref).Error; err != nil {
		return fmt.Errorf("unable to get or create reference: %w", err)
	}
	return nil
}

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
		annotations = append(annotations, Annotation{Value: string(a)})
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
		Annotations: annotations,
		Error:       errStr,
	}

	if err := s.db.Create(&testEvent).Error; err != nil {
		return fmt.Errorf("unable to create test event: %w", err)
	}

	return nil
}

// open a new connection to a sqlite3 database file
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

// connectionString creates a connection string for sqlite3
func connectionString(path string) (string, error) {
	if path == ":memory:" {
		return path, nil
	}
	if path == "" {
		return "", fmt.Errorf("no db filepath given")
	}
	return fmt.Sprintf("file:%s?cache=shared", path), nil
}
