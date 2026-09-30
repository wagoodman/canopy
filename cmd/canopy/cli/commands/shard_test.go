package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/anchore/clio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

func newTestApp() clio.Application {
	return clio.New(*clio.NewSetupConfig(clio.Identification{Name: "canopy", Version: "v0.0.0"}).
		WithUI(ui.TestNoUI()).
		WithInitializers(func(s *clio.State) error {
			log.Set(s.Logger)
			return nil
		}))
}

// runCanopy runs the CLI the way main does, returning the error main would turn into an exit code.
func runCanopy(t *testing.T, args ...string) error {
	t.Helper()
	// in CI, resolving the appearance would otherwise force a truecolor lipgloss profile for the
	// whole package and leak ANSI codes into other tests
	t.Setenv("NO_COLOR", "1")
	app := newTestApp()
	root := Root(app)
	root.AddCommand(Test(app), Shard(app))
	root.SetArgs(args)
	return root.Execute()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec ExitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return 1
}

// writeJoinReceipts writes a passing 3 shard run, one package per shard at 50% coverage.
func writeJoinReceipts(t *testing.T, dir string, coverMin *float64) {
	t.Helper()
	require.NoError(t, os.MkdirAll(shard.OutDir(dir), 0o755))
	units := []string{"m/p1", "m/p2", "m/p3"}
	for i, pkg := range units {
		cover := fmt.Sprintf("shard-%d.coverprofile", i+1)
		profile := fmt.Sprintf("mode: set\n%[1]s/f.go:1.1,2.1 1 1\n%[1]s/f.go:3.1,4.1 1 0\n", pkg)
		require.NoError(t, os.WriteFile(filepath.Join(shard.OutDir(dir), cover), []byte(profile), 0o600))
		require.NoError(t, shard.WriteReceipt(shard.ReceiptPath(dir, i+1), shard.Receipt{
			Version:       shard.ReceiptVersion,
			CanopyVersion: "v0.0.0",
			Index:         i + 1,
			Total:         len(units),
			Digest:        "sha256:same",
			Units:         units,
			Planned:       []string{pkg},
			Reported:      []string{pkg},
			Passed:        true,
			Tests:         shard.TestTally{Passed: 1},
			Gates:         shard.Gates{CoverMin: coverMin},
			Observations:  map[string]int64{pkg: 100},
			Coverprofile:  cover,
		}))
	}
}

func TestShardJoin_ExitCodes(t *testing.T) {
	eighty := 80.0
	tests := []struct {
		name       string
		recorded   *float64 // covermin recorded by the shards
		config     string   // .canopy.yaml in the working dir
		args       []string
		mutate     func(t *testing.T, dir string)
		wantExit   int
		wantSource string
	}{
		{name: "happy path", wantExit: 0},
		{
			name:     "missing shard",
			mutate:   func(t *testing.T, dir string) { require.NoError(t, os.Remove(shard.ReceiptPath(dir, 2))) },
			wantExit: shard.ExitUnverified,
		},
		{name: "covermin from .canopy.yaml only", config: "test:\n  covermin: 100\n", wantExit: shard.ExitGateFailed, wantSource: shard.ThresholdJoin},
		{name: "covermin only in receipts", recorded: &eighty, wantExit: shard.ExitGateFailed, wantSource: shard.ThresholdReceipts},
		{name: "explicit 0 on the join overrides receipts", recorded: &eighty, args: []string{"--covermin", "0"}, wantExit: 0, wantSource: shard.ThresholdJoin},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			t.Chdir(wd)
			t.Setenv("GITHUB_STEP_SUMMARY", "")
			if tt.config != "" {
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".canopy.yaml"), []byte(tt.config), 0o600))
			}
			dir := filepath.Join(wd, "shard")
			writeJoinReceipts(t, dir, tt.recorded)
			if tt.mutate != nil {
				tt.mutate(t, dir)
			}

			reportPath := filepath.Join(wd, "report.json")
			err := runCanopy(t, append([]string{"shard", "join", "--shard-dir", dir, "-o", "json=" + reportPath}, tt.args...)...)
			assert.Equal(t, tt.wantExit, exitCode(err), "err: %v", err)

			var r shard.Report
			b, rerr := os.ReadFile(reportPath)
			require.NoError(t, rerr, "the report is written even when the join fails")
			require.NoError(t, json.Unmarshal(b, &r))
			assert.Equal(t, tt.wantExit, r.ExitCode)
			assert.Equal(t, tt.wantSource, r.Checks.Coverage.ThresholdSource)
		})
	}
}

