package gotest

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// jsonlEvent is the JSON structure expected by go test -json output.
// this is used to serialize events back to JSONL format for replay.
type jsonlEvent struct {
	Time        string  `json:"Time"`
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test,omitempty"`
	Output      string  `json:"Output,omitempty"`
	Elapsed     float64 `json:"Elapsed,omitempty"`
	FailedBuild string  `json:"FailedBuild,omitempty"`
}

// EventBatchFetcher is an interface for fetching events from a data store in batches.
type EventBatchFetcher interface {
	// GetTestEventsBatch retrieves a batch of events with pagination.
	// Returns events, whether there are more events, and any error.
	GetTestEventsBatch(runID uuid.UUID, offset, limit int) ([]Event, bool, error)
}

// EventFilter defines criteria for filtering events during streaming.
type EventFilter struct {
	// Failed filters to only tests that failed
	Failed bool
	// Passed filters to only tests that passed
	Passed bool
	// Skipped filters to only tests that were skipped
	Skipped bool
	// TestPattern is a glob pattern to match test names
	TestPattern string
	// PackagePattern is a glob pattern to match package paths
	PackagePattern string
}

// IsEmpty returns true if no filters are configured.
func (f EventFilter) IsEmpty() bool {
	return !f.Failed && !f.Passed && !f.Skipped && f.TestPattern == "" && f.PackagePattern == ""
}

// DBEventReader streams events from a database in batches, converting them to JSONL format.
// It implements io.ReadCloser for use with the existing format pipeline.
type DBEventReader struct {
	fetcher   EventBatchFetcher
	runID     uuid.UUID
	filter    EventFilter
	batchSize int

	// state
	offset      int
	hasMore     bool
	initialized bool
	done        bool
	buf         *bytes.Buffer

	// for status filtering, we need to track terminal states
	// this requires a pre-scan if status filters are active
	matchingRefs map[string]bool
	statusScan   bool
}

// NewDBEventReader creates a reader that streams events from the database as JSONL.
// Events are fetched in batches to avoid loading all events into memory.
func NewDBEventReader(fetcher EventBatchFetcher, runID uuid.UUID, filter EventFilter, batchSize int) *DBEventReader {
	if batchSize <= 0 {
		batchSize = 1000
	}

	return &DBEventReader{
		fetcher:      fetcher,
		runID:        runID,
		filter:       filter,
		batchSize:    batchSize,
		hasMore:      true,
		buf:          &bytes.Buffer{},
		matchingRefs: make(map[string]bool),
		statusScan:   filter.Failed || filter.Passed || filter.Skipped,
	}
}

// Read implements io.Reader by streaming events as JSON lines.
func (r *DBEventReader) Read(p []byte) (int, error) {
	// if we have buffered data, read from it first
	if r.buf.Len() > 0 {
		return r.buf.Read(p)
	}

	if r.done {
		return 0, io.EOF
	}

	// initialize on first read (pre-scan for status filters if needed)
	if !r.initialized {
		if err := r.initialize(); err != nil {
			return 0, err
		}
		r.initialized = true
	}

	// fetch and serialize the next batch
	if err := r.fetchNextBatch(); err != nil {
		return 0, err
	}

	if r.buf.Len() == 0 {
		r.done = true
		return 0, io.EOF
	}

	return r.buf.Read(p)
}

// initialize performs any setup needed before streaming, including
// pre-scanning for status filters if needed.
func (r *DBEventReader) initialize() error {
	if !r.statusScan {
		return nil
	}

	// pre-scan all events to identify terminal states for status filtering
	// this is necessary because we need to know which tests passed/failed/skipped
	// before we can filter events for those tests
	offset := 0
	for {
		events, hasMore, err := r.fetcher.GetTestEventsBatch(r.runID, offset, r.batchSize)
		if err != nil {
			return err
		}

		for _, e := range events {
			refKey := e.Reference.String(false)
			switch e.Action {
			case PassAction:
				if r.filter.Passed {
					r.matchingRefs[refKey] = true
				}
			case FailAction:
				if r.filter.Failed {
					r.matchingRefs[refKey] = true
				}
			case SkipAction:
				if r.filter.Skipped {
					r.matchingRefs[refKey] = true
				}
			}
		}

		if !hasMore {
			break
		}
		offset += len(events)
	}

	return nil
}

// fetchNextBatch fetches the next batch of events and serializes them to the buffer.
func (r *DBEventReader) fetchNextBatch() error {
	if !r.hasMore {
		return nil
	}

	events, hasMore, err := r.fetcher.GetTestEventsBatch(r.runID, r.offset, r.batchSize)
	if err != nil {
		return err
	}

	r.hasMore = hasMore
	r.offset += len(events)

	for _, e := range events {
		if !r.shouldInclude(e) {
			continue
		}

		if err := r.serializeEvent(e); err != nil {
			return err
		}
	}

	return nil
}

// shouldInclude checks if an event passes all configured filters.
func (r *DBEventReader) shouldInclude(e Event) bool {
	// check status filters (if configured)
	if r.statusScan {
		refKey := e.Reference.String(false)
		if !r.matchingRefs[refKey] {
			return false
		}
	}

	// check package pattern
	if r.filter.PackagePattern != "" && !matchPattern(r.filter.PackagePattern, e.Reference.Package) {
		return false
	}

	// check test name pattern
	if r.filter.TestPattern != "" {
		testName := e.Reference.TestName(false)
		if testName == "" {
			// package-level event, include if package matches or no package filter
			return r.filter.PackagePattern == "" || matchPattern(r.filter.PackagePattern, e.Reference.Package)
		}
		if !matchPattern(r.filter.TestPattern, testName) {
			return false
		}
	}

	return true
}

// serializeEvent converts an event to JSONL format and writes it to the buffer.
func (r *DBEventReader) serializeEvent(e Event) error {
	je := jsonlEvent{
		Time:        e.Time.Format("2006-01-02T15:04:05.999999999Z07:00"),
		Action:      string(e.Action),
		Package:     e.Reference.Package,
		Test:        e.Reference.TestName(false),
		Output:      e.Output,
		FailedBuild: e.FailedBuild,
	}

	if e.Elapsed != nil {
		je.Elapsed = *e.Elapsed
	}

	data, err := json.Marshal(je)
	if err != nil {
		return err
	}

	r.buf.Write(data)
	r.buf.WriteByte('\n')
	return nil
}

// Close implements io.Closer.
func (r *DBEventReader) Close() error {
	r.done = true
	return nil
}

// matchPattern checks if the value matches the pattern using glob-style matching.
func matchPattern(pattern, value string) bool {
	// handle ... suffix (like ./cmd/...)
	if strings.HasSuffix(pattern, "...") {
		prefix := strings.TrimSuffix(pattern, "...")
		prefix = strings.TrimPrefix(prefix, "./")
		return strings.HasPrefix(value, prefix) || strings.Contains(value, "/"+prefix)
	}

	// convert glob pattern to filepath.Match compatible pattern
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		// if pattern is invalid, try substring match
		return strings.Contains(value, pattern)
	}
	return matched
}

// Ensure DBEventReader implements io.ReadCloser
var _ io.ReadCloser = (*DBEventReader)(nil)
