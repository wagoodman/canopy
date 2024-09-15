package gotest

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestEvent_HasAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		event      Event
		annotation Annotation
		hasAnn     bool
	}{
		{
			name: "annotation exists",
			event: Event{
				Annotations: []Annotation{"test-annotation"},
			},
			annotation: "test-annotation",
			hasAnn:     true,
		},
		{
			name: "annotation does not exist",
			event: Event{
				Annotations: []Annotation{"different-annotation"},
			},
			annotation: "test-annotation",
			hasAnn:     false,
		},
		{
			name:       "empty annotations",
			event:      Event{},
			annotation: "test-annotation",
			hasAnn:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.HasAnnotation(tt.annotation)
			assert.Equal(t, tt.hasAnn, result)
		})
	}
}

func TestEvent_Copy(t *testing.T) {
	event := Event{
		Index:          1,
		RunID:          uuid.New(),
		JSONL:          `{"Time":"2024-01-01T12:00:00Z","Action":"run"}`,
		PackageDirPath: "/path/to/package",
		Time:           time.Now(),
		Action:         Action("run"),
		Output:         "output",
		Annotations:    []Annotation{"test-annotation"},
	}

	copiedEvent := event.Copy()
	assert.Equal(t, event, copiedEvent)
}

func TestNewEvent(t *testing.T) {
	tests := []struct {
		name          string
		runID         uuid.UUID
		jsonl         JSONL
		pkgs          *golist.PackageCollection
		expectedEvent *Event
	}{
		{
			name:  "valid event creation",
			runID: uuid.New(),
			jsonl: JSONL{
				Index:   1,
				Raw:     `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
				Time:    "2024-01-01T12:00:00Z",
				Action:  "run",
				Package: "github.com/example/project",
				Test:    "TestExample",
				Output:  "output",
			},
			pkgs: golist.NewPackageCollection(
				golist.Package{
					ImportPath: "github.com/example/project",
					Dir:        "/path/to/package",
				},
			),
			expectedEvent: &Event{
				Index:          1,
				RunID:          uuid.Nil, // will compare separately
				JSONL:          `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
				PackageDirPath: "/path/to/package",
				Time:           time.Date(2024, 01, 01, 12, 0, 0, 0, time.UTC),
				Reference:      NewReference("github.com/example/project", "TestExample"),
				Action:         Action("run"),
				Output:         "output",
				Annotations:    ExtractAnnotations("output"),
			},
		},
		{
			name:  "event creation with error in JSONL",
			runID: uuid.New(),
			jsonl: JSONL{
				Error: errors.New("sample error"),
			},
			pkgs: nil,
			expectedEvent: &Event{
				Index: math.MaxInt64,
				Error: errors.New("sample error"),
			},
		},
		{
			name:  "event creation with invalid timestamp",
			runID: uuid.New(),
			jsonl: JSONL{
				Index:   1,
				Raw:     `{"Time":"invalid time","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
				Time:    "invalid time",
				Action:  "run",
				Package: "github.com/example/project",
				Test:    "TestExample",
				Output:  "output",
			},
			pkgs: nil,
			expectedEvent: &Event{
				Index:  1,
				RunID:  uuid.Nil, // will compare separately
				JSONL:  `{"Time":"invalid time","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
				Time:   time.Time{},
				Action: Action("run"),
				Output: "output",
				Reference: Reference{
					Package:  "github.com/example/project",
					FuncName: "TestExample",
				},
				Error: &time.ParseError{
					Layout:     "2006-01-02T15:04:05.999999999Z07:00",
					Value:      "invalid time",
					LayoutElem: "2006",
					ValueElem:  "invalid time",
					Message:    "",
				},
				Annotations: ExtractAnnotations("output"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewEvent(tt.runID, tt.jsonl, tt.pkgs)

			// compare all fields except RunID
			if tt.expectedEvent.RunID == uuid.Nil {
				tt.expectedEvent.RunID = result.RunID
			}

			assert.Equal(t, tt.expectedEvent, result)
		})
	}
}
