package gotest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResult_PackagePhases(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkg := NewReference("pkg/a", "")
	at := func(ref Reference, action Action, offset time.Duration) Event {
		return Event{Reference: ref, Action: action, Time: base.Add(offset)}
	}
	testA := NewReference("pkg/a", "TestA")
	testB := NewReference("pkg/a", "TestB")

	cases := []struct {
		name   string
		events []Event
		want   PackagePhases
		ok     bool
	}{
		{
			name: "split at the first test, ending at the last test event",
			events: []Event{
				at(pkg, StartAction, 0),
				// package-level output before the first test (e.g. TestMain setup) still counts as startup
				{Reference: pkg, Action: OutputAction, Output: "setting up\n", Time: base.Add(5 * time.Second)},
				at(testA, RunAction, 7800*time.Millisecond),
				at(testB, RunAction, 7810*time.Millisecond),
				at(testB, PassAction, 7815*time.Millisecond),
				at(testA, PassAction, 7820*time.Millisecond),
				// the tail after the last test (teardown, exit) is in neither phase
				at(pkg, PassAction, 7832*time.Millisecond),
			},
			want: PackagePhases{Startup: 7800 * time.Millisecond, Tests: 20 * time.Millisecond},
			ok:   true,
		},
		{
			name: "still in flight",
			events: []Event{
				at(pkg, StartAction, 0),
				at(testA, RunAction, time.Second),
			},
		},
		{
			name: "cached result, nothing was launched",
			events: []Event{
				at(pkg, StartAction, 0),
				at(testA, RunAction, 0),
				at(testA, PassAction, 0),
				{Reference: pkg, Action: OutputAction, Output: "ok  \tpkg/a\t(cached)\n", Annotations: []Annotation{Cached}, Time: base},
				at(pkg, PassAction, 0),
			},
		},
		{
			name: "no tests ran",
			events: []Event{
				at(pkg, StartAction, 0),
				at(pkg, PassAction, time.Second),
			},
		},
		{
			name: "no start event (older go versions)",
			events: []Event{
				at(testA, RunAction, 0),
				at(testA, PassAction, time.Second),
				at(pkg, PassAction, time.Second),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult(ResultConfig{})
			for _, e := range tt.events {
				result.Update(e)
			}

			got, ok := result.PackagePhases(pkg)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.want, got)
		})
	}
}
