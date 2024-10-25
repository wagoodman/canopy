package gotest

import (
	"io"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func ReplayJSON(reader io.Reader) <-chan JSONL {
	events := make(chan JSONL)

	go func() {
		defer close(events)
		jsonLFromReader(reader, events)
	}()

	return events
}

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

// func ReplayRun(reader io.Reader, runnerCfg RunnerConfig, resultCfg ResultConfig, onEvent ...func(event *Event)) *Run {
//	run := NewRun(runnerCfg)
//	run.Start = time.Now()
//	run.Result = *NewResult(resultCfg)
//
//	var lastEvent *Event
//
//	for e := range ReplayEvents(reader, runnerCfg.Packages) {
//		if run.Start.IsZero() {
//			run.Start = e.Time
//		}
//		run.Result.Update(e)
//
//		for _, fn := range onEvent {
//			fn(&e)
//		}
//
//		lastEvent = &e
//	}
//	if lastEvent != nil {
//		run.End = &lastEvent.Time
//	}
//
//	return run
//}

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
