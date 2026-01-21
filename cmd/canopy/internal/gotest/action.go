package gotest

import "strings"

const (
	UnknownAction     Action = "unknown-action"
	StartAction       Action = "start" // first JSONL event for a package
	RunAction         Action = "run"   // first JSONL event for a test/subtest
	PassAction        Action = "pass"  // last JSONL event for a test/subtest or package
	FailAction        Action = "fail"  // last JSONL event for a test/subtest or package
	SkipAction        Action = "skip"  // last JSONL event for a test/subtest or package
	OutputAction      Action = "output"
	BuildOutputAction Action = "build-output" // compiler output during build failure (Go 1.24+)
	BuildFailAction   Action = "build-fail"   // build failure event (Go 1.24+)
)

// Action represents the state transitions that tests go through during execution.
// Actions are parsed from `go test -json` output and used to track test lifecycle.
type Action string

// ParseAction converts a string from `go test -json` output into a typed Action.
// Returns UnknownAction for any unrecognized strings.
func ParseAction(s string) Action {
	switch strings.ToLower(s) {
	case "run":
		return RunAction
	case "pass":
		return PassAction
	case "fail":
		return FailAction
	case "skip":
		return SkipAction
	case "start":
		return StartAction
	case "output":
		return OutputAction
	case "build-output":
		return BuildOutputAction
	case "build-fail":
		return BuildFailAction
	}
	return UnknownAction
}

// Completed returns true if the action represents a terminal state for a test.
// Terminal states are pass, fail, or skip - once reached, no further events
// are expected for that test reference.
func (a Action) Completed() bool {
	switch a {
	case PassAction, FailAction, SkipAction:
		return true
	default:
		return false
	}
}
