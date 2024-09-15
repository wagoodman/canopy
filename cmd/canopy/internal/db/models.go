package db

import (
	"time"

	"gorm.io/datatypes"
)

const Version = "v1"

func models() []any {
	return []any{
		TestSession{},
		TestRun{},
		TestEvent{},
		Reference{},
		Annotation{},
	}
}

type TestSession struct {
	ID int64 `gorm:"primaryKey" json:"-"`

	UUID     string     `gorm:"column:uuid;index" json:"uuid"`
	Started  time.Time  `gorm:"column:started" json:"started"`
	Ended    *time.Time `gorm:"column:ended" json:"ended"`
	TestRuns *[]TestRun `gorm:"foreignKey:SessionID" json:"test_runs"`
}

type TestRun struct {
	ID        int64 `gorm:"primaryKey" json:"-"`
	SessionID int64 `gorm:"column:session_id" json:"-"`

	UUID     string         `gorm:"column:uuid;index" json:"uuid"`
	Started  time.Time      `gorm:"column:started" json:"started"`
	Ended    *time.Time     `gorm:"column:ended" json:"ended"`
	Config   datatypes.JSON `gorm:"column:config" json:"config"`
	Events   *[]TestEvent   `gorm:"foreignKey:RunID" json:"events"`
	Coverage *float64       `gorm:"column:coverage" json:"coverage"`
}

type TestEvent struct {
	ID    int64 `gorm:"primaryKey" json:"-"`
	RunID int64 `gorm:"column:run_id" json:"-"`

	Index int64 `gorm:"column:index" json:"index"`

	ReferenceID int64 `gorm:"column:reference_id" json:"-"`
	Reference   Reference

	Time   time.Time `gorm:"column:time" json:"time"`
	Action string    `gorm:"column:action" json:"action"`
	Output string    `gorm:"column:output" json:"output"`

	Annotations []Annotation `gorm:"many2many:test_event_annotations" json:"annotations"`
	Error       string       `gorm:"column:error" json:"error"`
}

type Reference struct {
	ID int64 `gorm:"primaryKey" json:"-"`

	Package  string `gorm:"column:package;index" json:"package"`       // the go package path
	FuncName string `gorm:"column:function;index" json:"function"`     // the test function name
	TRunName string `gorm:"column:t_run_name;index" json:"t_run_name"` // instance of t.Run within the function
}

type Annotation struct {
	ID int64 `gorm:"primaryKey" json:"-"`

	Value string `gorm:"column:value" json:"value"`
}
