package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

var updateJoinGoldens = flag.Bool("update", false, "update golden files")

func loadJoinFixture(t *testing.T, name string) shard.Report {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "shard", "testdata", "join", name+".json"))
	require.NoError(t, err)
	var r shard.Report
	require.NoError(t, json.Unmarshal(data, &r))
	return r
}

func assertJoinGolden(t *testing.T, name, ext string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "shard-join", name+"."+ext)
	if *updateJoinGoldens {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

func TestRenderJoin(t *testing.T) {
	// missing shard 2 and coverage below the threshold, built from the passing fixture
	missing := loadJoinFixture(t, "happy")
	missing.Shards[1] = shard.ReportShard{Index: 2}
	missing.Checks.Verified.OK, missing.Checks.Coverage.OK = false, false
	missing.Result, missing.ExitCode = "fail", shard.ExitUnverified
	pctBelow, threshold := 78.3, 80.0
	missing.Checks.Coverage.Percent, missing.Checks.Coverage.Threshold = &pctBelow, &threshold
	missing.Checks.Coverage.ThresholdSource = shard.ThresholdJoin
	missing.Problems = []shard.Problem{
		{Check: shard.CheckVerified, Kind: shard.KindMissingShard, Shards: []int{2}, Message: "missing receipt for shard 2/3 (job failed before canopy finished, or artifact not uploaded)"},
		{Check: shard.CheckCoverage, Kind: shard.KindCoverageBelow, Message: "coverage below threshold: 78.30% < 80.00%"},
	}
	missing.Warnings = []string{"canopy version differs between shards"}
	missing.Checks.Metrics = shard.MetricsCheck{Packages: 6, Warning: "shards failed verification"}
	missing.Suggestion = nil

	// a run with no timing data only has package counts to suggest
	static := loadJoinFixture(t, "happy")
	static.Suggestion = &shard.Suggestion{Current: 3, Static: true, Note: shard.StaticSuggestionNote}
	for i := range static.Shards {
		static.Shards[i].EstimatedMS = nil
	}

	// receipts from different runs stop the join before any shard is filled in
	mixed := shard.Report{
		Total:  3,
		Shards: []shard.ReportShard{{Index: 1}, {Index: 2}, {Index: 3}},
		Problems: []shard.Problem{
			{Check: shard.CheckVerified, Kind: shard.KindTotalMismatch, Message: "receipts disagree on shard total: shard-2 says 2, shard-1 says 3 (stale files in .canopy/shard/out?)"},
		},
		Result:   "fail",
		ExitCode: shard.ExitUnverified,
	}
	mixed.Checks.Tests.OK, mixed.Checks.Coverage.OK = true, true

	// several verified problems at once, with the packages they name
	unverified := loadJoinFixture(t, "weights_mismatch")
	unverified.Problems = append(unverified.Problems,
		shard.Problem{Check: shard.CheckVerified, Kind: shard.KindNeverRan, Packages: []string{"m/e", "m/f"}, Message: "2 packages never ran"},
		shard.Problem{Check: shard.CheckVerified, Kind: shard.KindRanTwice, Shards: []int{1, 3}, Packages: []string{"m/a"}, Message: "1 package ran twice"},
	)

	cases := map[string]shard.Report{
		"unverified":        unverified,
		"total_mismatch":    mixed,
		"happy":             loadJoinFixture(t, "happy"),
		"failed_shard":      loadJoinFixture(t, "failed_shard"),
		"run_flag":          loadJoinFixture(t, "run_flag"),
		"weights_mismatch":  loadJoinFixture(t, "weights_mismatch"),
		"missing_coverage":  missing,
		"static_suggestion": static,
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			var text, md bytes.Buffer
			require.NoError(t, renderJoinText(&text, r, false))
			require.NoError(t, renderJoinMarkdown(&md, r))
			assertJoinGolden(t, name, "txt", text.Bytes())
			assertJoinGolden(t, name, "md", md.Bytes())
		})
	}
}

func TestRenderPlan(t *testing.T) {
	units := []shard.Unit{
		{Package: "m/slow", Weight: 18000}, {Package: "m/a", Weight: 3000}, {Package: "m/b", Weight: 2000, Estimated: true},
		{Package: "m/c", Weight: 1000}, {Package: "m/d", Weight: 500},
	}
	weights := shard.ReportWeights{Source: shard.SourceMetrics, Measured: 4, Estimated: 1}
	plan := func(total int) *shardPlanReport {
		r := &shardPlanReport{Packages: len(units), Weights: weights}
		p := shard.NewPlan(units, total)
		pt := shardPlanTotal{Total: total, Digest: "sha256:80e279d11624db27f8b8101503f3b897"}
		byPkg := map[string]shard.Unit{}
		for _, u := range units {
			byPkg[u.Package] = u
		}
		for i, pkgs := range p.Shards {
			s := shardPlanShard{Index: i + 1, Load: p.Loads[i]}
			for _, pkg := range pkgs {
				s.Packages = append(s.Packages, byPkg[pkg])
			}
			pt.Shards = append(pt.Shards, s)
		}
		r.Plans = []shardPlanTotal{pt}
		r.Suggestion = shard.NewSuggestion(units, total, time.Minute, 4)
		return r
	}

	// without --shards: every count is planned and only the suggestion is shown
	suggestion := plan(1)
	suggestion.Plans = append(suggestion.Plans, plan(2).Plans...)
	suggestion.Suggestion = shard.NewSuggestion(units, 0, time.Minute, 4)

	// where more shards pay off
	var many []shard.Unit
	for i := range 12 {
		many = append(many, shard.Unit{Package: fmt.Sprintf("m/p%02d", i), Weight: 60000})
	}
	split := &shardPlanReport{Packages: len(many), Weights: shard.ReportWeights{Source: shard.SourceMetrics, Measured: 12}, Plans: []shardPlanTotal{{}, {}},
		Suggestion: shard.NewSuggestion(many, 0, 10*time.Second, 1)}

	// test counts as weights
	static := plan(2)
	for i := range static.Plans[0].Shards {
		s := &static.Plans[0].Shards[i]
		s.Load = 0
		for j := range s.Packages {
			s.Packages[j].Weight /= 100
			s.Load += s.Packages[j].Weight
		}
	}
	static.Weights = shard.ReportWeights{Source: shard.SourceStatic, Estimated: 5, Ignored: "no metrics file"}
	static.Suggestion = &shard.Suggestion{Current: 2, Static: true}

	cases := map[string]*shardPlanReport{
		"plan":        plan(3),
		"suggestion":  suggestion,
		"split":       split,
		"static_plan": static,
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			var text bytes.Buffer
			require.NoError(t, renderPlanText(&text, r, false))
			path := filepath.Join("testdata", "shard-plan", name+".txt")
			if *updateJoinGoldens {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, text.Bytes(), 0o600))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, string(want), text.String())
		})
	}
}
