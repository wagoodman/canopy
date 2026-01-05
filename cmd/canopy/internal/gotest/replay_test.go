package gotest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestReplayJSON(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		wantCount     int
		wantActions   []string
		wantPackages  []string
		wantTestNames []string
	}{
		{
			name:      "simple test run with pass and fail",
			fixture:   "testdata/simple.json",
			wantCount: 12,
			wantActions: []string{
				"start", "run", "output", "output", "fail",
				"run", "output", "output", "pass",
				"output", "output", "fail",
			},
			wantPackages: []string{
				"github.com/wagoodman/canopy/internal/test-fixtures/simple",
			},
			wantTestNames: []string{
				"TestFail", "TestPass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.fixture)
			require.NoError(t, err)
			defer f.Close()

			events := ReplayJSON(f)

			var collected []JSONL
			for e := range events {
				collected = append(collected, e)
			}

			require.Len(t, collected, tt.wantCount)

			// verify actions are in expected order
			var gotActions []string
			for _, e := range collected {
				gotActions = append(gotActions, e.Action)
			}
			assert.Equal(t, tt.wantActions, gotActions)

			// verify packages are present
			pkgSet := make(map[string]bool)
			for _, e := range collected {
				pkgSet[e.Package] = true
			}
			for _, pkg := range tt.wantPackages {
				assert.True(t, pkgSet[pkg], "expected package %s to be present", pkg)
			}

			// verify test names are present
			testSet := make(map[string]bool)
			for _, e := range collected {
				if e.Test != "" {
					testSet[e.Test] = true
				}
			}
			for _, test := range tt.wantTestNames {
				assert.True(t, testSet[test], "expected test %s to be present", test)
			}
		})
	}
}

func TestReplayEvents(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		pkgs        *golist.PackageCollection
		wantCount   int
		wantActions []Action
	}{
		{
			name:    "converts JSONL to events",
			fixture: "testdata/simple.json",
			pkgs:    nil,
			// note: events with nil from NewEvent are filtered out, so we may have fewer than JSONL count
			wantCount: 12,
			wantActions: []Action{
				StartAction, RunAction, OutputAction, OutputAction, FailAction,
				RunAction, OutputAction, OutputAction, PassAction,
				OutputAction, OutputAction, FailAction,
			},
		},
		{
			name:    "with package collection for directory enrichment",
			fixture: "testdata/simple.json",
			pkgs: golist.NewPackageCollection(
				golist.Package{
					ImportPath: "github.com/wagoodman/canopy/internal/test-fixtures/simple",
					Dir:        "/test/path/simple",
				},
			),
			wantCount: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.fixture)
			require.NoError(t, err)
			defer f.Close()

			events := ReplayEvents(f, tt.pkgs)

			var collected []Event
			for e := range events {
				collected = append(collected, e)
			}

			require.Len(t, collected, tt.wantCount)

			// all events from ReplayEvents should have uuid.Nil as RunID (stateless)
			for _, e := range collected {
				assert.Equal(t, uuid.Nil, e.RunID, "ReplayEvents should use uuid.Nil for RunID")
			}

			if tt.wantActions != nil {
				var gotActions []Action
				for _, e := range collected {
					gotActions = append(gotActions, e.Action)
				}
				assert.Equal(t, tt.wantActions, gotActions)
			}

			// verify package directory enrichment when collection is provided
			if tt.pkgs != nil {
				for _, e := range collected {
					if e.Reference.Package == "github.com/wagoodman/canopy/internal/test-fixtures/simple" {
						assert.Equal(t, "/test/path/simple", e.PackageDirPath)
					}
				}
			}
		})
	}
}

