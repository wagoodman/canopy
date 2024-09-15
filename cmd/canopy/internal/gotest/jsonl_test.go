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
				Error: fmt.Errorf("error unmarshalling go test JSONL: invalid character 'i' looking for beginning of value"),
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
