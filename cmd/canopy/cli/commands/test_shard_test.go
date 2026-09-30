package commands

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/anchore/fangs"
	"github.com/anchore/go-logger/adapter/discard"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

const shardFixture = "github.com/wagoodman/canopy/internal/test-fixtures/sharding"

// shardFixtureConfig is a test config over the sharding fixture with no metrics file.
func shardFixtureConfig(t *testing.T, shardValue string) *testConfig {
	t.Helper()
	cfg := defaultTestOptions().Test
	cfg.Specifiers = []string{shardFixture + "/..."}
	cfg.Shard.Dir = t.TempDir()
	cfg.ShardIndex.Value = shardValue
	require.NoError(t, cfg.ShardIndex.PostLoad())
	return &cfg
}

func TestShard_FixtureCoveredExactlyOnce(t *testing.T) {
	all := shardFixtureConfig(t, "")
	_, err := selectTestPackages(all, "v0.0.0")
	require.NoError(t, err)
	want := all.Runtime.Packages.ImportPaths()
	require.Len(t, want, 11)

	for _, total := range []int{1, 3, 4, 12} {
		seen := map[string]int{}
		var digests []string
		for i := 1; i <= total; i++ {
			cfg := shardFixtureConfig(t, fmt.Sprintf("%d/%d", i, total))
			proceed, err := selectTestPackages(cfg, "v0.0.0")
			require.NoError(t, err)
			sh := cfg.Runtime.shard
			require.NotNil(t, sh)
			assert.Equal(t, proceed, len(sh.Planned()) > 0)
			assert.Equal(t, !proceed, cfg.Runtime.NothingToRun)
			assert.Equal(t, sh.Planned(), cfg.Runtime.Packages.ImportPaths())
			for _, p := range cfg.Runtime.Packages.ImportPaths() {
				seen[p]++
			}
			digests = append(digests, sh.Digest)
		}
		for _, p := range want {
			assert.Equal(t, 1, seen[p], "total %d: %s", total, p)
		}
		assert.Len(t, seen, len(want))
		for _, d := range digests {
			assert.Equal(t, digests[0], d, "total %d: shards disagree on the digest", total)
		}
	}
}

func TestShard_TestCountPlanGolden(t *testing.T) {
	cfg := shardFixtureConfig(t, "1/3")
	_, err := selectTestPackages(cfg, "v0.0.0")
	require.NoError(t, err)
	sh := cfg.Runtime.shard

	assert.Equal(t, shard.SourceStatic, sh.Weights.Source)
	var wideShard []string
	for _, s := range sh.Plan.Shards {
		for _, p := range s {
			if p == shardFixture+"/wide" {
				wideShard = s
			}
		}
	}
	assert.Equal(t, []string{shardFixture + "/wide"}, wideShard, "wide is heaviest by test count and should be alone")

	var sb strings.Builder
	for i, s := range sh.Plan.Shards {
		fmt.Fprintf(&sb, "shard %d/3 load %d\n", i+1, sh.Plan.Loads[i])
		for _, p := range s {
			fmt.Fprintf(&sb, "  %s\n", strings.TrimPrefix(p, shardFixture+"/"))
		}
	}
	sb.WriteString(strings.ReplaceAll(strings.Join(sh.Inputs.Weights, "\n"), shardFixture+"/", ""))
	snaps.MatchSnapshot(t, sb.String())
}

func TestShard_DigestIgnoresIndexAndShuffleSeed(t *testing.T) {
	var digests []string
	for _, v := range []string{"1/2", "2/2"} {
		cfg := shardFixtureConfig(t, v)
		cfg.Shuffle = true
		_, err := selectTestPackages(cfg, "v0.0.0")
		require.NoError(t, err)
		// the seed is generated per run; it must not leak into anything hashed
		buildRunConfig(*cfg)
		digests = append(digests, cfg.Runtime.shard.Digest)
		assert.Contains(t, cfg.Runtime.shard.Inputs.Run, "shuffle true")
	}
	assert.Equal(t, digests[0], digests[1])
}

// every go build/test option must show up in the [run] group, or a flag that changes how tests run
// could differ between shards without failing the join.
func TestShardRunLines_CoverEveryGoOption(t *testing.T) {
	opts := defaultTestOptions()
	fangs.AddFlags(discard.New(), pflag.NewFlagSet("test", pflag.ContinueOnError), opts)
	render := func() []string {
		require.NoError(t, opts.Test.GoBuild.PostLoad())
		require.NoError(t, opts.Test.GoTest.PostLoad())
		return shardRunLines(opts.Test)
	}
	base := render()

	for _, s := range []any{&opts.Test.GoBuild, &opts.Test.GoTest} {
		v := reflect.ValueOf(s).Elem()
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			switch f.Name {
			case "RenderedFlags", "IgnoreRenderingFlags", "NamedFlagSet":
				continue
			}
			if !f.IsExported() {
				continue
			}
			field := v.Field(i)
			orig := reflect.ValueOf(field.Interface())
			switch field.Kind() {
			case reflect.Bool:
				field.SetBool(true)
			case reflect.String:
				field.SetString("x")
			case reflect.Int:
				field.SetInt(3)
			default:
				t.Fatalf("%s.%s: unhandled kind %s", v.Type().Name(), f.Name, field.Kind())
			}
			assert.NotEqual(t, base, render(), "%s.%s is not in the [run] input group", v.Type().Name(), f.Name)
			field.Set(orig)
		}
	}
}

