package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

const (
	shardPassJSON = `{"Time":"2026-01-01T00:00:00Z","Action":"start","Package":"acme/store"}
{"Time":"2026-01-01T00:00:01Z","Action":"run","Package":"acme/store","Test":"TestPut"}
{"Time":"2026-01-01T00:00:02Z","Action":"pass","Package":"acme/store","Test":"TestPut","Elapsed":1}
{"Time":"2026-01-01T00:00:03Z","Action":"pass","Package":"acme/store","Elapsed":3}
`
	shardFailJSON = `{"Time":"2026-01-01T00:00:00Z","Action":"start","Package":"acme/sync"}
{"Time":"2026-01-01T00:00:01Z","Action":"run","Package":"acme/sync","Test":"TestReconcile"}
{"Time":"2026-01-01T00:00:02Z","Action":"fail","Package":"acme/sync","Test":"TestReconcile","Elapsed":1}
{"Time":"2026-01-01T00:00:03Z","Action":"fail","Package":"acme/sync","Elapsed":3}
`
)

// testShardRuntime plans 6 packages over 3 shards with metrics weights for all but one.
func testShardRuntime(index, total int, pkgs []string, coverMin *float64) *shardRuntime {
	counts := map[string]int64{}
	var units []shard.Unit
	for i, p := range pkgs {
		counts[p] = int64(i + 1)
		u := shard.Unit{Package: p, Weight: int64(72_000 / (i + 1))}
		if i == len(pkgs)-1 {
			u.Estimated, u.Weight = true, 1400
		}
		units = append(units, u)
	}
	plan := shard.NewPlan(units, total)
	sh := &shardRuntime{
		Index: index, Total: total, From: "flag",
		Weights: shard.WeightResult{Units: units, Source: shard.SourceMetrics, Measured: len(pkgs) - 1, Estimated: 1},
		Counts:  counts, Plan: plan, Digest: "sha256:9f3c1a2b77d0e4c1aabbccdd",
		Metrics:     shard.ReceiptMetrics{File: "sha256:77d0e4c1aabbccdd", Env: shard.Env{GOOS: "linux", GOARCH: "arm64"}},
		MetricsPath: ".canopy/shard/metrics/linux-arm64-cced3044f0ac.json",
		Inputs: shard.Inputs{
			Go:     []string{"goversion go1.27.1", "goos linux", "goarch arm64"},
			Run:    []string{"test-flag -race", "test-flag -timeout=10m"},
			Source: []string{"commit a1b2c3d4e5f6", "dirty false"},
		},
	}
	sh.Gates.CoverMin = coverMin
	return sh
}

func TestShardUI(t *testing.T) {
	pkgs := []string{"acme/store", "acme/api", "acme/sync", "acme/queue", "acme/version", "acme/newthing"}
	cover := 80.0

	static := testShardRuntime(1, 3, pkgs, nil)
	static.Weights = shard.WeightResult{Units: static.Weights.Units, Source: shard.SourceStatic, Ignored: "no metrics file"}
	static.Auto, static.From = true, "CI_NODE_INDEX/CI_NODE_TOTAL"

	fallback := testShardRuntime(1, 3, pkgs, nil)
	fallback.Weights = shard.WeightResult{Units: fallback.Weights.Units, Source: shard.SourceStatic, Ignored: `metrics ignored, recorded with "cover true" but this run has "cover false"`}

	cases := []struct {
		name string
		sh   *shardRuntime
		run  string // go test json of the run, empty for an empty shard
		cov  float64
	}{
		{name: "passing", sh: testShardRuntime(2, 3, pkgs, &cover), run: shardPassJSON, cov: 81.7},
		{name: "failing", sh: testShardRuntime(3, 3, pkgs, &cover), run: shardFailJSON, cov: 79.4},
		{name: "static_weights", sh: static, run: shardPassJSON, cov: 81.7},
		{name: "fallback_weights", sh: fallback, run: shardPassJSON, cov: 81.7},
		{name: "empty", sh: testShardRuntime(6, 8, pkgs[:2], nil)},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var sb strings.Builder
			printShardHeader(&sb, tt.sh, false)
			if tt.run == "" {
				printEmptyShard(&sb, tt.sh, false)
			} else {
				run := gotest.ReplayRun(strings.NewReader(tt.run), gotest.RunnerConfig{}, gotest.ResultConfig{}, nil)
				run.Result.SetCoverage(&tt.cov)
				cfg := presenter.GoSummaryConfig{PackageNameWidth: 40, DurationFromEvents: true}.WithShardTrailer(shardTrailer(tt.sh))
				require.NoError(t, cfg.New(*run).Present(&sb, &sb))
			}
			assertShardUIGolden(t, tt.name, sb.String())
		})
	}
}

func assertShardUIGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "shard-ui", name+".txt")
	if *updateJoinGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}
