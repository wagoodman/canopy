package gotest

import (
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

type Event struct {
	Index          int64
	RunID          uuid.UUID
	JSONL          string
	PackageDirPath string
	Time           time.Time
	Reference      Reference
	Action         Action
	Output         string
	Annotations    []Annotation // TODO: maybe make this a map?
	Error          error
}

func (e Event) HasAnnotation(a Annotation) bool {
	for _, ann := range e.Annotations {
		if ann == a {
			return true
		}
	}
	return false
}

func (e Event) Copy() Event {
	return e
}

func NewEvent(runID uuid.UUID, jsonl JSONL, pkgs *golist.PackageCollection) *Event {
	if jsonl.Error != nil {
		return &Event{Index: math.MaxInt64, Error: jsonl.Error}
	}

	timestamp, err := time.Parse(time.RFC3339Nano, jsonl.Time)

	var dir string
	if pkgs != nil {
		dir = pkgs.GetDir(jsonl.Package)
	}

	return &Event{
		Index:          jsonl.Index,
		RunID:          runID,
		JSONL:          jsonl.Raw,
		Time:           timestamp,
		PackageDirPath: dir,
		Reference:      NewReference(jsonl.Package, jsonl.Test),
		Action:         ParseAction(jsonl.Action),
		Annotations:    ExtractAnnotations(jsonl.Output),
		Output:         jsonl.Output,
		Error:          err,
	}
}
