package gotest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONL(t *testing.T) {
	tests := []struct {
		name     string
		ogLine   string
		idx      int64
		expected JSONL
	}{
		{
			name:   "valid JSONL",
			ogLine: `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
			idx:    1,
			expected: JSONL{
				Index:   1,
				Raw:     `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"github.com/example/project","Test":"TestExample"}`,
				Time:    "2024-01-01T12:00:00Z",
				Action:  "run",
				Package: "github.com/example/project",
				Test:    "TestExample",
			},
		},
		{
			name:   "fail with setup failed",
			ogLine: "FAIL    github.com/example/project [setup failed]",
			idx:    2,
			expected: JSONL{
				Index:   2,
				Raw:     "FAIL    github.com/example/project [setup failed]",
				Time:    "", // dynamic value, not checking this field in test
				Action:  "fail",
				Package: "github.com/example/project",
				Test:    "",
				Output:  "FAIL    github.com/example/project [setup failed]",
			},
		},
		{
			name:   "fail with build failed",
			ogLine: "FAIL    github.com/example/project [build failed]",
			idx:    3,
			expected: JSONL{
				Index:   3,
				Raw:     "FAIL    github.com/example/project [build failed]",
				Time:    "", // dynamic value, not checking this field in test
				Action:  "fail",
				Package: "github.com/example/project",
				Test:    "",
				Output:  "FAIL    github.com/example/project [build failed]",
			},
		},
		{
			name:   "invalid JSONL",
			ogLine: "invalid JSONL",
			idx:    4,
			expected: JSONL{
				Index: 4,
				Raw:   "invalid JSONL",
				Error: fmt.Errorf("unable to unmarshal go test JSONL: invalid character 'i' looking for beginning of value"),
			},
		},
		{
			name:   "build-output with ImportPath",
			ogLine: `{"ImportPath":"internal/unsafeheader","Action":"build-output","Output":"# internal/unsafeheader\n"}`,
			idx:    5,
			expected: JSONL{
				Index:      5,
				Raw:        `{"ImportPath":"internal/unsafeheader","Action":"build-output","Output":"# internal/unsafeheader\n"}`,
				Action:     "build-output",
				ImportPath: "internal/unsafeheader",
				Output:     "# internal/unsafeheader\n",
			},
		},
		{
			name:   "build-output with compiler error",
			ogLine: `{"ImportPath":"internal/unsafeheader","Action":"build-output","Output":"compile: version \"go1.24.4\" does not match go tool version \"go1.24.9\"\n"}`,
			idx:    6,
			expected: JSONL{
				Index:      6,
				Raw:        `{"ImportPath":"internal/unsafeheader","Action":"build-output","Output":"compile: version \"go1.24.4\" does not match go tool version \"go1.24.9\"\n"}`,
				Action:     "build-output",
				ImportPath: "internal/unsafeheader",
				Output:     "compile: version \"go1.24.4\" does not match go tool version \"go1.24.9\"\n",
			},
		},
		{
			name:   "build-fail with ImportPath",
			ogLine: `{"ImportPath":"internal/unsafeheader","Action":"build-fail"}`,
			idx:    7,
			expected: JSONL{
				Index:      7,
				Raw:        `{"ImportPath":"internal/unsafeheader","Action":"build-fail"}`,
				Action:     "build-fail",
				ImportPath: "internal/unsafeheader",
			},
		},
		{
			name:   "fail with FailedBuild",
			ogLine: `{"Time":"2026-01-20T10:42:43.974154-05:00","Action":"fail","Package":"github.com/wagoodman/canopy/cmd/canopy","Elapsed":0,"FailedBuild":"github.com/lindell/go-ordered-set/orderedset"}`,
			idx:    8,
			expected: JSONL{
				Index:       8,
				Raw:         `{"Time":"2026-01-20T10:42:43.974154-05:00","Action":"fail","Package":"github.com/wagoodman/canopy/cmd/canopy","Elapsed":0,"FailedBuild":"github.com/lindell/go-ordered-set/orderedset"}`,
				Time:        "2026-01-20T10:42:43.974154-05:00",
				Action:      "fail",
				Package:     "github.com/wagoodman/canopy/cmd/canopy",
				FailedBuild: "github.com/lindell/go-ordered-set/orderedset",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewJSONL(tt.ogLine, tt.idx)

			// if time is dynamically generated, skip checking this field
			if tt.expected.Time == "" {
				tt.expected.Time = result.Time
			}

			if tt.expected.Error != nil {
				assert.Error(t, result.Error)
				assert.EqualError(t, result.Error, tt.expected.Error.Error())
				tt.expected.Error = nil
				result.Error = nil
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONL_String(t *testing.T) {
	tests := []struct {
		name     string
		jsonl    JSONL
		expected string
	}{
		{
			name: "no error",
			jsonl: JSONL{
				Package: "github.com/example/project",
				Test:    "TestExample",
				Action:  "pass",
				Output:  "example output",
			},
			expected: `github.com/example/project(TestExample): pass "example output"`,
		},
		{
			name: "with error",
			jsonl: JSONL{
				Error: fmt.Errorf("sample error"),
			},
			expected: "error: sample error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.jsonl.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