func TestShardReceipt(t *testing.T) {
	t0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	lines := []string{
		// passing, fresh
		`{"Action":"start","Package":"ex/pass"}`,
		`{"Action":"run","Package":"ex/pass","Test":"TestA"}`,
		`{"Action":"pass","Package":"ex/pass","Test":"TestA","Elapsed":0.1}`,
		`{"Action":"pass","Package":"ex/pass","Elapsed":0.25}`,
		// passing, from the go test cache
		`{"Action":"start","Package":"ex/cached"}`,
		`{"Action":"run","Package":"ex/cached","Test":"TestA"}`,
		`{"Action":"pass","Package":"ex/cached","Test":"TestA"}`,
		`{"Action":"output","Package":"ex/cached","Output":"ok  \tex/cached\t(cached)\n"}`,
		`{"Action":"pass","Package":"ex/cached","Elapsed":0}`,
		// no test files
		`{"Action":"start","Package":"ex/none"}`,
		`{"Action":"output","Package":"ex/none","Output":"?   \tex/none\t[no test files]\n"}`,
		`{"Action":"skip","Package":"ex/none","Elapsed":0}`,
		// failing test
		`{"Action":"start","Package":"ex/fail"}`,
		`{"Action":"run","Package":"ex/fail","Test":"TestF"}`,
		`{"Action":"fail","Package":"ex/fail","Test":"TestF","Elapsed":0.01}`,
		`{"Action":"fail","Package":"ex/fail","Elapsed":0.3}`,
		// test build failure (go1.24+ reports it with FailedBuild)
		`{"ImportPath":"ex/broken [ex/broken.test]","Action":"build-output","Output":"broken_test.go:3:1: syntax error\n"}`,
		`{"ImportPath":"ex/broken [ex/broken.test]","Action":"build-fail"}`,
		`{"Action":"start","Package":"ex/broken"}`,
		`{"Action":"output","Package":"ex/broken","Output":"FAIL\tex/broken [build failed]\n"}`,
		`{"Action":"fail","Package":"ex/broken","Elapsed":0,"FailedBuild":"ex/broken [ex/broken.test]"}`,
	}
	run := &gotest.Run{ID: uuid.New(), Result: *gotest.NewResult(gotest.ResultConfig{})}
	for i, l := range lines {
		j := gotest.NewJSONL(l, int64(i))
		j.Time = t0.Add(time.Duration(i) * 10 * time.Millisecond).Format(time.RFC3339Nano)
		run.Result.Update(*gotest.NewEvent(run.ID, j, nil))
	}

	units := []shard.Unit{{Package: "ex/pass", Weight: 2}, {Package: "ex/cached", Weight: 2}, {Package: "ex/none", Weight: 1}, {Package: "ex/fail", Weight: 2}, {Package: "ex/broken", Weight: 1}, {Package: "ex/other", Weight: 50}}
	covermin := 80.0
	sh := &shardRuntime{
		Index:   2,
		Total:   2,
		Plan:    shard.NewPlan(units, 2),
		Weights: shard.WeightResult{Units: units, Source: shard.SourceStatic, Estimated: len(units)},
		Inputs: shard.Inputs{
			Selection: []string{"specifier ./...", "package ex/pass"},
			Run:       []string{"cover false"},
		},
		Digest: "sha256:abc",
		Gates:  shard.Gates{CoverMin: &covermin},
	}
	require.Equal(t, []string{"ex/other"}, sh.Plan.Shards[0])

	r := shardReceipt(sh, "v1.2.3", run, run.Result.Passed())

	assert.Equal(t, "v1.2.3", r.CanopyVersion)
	assert.Equal(t, []any{2, 2, "sha256:abc"}, []any{r.Index, r.Total, r.Digest})
	assert.Equal(t, []string{"ex/broken", "ex/cached", "ex/fail", "ex/none", "ex/pass"}, r.Planned)
	assert.Equal(t, int64(8), r.LoadMS)
	assert.Len(t, r.Units, 6)
	// every package with a final pass/fail/skip, including the test build failure
	assert.Equal(t, r.Planned, r.Reported)
	assert.False(t, r.Passed)
	// only the fresh passing package is an observation (not cached, not failed, not no-tests)
	assert.Equal(t, map[string]int64{"ex/pass": 250}, r.Observations)
	assert.Equal(t, []shard.Failure{{Package: "ex/fail", Test: "TestF"}}, r.Failures)
	assert.Equal(t, []string{"ex/broken"}, r.FailedPkgs)
	assert.Equal(t, shard.TestTally{Passed: 2, Failed: 1}, r.Tests)
	assert.Equal(t, &covermin, r.Gates.CoverMin)
	assert.Equal(t, []string{"specifier ./..."}, r.Inputs["selection"].Lines, "package lines are left out")
	assert.Equal(t, shard.SourceStatic, r.Inputs["weights"].Source)
	assert.Empty(t, r.Inputs["weights"].Lines)

	// nothing ran: an empty receipt that still passes
	empty := shardReceipt(&shardRuntime{Index: 1, Total: 2, Plan: sh.Plan}, "v1.2.3", nil, true)
	assert.True(t, empty.Passed)
	assert.Equal(t, []string{"ex/other"}, empty.Planned)
	assert.Empty(t, empty.Reported)
	assert.NotNil(t, empty.Reported)
	assert.NotNil(t, empty.Failures)
	assert.NotNil(t, empty.Observations)
}

func TestShardSelectionLines_RejectsOptionLikeRef(t *testing.T) {
	cfg := testConfig{Affected: true, AffectedSince: "--output=/tmp/x"}
	if _, err := shardSelectionLines(cfg); err == nil {
		t.Fatal("expected an error for a ref starting with '-'")
	}
}