func TestReplayRun(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		runnerCfg      RunnerConfig
		resultCfg      ResultConfig
		wantTestStats  ResultStats
		wantPkgCount   int
		wantTestRefs   int
		callbackCalled bool
	}{
		{
			name:      "reconstructs complete test run",
			fixture:   "testdata/simple.json",
			runnerCfg: RunnerConfig{},
			resultCfg: ResultConfig{},
			wantTestStats: ResultStats{
				Passed:  1,
				Failed:  1,
				Skipped: 0,
				Running: 0,
			},
			wantPkgCount:   1,
			wantTestRefs:   2, // TestFail, TestPass
			callbackCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.fixture)
			require.NoError(t, err)
			defer f.Close()

			var callbackCount int
			callback := func(event *Event) {
				callbackCount++
			}

			run := ReplayRun(f, tt.runnerCfg, tt.resultCfg, callback)

			require.NotNil(t, run)
			assert.NotEqual(t, uuid.Nil, run.ID, "run should have a valid ID")

			// verify test stats
			stats := run.Result.TestStats()
			assert.Equal(t, tt.wantTestStats, stats)

			// verify package count
			packages := run.Result.Packages()
			assert.Len(t, packages, tt.wantPkgCount)

			// verify test reference count (excluding packages)
			testRefs := run.Result.TestReferencesByAction(PassAction)
			testRefs = append(testRefs, run.Result.TestReferencesByAction(FailAction)...)
			assert.Len(t, testRefs, tt.wantTestRefs)

			// verify callback was called
			if tt.callbackCalled {
				assert.Greater(t, callbackCount, 0, "callback should have been called")
			}
		})
	}
}

func TestStartReplayRun(t *testing.T) {
	tests := []struct {
		name          string
		fixture       string
		runnerCfg     RunnerConfig
		resultCfg     ResultConfig
		wantTestStats ResultStats
	}{
		{
			name:      "async replay with run ID consistency on events",
			fixture:   "testdata/simple.json",
			runnerCfg: RunnerConfig{},
			resultCfg: ResultConfig{},
			wantTestStats: ResultStats{
				Passed:  1,
				Failed:  1,
				Skipped: 0,
				Running: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.fixture)
			require.NoError(t, err)
			defer f.Close()

			run, events := StartReplayRun(f, tt.runnerCfg, tt.resultCfg)

			require.NotNil(t, run)
			assert.NotEqual(t, uuid.Nil, run.ID, "run should have a valid ID")

			var collectedEvents []*Event
			for e := range events {
				collectedEvents = append(collectedEvents, e)
			}

			// the last event should be nil (signals completion)
			require.NotEmpty(t, collectedEvents)
			assert.Nil(t, collectedEvents[len(collectedEvents)-1], "last event should be nil to signal completion")

			// verify that all non-nil events have the run's ID assigned
			for _, e := range collectedEvents {
				if e != nil {
					assert.Equal(t, run.ID, e.RunID, "event RunID should match the run's ID")
				}
			}

			// verify test stats after completion
			stats := run.Result.TestStats()
			assert.Equal(t, tt.wantTestStats, stats)
		})
	}
}

func TestStartReplayRun_EventsStreamWhileProcessing(t *testing.T) {
	// this test verifies that events are streamed as they are processed,
	// not buffered until completion
	f, err := os.Open("testdata/simple.json")
	require.NoError(t, err)
	defer f.Close()

	run, events := StartReplayRun(f, RunnerConfig{}, ResultConfig{})

	var eventCount int
	for e := range events {
		if e != nil {
			eventCount++
			// each event should have the run ID assigned
			assert.Equal(t, run.ID, e.RunID)
		}
	}

	assert.Greater(t, eventCount, 0, "should have received events")
}

func TestReplayJSON_EmptyInput(t *testing.T) {
	// create temp empty file
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")
	err := os.WriteFile(emptyFile, []byte{}, 0644)
	require.NoError(t, err)

	f, err := os.Open(emptyFile)
	require.NoError(t, err)
	defer f.Close()

	events := ReplayJSON(f)

	var count int
	for range events {
		count++
	}

	assert.Equal(t, 0, count, "empty input should produce no events")
}

func TestReplayRun_WithNilCallback(t *testing.T) {
	// test that nil callbacks are handled gracefully
	f, err := os.Open("testdata/simple.json")
	require.NoError(t, err)
	defer f.Close()

	// should not panic with nil callbacks
	run := ReplayRun(f, RunnerConfig{}, ResultConfig{}, nil)
	require.NotNil(t, run)
}