func TestShardJoin_Outputs(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	dir := filepath.Join(wd, "shard")
	writeJoinReceipts(t, dir, nil)
	summary := filepath.Join(wd, "summary.md")

	// the default includes github-summary under GitHub Actions, appended to
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	require.NoError(t, runCanopy(t, "shard", "join", "--shard-dir", dir))
	require.NoError(t, runCanopy(t, "shard", "join", "--shard-dir", dir))
	b, err := os.ReadFile(summary)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(b), "### canopy shard join"))

	// an explicit -o replaces the default entirely
	require.NoError(t, os.Remove(summary))
	require.NoError(t, runCanopy(t, "shard", "join", "--shard-dir", dir, "-o", "json="+filepath.Join(wd, "r.json")))
	assert.NoFileExists(t, summary)
	assert.FileExists(t, filepath.Join(wd, "r.json"))

	// with =path it goes to that file
	other := filepath.Join(wd, "other.md")
	require.NoError(t, runCanopy(t, "shard", "join", "--shard-dir", dir, "-o", "github-summary="+other))
	assert.FileExists(t, other)

	// bare github-summary needs $GITHUB_STEP_SUMMARY
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	err = runCanopy(t, "shard", "join", "--shard-dir", dir, "-o", "github-summary")
	require.ErrorContains(t, err, "GITHUB_STEP_SUMMARY")

	// test.output holds canopy test's formats and must not affect the join; test.shard.output does
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".canopy.yaml"), []byte("test:\n  output: [go]\n  shard:\n    output: [json="+filepath.Join(wd, "cfg.json")+"]\n"), 0o600))
	require.NoError(t, runCanopy(t, "shard", "join", "--shard-dir", dir))
	assert.FileExists(t, filepath.Join(wd, "cfg.json"))
}

// every gate a shard records must be settable on both `canopy test` and the join, or the join could
// never override it (and a new gate added to one command only would go unnoticed).
func TestShardGates_SameOnTestAndJoin(t *testing.T) {
	app := newTestApp()
	test, join := Test(app), ShardJoin(app)
	gates := reflect.TypeOf(shard.Gates{})
	for i := range gates.NumField() {
		name, _, _ := strings.Cut(gates.Field(i).Tag.Get("json"), ",")
		assert.NotNil(t, test.Flags().Lookup(name), "canopy test has no --%s", name)
		assert.NotNil(t, join.Flags().Lookup(name), "canopy shard join has no --%s", name)
	}
}

func TestShardPlan_JSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "plan.json")
	shardDir := t.TempDir()
	require.NoError(t, runCanopy(t, "shard", "plan", shardFixture+"/...", "--shards", "3", "--shard-dir", shardDir, "-o", "json="+out))

	var r shardPlanReport
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &r))

	assert.Equal(t, 11, r.Packages)
	assert.Equal(t, shard.SourceStatic, r.Weights.Source)
	require.Len(t, r.Plans, 1)
	p := r.Plans[0]
	assert.Equal(t, 3, p.Total)
	require.Len(t, p.Shards, 3)
	for _, s := range p.Shards {
		for _, u := range s.Packages {
			if u.Package == shardFixture+"/wide" {
				assert.Len(t, s.Packages, 1, "wide is heaviest by test count and should be alone")
			}
		}
	}
	require.NotNil(t, r.Suggestion)
	assert.True(t, r.Suggestion.Static)

	// the digest a shard of the same config records
	cfg := shardFixtureConfig(t, "1/3")
	cfg.Shard.Dir = shardDir
	_, err = selectTestPackages(cfg, "v0.0.0")
	require.NoError(t, err)
	assert.Equal(t, cfg.Runtime.shard.Digest, p.Digest)
}
