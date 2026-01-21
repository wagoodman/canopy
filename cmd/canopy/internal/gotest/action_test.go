package gotest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAction(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Action
	}{
		{
			name:  "run",
			input: "run",
			want:  RunAction,
		},
		{
			name:  "pass",
			input: "pass",
			want:  PassAction,
		},
		{
			name:  "fail",
			input: "fail",
			want:  FailAction,
		},
		{
			name:  "skip",
			input: "skip",
			want:  SkipAction,
		},
		{
			name:  "start",
			input: "start",
			want:  StartAction,
		},
		{
			name:  "output",
			input: "output",
			want:  OutputAction,
		},
		{
			name:  "build-output",
			input: "build-output",
			want:  BuildOutputAction,
		},
		{
			name:  "build-fail",
			input: "build-fail",
			want:  BuildFailAction,
		},
		{
			name:  "unknown action",
			input: "unknown",
			want:  UnknownAction,
		},
		{
			name:  "case insensitive - uppercase",
			input: "PASS",
			want:  PassAction,
		},
		{
			name:  "case insensitive - build-output",
			input: "BUILD-OUTPUT",
			want:  BuildOutputAction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAction(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAction_Completed(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{
			name:   "pass is completed",
			action: PassAction,
			want:   true,
		},
		{
			name:   "fail is completed",
			action: FailAction,
			want:   true,
		},
		{
			name:   "skip is completed",
			action: SkipAction,
			want:   true,
		},
		{
			name:   "run is not completed",
			action: RunAction,
			want:   false,
		},
		{
			name:   "start is not completed",
			action: StartAction,
			want:   false,
		},
		{
			name:   "output is not completed",
			action: OutputAction,
			want:   false,
		},
		{
			name:   "build-output is not completed",
			action: BuildOutputAction,
			want:   false,
		},
		{
			name:   "build-fail is not completed",
			action: BuildFailAction,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.action.Completed()
			require.Equal(t, tt.want, got)
		})
	}
}
