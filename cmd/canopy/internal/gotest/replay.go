package gotest

import (
	"io"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

// ReplayJSON reads `go test -json` output from a reader and returns a channel of parsed JSONL events.
// This is useful for replaying previously captured test runs for analysis or display.
func ReplayJSON(reader io.Reader) <-chan JSONL {
	events := make(chan JSONL)

	go func() {
		defer close(events)
		jsonLFromReader(reader, events)
	}()

	return events
}

// ReplayEvents reads `go test -json` output and converts it to a stream of Events.
// Takes a package collection to enrich events with package directory information.
func ReplayEvents(reader io.Reader, pkgs *golist.PackageCollection) <-chan Event {
	events := make(chan Event)

	go func() {
		defer close(events)
		for j := range ReplayJSON(reader) {
			e := NewEvent(uuid.Nil, j, pkgs)
			if e == nil {
				continue
			}
			events <- *e
		}
	}()

	return events
}

// ReplayRun reconstructs a complete test run from previously captured `go test -json` output.
// Processes all events and invokes the optional onEvent callbacks for each event.
// Returns the complete Run with all results aggregated.
func ReplayRun(reader io.Reader, runnerCfg RunnerConfig, resultCfg ResultConfig, onEvent ...func(event *Event)) *Run {
	run, evs := StartReplayRun(reader, runnerCfg, resultCfg)
	for e := range evs {
		for _, fn := range onEvent {
			if fn != nil {
				fn(e)
			}
		}
	}
	return run
}

// StartReplayRun begins replaying a test run asynchronously, returning the Run and an event channel.
// This allows processing events as they are replayed rather than waiting for completion.
// The event channel will receive a nil event when replay is complete.
func StartReplayRun(reader io.Reader, runnerCfg RunnerConfig, resultCfg ResultConfig) (*Run, <-chan *Event) {
	run := NewRun(runnerCfg)
	run.Result = *NewResult(resultCfg)
	evs := make(chan *Event)

	go func() {
		defer close(evs)

		for e := range ReplayEvents(reader, runnerCfg.Packages) {
			run.Result.Update(e)
			evs <- &e
		}
		evs <- nil
	}()

	return run, evs
}
