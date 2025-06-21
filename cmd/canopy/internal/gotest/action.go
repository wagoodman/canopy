package gotest

import "strings"

const (
	UnknownAction Action = "unknown-action"
	StartAction   Action = "start" // first JSONL event for a package
	RunAction     Action = "run"   // first JSONL event for a test/subtest
	PassAction    Action = "pass"  // last JSONL event for a test/subtest or package
	FailAction    Action = "fail"  // last JSONL event for a test/subtest or package
	SkipAction    Action = "skip"  // last JSONL event for a test/subtest or package
	OutputAction  Action = "output"
)

type Action string

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
	}
	return UnknownAction
}

func (a Action) Completed() bool {
	switch a {
	case PassAction, FailAction, SkipAction:
		return true
	default:
		return false
	}
}
