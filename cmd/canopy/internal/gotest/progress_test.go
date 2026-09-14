package gotest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestResult_StartedPackages(t *testing.T) {
	result := NewResult(ResultConfig{})
	require.Empty(t, result.StartedPackages())

	result.Update(Event{Action: StartAction, Reference: Reference{Package: "pkg/a"}})
	result.Update(Event{Action: StartAction, Reference: Reference{Package: "pkg/b"}})

	// events for a package that already started must not count it again
	result.Update(Event{Action: RunAction, Reference: Reference{Package: "pkg/a", FuncName: "TestA"}})
	result.Update(Event{Action: StartAction, Reference: Reference{Package: "pkg/a"}})

	require.ElementsMatch(t, []string{"pkg/a", "pkg/b"}, result.StartedPackages())
}

func TestRun_BuildProgress(t *testing.T) {
	run := NewRun(RunnerConfig{
		Packages: golist.NewPackageCollection(
			golist.Package{ImportPath: "pkg/a"},
			golist.Package{ImportPath: "pkg/b"},
			golist.Package{ImportPath: "pkg/c"},
		),
	})
	run.Result = *NewResult(ResultConfig{})

	require.Equal(t, BuildProgress{Expected: 3}, run.BuildProgress())

	run.Result.Update(Event{Action: StartAction, Reference: Reference{Package: "pkg/a"}})

	require.Equal(t, BuildProgress{Expected: 3, Started: 1}, run.BuildProgress())
}

func TestRun_BuildProgress_UnknownPackages(t *testing.T) {
	// replaying recorded events doesn't tell us what was asked for, only what happened
	run := NewRun(RunnerConfig{})
	run.Result = *NewResult(ResultConfig{})

	require.Nil(t, run.ExpectedPackages())
	require.Equal(t, BuildProgress{}, run.BuildProgress())
}

func TestBuildProgress_States(t *testing.T) {
	cases := []struct {
		name     string
		progress BuildProgress
		building bool
		known    bool
	}{
		{name: "nothing started", progress: BuildProgress{Expected: 3}, building: true, known: true},
		{name: "partially started", progress: BuildProgress{Expected: 3, Started: 2}, building: true, known: true},
		{name: "all started", progress: BuildProgress{Expected: 3, Started: 3}, building: false, known: true},
		{name: "unknown package set, nothing started", progress: BuildProgress{}, building: true, known: false},
		// without a package count, anything having started is the only signal that the build got somewhere
		{name: "unknown package set, some started", progress: BuildProgress{Started: 2}, building: false, known: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.building, tt.progress.Building())
			require.Equal(t, tt.known, tt.progress.Known())
		})
	}
}
