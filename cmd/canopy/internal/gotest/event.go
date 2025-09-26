package gotest

import (
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

// Event represents a single test execution event parsed from `go test -json` output.
// Events are immutable once created and form the basis for all test state tracking
// and result aggregation within the system.
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

// HasAnnotation checks if the event contains any of the specified annotations.
// Useful for filtering events based on special conditions like build failures or cached results.
func (e Event) HasAnnotation(as ...Annotation) bool {
	for _, ann := range e.Annotations {
		for _, a := range as {
			if ann == a {
				return true
			}
		}
	}
	return false
}

// Copy creates a shallow copy of the event. Since events are immutable after creation,
// this is safe and efficient for passing events between goroutines.
func (e Event) Copy() Event {
	return e
}

// NewEvent constructs an Event from raw go test JSON output, enriching it with
// package directory information and parsed annotations. If the JSONL contains
// an error, returns an event with that error and a maximum index for ordering.
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
