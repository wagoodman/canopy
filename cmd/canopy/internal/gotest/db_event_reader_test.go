package gotest

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mockFetcher implements EventBatchFetcher for testing.
type mockFetcher struct {
	events    []Event
	batchSize int
}

func (m *mockFetcher) GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]Event, bool, error) {
	if offset >= len(m.events) {
		return nil, false, nil
	}

	end := offset + limit
	if end > len(m.events) {
		end = len(m.events)
	}

	return m.events[offset:end], end < len(m.events), nil
}

func TestDBEventReader_Basic(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	events := []Event{
		{
			Index: 1,
			Time:  now,
			Reference: Reference{
				Package:  "github.com/example/pkg",
				FuncName: "TestFoo",
			},
			Action: RunAction,
		},
		{
			Index:  2,
			Time:   now.Add(time.Millisecond),
			Action: OutputAction,
			Reference: Reference{
				Package:  "github.com/example/pkg",
				FuncName: "TestFoo",
			},
			Output: "=== RUN   TestFoo\n",
		},
		{
			Index: 3,
			Time:  now.Add(10 * time.Millisecond),
			Reference: Reference{
				Package:  "github.com/example/pkg",
				FuncName: "TestFoo",
			},
			Action: PassAction,
		},
	}

	fetcher := &mockFetcher{events: events}
	reader := NewDBEventReader(fetcher, uuid.New(), EventFilter{}, 10)

	// read all lines
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.NoError(t, scanner.Err())
	require.Len(t, lines, 3)

	// verify actions
	var first, second, third jsonlEvent
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &third))

	require.Equal(t, "run", first.Action)
	require.Equal(t, "output", second.Action)
	require.Equal(t, "pass", third.Action)
}

func TestDBEventReader_Batching(t *testing.T) {
	now := time.Now()

	// create more events than one batch
	var events []Event
	for i := 0; i < 25; i++ {
		events = append(events, Event{
			Index: int64(i + 1),
			Time:  now.Add(time.Duration(i) * time.Millisecond),
			Reference: Reference{
				Package:  "github.com/example/pkg",
				FuncName: "TestFoo",
			},
			Action: OutputAction,
			Output: "line\n",
		})
	}

	fetcher := &mockFetcher{events: events}
	// use small batch size to test batching
	reader := NewDBEventReader(fetcher, uuid.New(), EventFilter{}, 10)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	// count lines (should be 25)
	lineCount := 0
	for _, b := range data {
		if b == '\n' {
			lineCount++
		}
	}
	require.Equal(t, 25, lineCount)
}

func TestDBEventReader_FilterByStatus(t *testing.T) {
	now := time.Now()

	events := []Event{
		// passing test
		{Index: 1, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestPass"}, Action: RunAction},
		{Index: 2, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestPass"}, Action: PassAction},
		// failing test
		{Index: 3, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestFail"}, Action: RunAction},
		{Index: 4, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestFail"}, Action: FailAction},
		// skipped test
		{Index: 5, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestSkip"}, Action: RunAction},
		{Index: 6, Time: now, Reference: Reference{Package: "pkg", FuncName: "TestSkip"}, Action: SkipAction},
	}

	tests := []struct {
		name          string
		filter        EventFilter
		expectedTests []string
	}{
		{
			name:          "no filter",
			filter:        EventFilter{},
			expectedTests: []string{"TestPass", "TestPass", "TestFail", "TestFail", "TestSkip", "TestSkip"},
		},
		{
			name:          "failed only",
			filter:        EventFilter{Failed: true},
			expectedTests: []string{"TestFail", "TestFail"},
		},
		{
			name:          "passed only",
			filter:        EventFilter{Passed: true},
			expectedTests: []string{"TestPass", "TestPass"},
		},
		{
			name:          "skipped only",
			filter:        EventFilter{Skipped: true},
			expectedTests: []string{"TestSkip", "TestSkip"},
		},
		{
			name:          "passed or failed",
			filter:        EventFilter{Passed: true, Failed: true},
			expectedTests: []string{"TestPass", "TestPass", "TestFail", "TestFail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &mockFetcher{events: events}
			reader := NewDBEventReader(fetcher, uuid.New(), tt.filter, 100)

			scanner := bufio.NewScanner(reader)
			var foundTests []string
			for scanner.Scan() {
				var je jsonlEvent
				require.NoError(t, json.Unmarshal([]byte(scanner.Text()), &je))
				foundTests = append(foundTests, je.Test)
			}
			require.NoError(t, scanner.Err())
			require.Equal(t, tt.expectedTests, foundTests)
		})
	}
}

func TestDBEventReader_FilterByPattern(t *testing.T) {
	now := time.Now()

	events := []Event{
		{Index: 1, Time: now, Reference: Reference{Package: "github.com/foo/bar", FuncName: "TestOne"}, Action: RunAction},
		{Index: 2, Time: now, Reference: Reference{Package: "github.com/foo/bar", FuncName: "TestOne"}, Action: PassAction},
		{Index: 3, Time: now, Reference: Reference{Package: "github.com/foo/baz", FuncName: "TestTwo"}, Action: RunAction},
		{Index: 4, Time: now, Reference: Reference{Package: "github.com/foo/baz", FuncName: "TestTwo"}, Action: PassAction},
		{Index: 5, Time: now, Reference: Reference{Package: "github.com/other/pkg", FuncName: "TestThree"}, Action: RunAction},
		{Index: 6, Time: now, Reference: Reference{Package: "github.com/other/pkg", FuncName: "TestThree"}, Action: PassAction},
	}

	tests := []struct {
		name          string
		filter        EventFilter
		expectedTests []string
	}{
		{
			name:          "filter by test name pattern",
			filter:        EventFilter{TestPattern: "TestOne"},
			expectedTests: []string{"TestOne", "TestOne"},
		},
		{
			name:          "filter by package pattern with ...",
			filter:        EventFilter{PackagePattern: "github.com/foo/..."},
			expectedTests: []string{"TestOne", "TestOne", "TestTwo", "TestTwo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := &mockFetcher{events: events}
			reader := NewDBEventReader(fetcher, uuid.New(), tt.filter, 100)

			scanner := bufio.NewScanner(reader)
			var foundTests []string
			for scanner.Scan() {
				var je jsonlEvent
				require.NoError(t, json.Unmarshal([]byte(scanner.Text()), &je))
				foundTests = append(foundTests, je.Test)
			}
			require.NoError(t, scanner.Err())
			require.Equal(t, tt.expectedTests, foundTests)
		})
	}
}

func TestDBEventReader_Empty(t *testing.T) {
	fetcher := &mockFetcher{events: nil}
	reader := NewDBEventReader(fetcher, uuid.New(), EventFilter{}, 10)

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Empty(t, data)
}
