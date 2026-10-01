package shard

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

var (
	env     = Env{GOOS: "linux", GOARCH: "arm64"}
	allPkgs = []string{"m/a", "m/b", "m/c", "m/d", "m/e", "m/f"}
	split   = [][]string{{"m/a", "m/b"}, {"m/c", "m/d"}, {"m/e", "m/f"}}
)

// joinFixture is a 3 shard run that passes: each shard's inputs, its receipt and its coverprofile
// body. Tests mutate it before it's written; seal recomputes the digests from Inputs.
type joinFixture struct {
	inputs   []Inputs
	receipts []Receipt
	profiles []string
	garbage  []int // shard indexes whose receipt file is written unparseable
}

func newJoinFixture(metricsFile string) *joinFixture {
	f := &joinFixture{}
	for i, planned := range split {
		var units []Unit
		for _, p := range allPkgs {
			units = append(units, Unit{Package: p, Weight: 100})
		}
		in := Inputs{
			Plan:      PlanLines("v0.9.0", 3),
			Selection: append([]string{"specifier ./...", "affected false"}, prefixed("package ", allPkgs)...),
			Run:       []string{"test-flag -timeout=10m", "cover true", "no-cache false", "shuffle false"},
			Go:        []string{"goversion go1.27.1", "goos linux", "goarch arm64", "env CGO_ENABLED=1"},
			Source:    []string{"commit a1b2c3", "dirty false"},
			Weights:   []string{"source " + SourceMetrics}, // seal adds the units
		}
		obs := map[string]int64{}
		var profile string
		for _, p := range planned {
			obs[p] = 110
			profile += fmt.Sprintf("%s.go:1.1,2.1 3 1\n%s.go:3.1,4.1 1 0\n", p, p)
		}
		f.inputs = append(f.inputs, in)
		f.profiles = append(f.profiles, "mode: set\n"+profile)
		f.receipts = append(f.receipts, Receipt{
			Version:       ReceiptVersion,
			CanopyVersion: "v0.9.0",
			Index:         i + 1,
			Total:         3,
			Metrics:       ReceiptMetrics{File: metricsFile, Env: env, Profile: []string{"sha256:run"}},
			Runner:        ReceiptRunner{CPUs: 2},
			Units:         units,
			Planned:       planned,
			LoadMS:        int64(100 * len(planned)),
			Reported:      planned,
			Passed:        true,
			ElapsedMS:     int64(2000 + 100*i),
			Tests:         TestTally{Passed: 10 + i, Skipped: i},
			Failures:      []Failure{},
			Observations:  obs,
			Coverprofile:  fmt.Sprintf("shard-%d.coverprofile", i+1),
		})
	}
	return f
}

func prefixed(prefix string, items []string) []string {
	var out []string
	for _, s := range items {
		out = append(out, prefix+s)
	}
	return out
}

// seal fills each receipt's digests and input lines from its Inputs and units, the way a shard would.
func (f *joinFixture) seal() {
	for i := range f.receipts {
		in := f.inputs[i]
		rc := &f.receipts[i]
		in.Weights = WeightLines(strings.TrimPrefix(in.Weights[0], "source "), rc.Units)
		rc.Digest = in.Digest()
		rc.Inputs = map[string]ReceiptInput{}
		for _, g := range in.Groups() {
			ri := ReceiptInput{Digest: g.Digest(), Lines: g.Lines}
			switch g.Name {
			case "selection":
				ri.Lines = slices.DeleteFunc(slices.Clone(g.Lines), func(l string) bool { return len(l) > 8 && l[:8] == "package " })
			case "weights":
				ri = ReceiptInput{Digest: g.Digest(), Source: g.Lines[0][len("source "):]}
				for _, l := range g.Lines[1:] {
					if l[len(l)-4:] == " est" {
						ri.Estimated++
					} else {
						ri.Measured++
					}
				}
			}
			rc.Inputs[g.Name] = ri
		}
	}
}

// write puts the receipts and coverprofiles in <dir>/out, the way the shards' artifacts land.
func (f *joinFixture) write(t *testing.T, shardDir string) []LoadedReceipt {
	t.Helper()
	out := OutDir(shardDir)
	require.NoError(t, os.MkdirAll(out, 0o755))
	for i, rc := range f.receipts {
		path := filepath.Join(out, fmt.Sprintf("shard-%d.json", i+1))
		if slices.Contains(f.garbage, rc.Index) {
			require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
			continue
		}
		require.NoError(t, WriteReceipt(path, rc))
		if rc.Coverprofile != "" {
			require.NoError(t, os.WriteFile(filepath.Join(out, rc.Coverprofile), []byte(f.profiles[i]), 0o600))
		}
	}
	loaded, err := LoadDir(out)
	require.NoError(t, err)
	return loaded
}

// writeOldMetrics writes a metrics file every package has samples in and returns its digest.
func writeOldMetrics(t *testing.T, shardDir string) string {
	t.Helper()
	m := &Metrics{Version: metricsVersion, Env: env, Profile: []string{"sha256:run"}, CPUs: 2, Packages: map[string]PackageMetrics{}}
	for _, p := range allPkgs {
		m.Packages[p] = PackageMetrics{MS: []int64{100}}
	}
	path := MetricsPath(shardDir, env, []string{"sha256:run"})
	require.NoError(t, WriteMetrics(path, m))
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}

func ptr[T any](v T) *T { return &v }

func kinds(r Report) []string {
	var out []string
	for _, p := range r.Problems {
		out = append(out, p.Kind)
	}
	return out
}

func problemOf(t *testing.T, r Report, kind string) Problem {
	t.Helper()
	for _, p := range r.Problems {
		if p.Kind == kind {
			return p
		}
	}
	require.Failf(t, "no problem", "kind %q not in %v", kind, kinds(r))
	return Problem{}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(f *joinFixture)
		coverMin *float64
		golden   string
		wantExit int
		want     []string // problem kinds, in order
		check    func(t *testing.T, r Report)
	}{
		{
			name:     "happy path",
			golden:   "happy",
			wantExit: ExitPass,
			check: func(t *testing.T, r Report) {
				assert.Equal(t, "pass", r.Result)
				assert.Len(t, r.Digests, 1)
				assert.Equal(t, 6, r.Checks.Verified.RanOnce)
				assert.Equal(t, 75.0, *r.Checks.Coverage.Percent)
				assert.True(t, r.Checks.Metrics.Written)
				assert.Equal(t, 6, r.Checks.Metrics.Packages)
				assert.Equal(t, int64(200), *r.Shards[0].EstimatedMS)
				assert.False(t, r.Suggestion.Static)
				assert.Empty(t, r.Warnings)
			},
		},
		{
			name:     "missing shard",
			mutate:   func(f *joinFixture) { f.drop(2) },
			wantExit: ExitUnverified,
			want:     []string{KindMissingShard},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindMissingShard)
				assert.Equal(t, []int{2}, p.Shards)
				assert.Equal(t, "missing receipt for shard 2/3 (job failed before canopy finished, or artifact not uploaded)", p.Message)
				assert.Empty(t, p.Hint)
				assert.Len(t, r.Shards, 3)
				assert.False(t, r.Shards[1].Present)
				assert.Nil(t, r.Shards[1].ReportResult)
			},
		},
		{
			name: "missing shard next to a failed one hints at fail-fast",
			mutate: func(f *joinFixture) {
				f.drop(2)
				f.receipts[0].Passed = false
			},
			wantExit: ExitUnverified,
			want:     []string{KindMissingShard, KindShardFailed},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, "cancelled? use fail-fast: false", problemOf(t, r, KindMissingShard).Hint)
			},
		},
		{
			name: "duplicate shard",
			mutate: func(f *joinFixture) {
				f.receipts[2].Index = 2
			},
			wantExit: ExitUnverified,
			want:     []string{KindDuplicateShard, KindMissingShard},
			check: func(t *testing.T, r Report) {
				assert.Contains(t, problemOf(t, r, KindDuplicateShard).Message, "shard 2/3 has 2 receipts: shard-2, shard-3")
			},
		},
		{
			name: "metrics vs static weights",
			mutate: func(f *joinFixture) {
				var units []Unit
				for _, p := range allPkgs {
					units = append(units, Unit{Package: p, Weight: 1, Estimated: true})
				}
				f.inputs[2].Weights = []string{"source " + SourceStatic}
				f.receipts[2].Units = units
				f.receipts[2].Metrics = ReceiptMetrics{Env: env, Profile: []string{"sha256:run"}, Ignored: "no metrics file"}
			},
			golden:   "weights_mismatch",
			wantExit: ExitUnverified,
			want:     []string{KindInputMismatch},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindInputMismatch)
				assert.Equal(t, "weights", p.Group)
				assert.Equal(t, []int{1, 2}, p.Baseline)
				assert.Equal(t, []int{3}, p.Differing)
				assert.Equal(t, map[string][]int{"metrics": {1, 2}, "static": {3}}, p.Values["source"])
				assert.Contains(t, p.Message, "[weights] shards 1,2: metrics sha256:")
				assert.Contains(t, p.Message, "(6 measured, 0 estimated) / shard 3: static (no metrics file)")
				assert.Len(t, r.Digests, 2)
				// the plans agree, so the cover checks still ran and passed
				assert.True(t, r.Checks.Tests.OK)
				assert.Equal(t, 6, r.Checks.Verified.RanOnce)
			},
		},
		{
			name: "a -run flag on one shard",
			mutate: func(f *joinFixture) {
				f.inputs[1].Run = append(f.inputs[1].Run, "test-flag -run=TestDoesNotExist")
			},
			golden:   "run_flag",
			wantExit: ExitUnverified,
			want:     []string{KindInputMismatch},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindInputMismatch)
				assert.Equal(t, "run", p.Group)
				assert.Equal(t, []int{2}, p.Differing)
				assert.Equal(t, map[int][]string{2: {"test-flag -run=TestDoesNotExist"}}, p.OnlyIn)
				assert.Equal(t, "[run] shard 2 only: test-flag -run=TestDoesNotExist", p.Message)
				assert.Equal(t, "a flag or CANOPY_TEST_* env var is set on some jobs only", p.Hint)
			},
		},
		{
			name: "different toolchain",
			mutate: func(f *joinFixture) {
				f.inputs[2].Go[0] = "goversion go1.26.4"
			},
			wantExit: ExitUnverified,
			want:     []string{KindInputMismatch},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindInputMismatch)
				assert.Equal(t, "go", p.Group)
				assert.Equal(t, "[go] shards 1,2: goversion go1.27.1 / shard 3: goversion go1.26.4", p.Message)
			},
		},
		{
			name: "different commit",
			mutate: func(f *joinFixture) {
				f.inputs[0].Source[0] = "commit ffffff"
			},
			wantExit: ExitUnverified,
			want:     []string{KindInputMismatch},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindInputMismatch)
				assert.Equal(t, "source", p.Group)
				assert.Equal(t, "shards checked out different commits", p.Hint)
				assert.Equal(t, "[source] shard 1: commit ffffff / shards 2,3: commit a1b2c3", p.Message)
			},
		},
		{
			name: "different package selection",
			mutate: func(f *joinFixture) {
				f.inputs[2].Selection = append(slices.Clone(f.inputs[2].Selection), "package m/g", "package m/h")
				f.receipts[2].Units = append(slices.Clone(f.receipts[2].Units), Unit{Package: "m/g", Weight: 100}, Unit{Package: "m/h", Weight: 100})
			},
			wantExit: ExitUnverified,
			// more units means more weight lines, so [weights] differs too
			want: []string{KindInputMismatch, KindInputMismatch},
			check: func(t *testing.T, r Report) {
				p := problemOf(t, r, KindInputMismatch)
				assert.Equal(t, "[selection] shard 3 has 2 packages the others don't: m/g, m/h", p.Message)
			},
		},
		{
			name: "mismatched total",
			mutate: func(f *joinFixture) {
				f.receipts[2].Total = 4
			},
			wantExit: ExitUnverified,
			want:     []string{KindTotalMismatch},
			check: func(t *testing.T, r Report) {
				assert.Contains(t, r.Problems[0].Message, "receipts disagree on shard total: shard-1 says 3, shard-3 says 4 (stale files in")
				assert.Len(t, r.Shards, 3)
			},
		},
		{
			name: "planned but not reported",
			mutate: func(f *joinFixture) {
				f.receipts[1].Reported = []string{"m/c"}
			},
			wantExit: ExitUnverified,
			want:     []string{KindPlannedNotReported},
			check: func(t *testing.T, r Report) {
				p := r.Problems[0]
				assert.Equal(t, []int{2}, p.Shards)
				assert.Equal(t, "shard 2 never reported 1 package it planned", p.Message)
				assert.Equal(t, []string{"m/d"}, p.Packages)
			},
		},
		{
			name: "digest mismatch still finds dropped and doubled packages",
			mutate: func(f *joinFixture) {
				f.inputs[2].Source[0] = "commit ffffff"
				f.receipts[2].Planned = []string{"m/d", "m/e"}
				f.receipts[2].Reported = []string{"m/d", "m/e"}
			},
			wantExit: ExitUnverified,
			want:     []string{KindInputMismatch, KindNeverRan, KindRanTwice},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, []string{"m/f"}, problemOf(t, r, KindNeverRan).Packages)
				twice := problemOf(t, r, KindRanTwice)
				assert.Equal(t, []string{"m/d"}, twice.Packages)
				assert.Equal(t, []int{2, 3}, twice.Shards)
				assert.False(t, r.Checks.Metrics.Written, "unverified timings must not be saved")
				assert.Equal(t, "shards failed verification", r.Checks.Metrics.Warning)
			},
		},
		{
			name: "failed shard",
			mutate: func(f *joinFixture) {
				f.receipts[1].Passed = false
				f.receipts[1].Tests.Failed = 1
				f.receipts[1].Failures = []Failure{{Package: "m/d", Test: "TestReconcile"}}
				delete(f.receipts[1].Observations, "m/d")
			},
			golden:   "failed_shard",
			wantExit: ExitTestsFailed,
			want:     []string{KindShardFailed},
			check: func(t *testing.T, r Report) {
				p := r.Problems[0]
				assert.Equal(t, CheckTests, p.Check)
				assert.Equal(t, "shard 2/3 failed: 1 failed test in 1 package", p.Message)
				assert.False(t, r.Checks.Tests.OK)
				assert.True(t, r.Checks.Verified.OK)
				assert.Equal(t, 5, r.Checks.Metrics.Packages)
			},
		},
		{
			name:     "covermin on the join, below",
			coverMin: ptr(80.0),
			wantExit: ExitGateFailed,
			want:     []string{KindCoverageBelow},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, ThresholdJoin, r.Checks.Coverage.ThresholdSource)
				assert.Equal(t, "coverage below threshold: 75.00% < 80.00%", r.Problems[0].Message)
			},
		},
		{
			name:     "covermin only in receipts",
			mutate:   func(f *joinFixture) { f.setCoverMin(ptr(70.0), ptr(70.0), ptr(70.0)) },
			wantExit: ExitPass,
			check: func(t *testing.T, r Report) {
				assert.Equal(t, ThresholdReceipts, r.Checks.Coverage.ThresholdSource)
				assert.Equal(t, 70.0, *r.Checks.Coverage.Threshold)
			},
		},
		{
			name:     "covermin only in receipts, below",
			mutate:   func(f *joinFixture) { f.setCoverMin(ptr(90.0), ptr(90.0), ptr(90.0)) },
			wantExit: ExitGateFailed,
			want:     []string{KindCoverageBelow},
		},
		{
			name: "shards disagree on covermin",
			mutate: func(f *joinFixture) {
				f.setCoverMin(ptr(70.0), ptr(70.0), nil)
				f.inputs[2].Gates = f.inputs[0].Gates // keep the digests equal to see the conflict alone
			},
			wantExit: ExitGateFailed,
			want:     []string{KindGateConflict},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, "shards disagree on covermin: shards 1,2: 70%, shard 3: unset", r.Problems[0].Message)
				assert.Nil(t, r.Checks.Coverage.Threshold)
			},
		},
		{
			name:     "explicit join value overrides receipts",
			mutate:   func(f *joinFixture) { f.setCoverMin(ptr(90.0), ptr(90.0), ptr(90.0)) },
			coverMin: ptr(70.0),
			wantExit: ExitPass,
			check: func(t *testing.T, r Report) {
				c := r.Checks.Coverage
				assert.Equal(t, ThresholdJoin, c.ThresholdSource)
				assert.Equal(t, 70.0, *c.Threshold)
				assert.Equal(t, 90.0, *c.ReceiptThreshold)
				assert.Equal(t, []string{"coverage threshold 70% set on the join overrides 90% recorded by shards"}, r.Warnings)
			},
		},
		{
			name:     "coverage on some shards only",
			mutate:   func(f *joinFixture) { f.receipts[1].Coverprofile = "" },
			wantExit: ExitGateFailed,
			want:     []string{KindCoverageMissing},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, []int{2}, r.Problems[0].Shards)
				assert.Contains(t, r.Problems[0].Message, "coverage enabled on some shards but not others")
			},
		},
		{
			name: "covermin but no coverage",
			mutate: func(f *joinFixture) {
				for i := range f.receipts {
					f.receipts[i].Coverprofile = ""
				}
			},
			coverMin: ptr(80.0),
			wantExit: ExitGateFailed,
			want:     []string{KindCoverageMissing},
			check: func(t *testing.T, r Report) {
				assert.Equal(t, "coverage threshold 80% set but no shard collected coverage", r.Problems[0].Message)
			},
		},
		{
			name:     "unreadable receipt",
			mutate:   func(f *joinFixture) { f.garbage = []int{2} },
			wantExit: ExitCannotRun,
			want:     []string{KindUnreadableReceipt},
			check: func(t *testing.T, r Report) {
				assert.Contains(t, r.Problems[0].Message, "unable to read receipt shard-2.json")
				assert.Nil(t, r.Suggestion)
			},
		},
		{
			name:     "no receipts",
			mutate:   func(f *joinFixture) { f.receipts = nil },
			wantExit: ExitCannotRun,
			want:     []string{KindUnreadableReceipt},
		},
		{
			name: "verification outranks gates and failures",
			mutate: func(f *joinFixture) {
				f.drop(3)
				f.receipts[0].Passed = false
			},
			coverMin: ptr(80.0),
			wantExit: ExitUnverified,
			want:     []string{KindMissingShard, KindShardFailed, KindCoverageBelow},
		},
		{
			name:     "gates outrank failures",
			mutate:   func(f *joinFixture) { f.receipts[0].Passed = false },
			coverMin: ptr(80.0),
			wantExit: ExitGateFailed,
			want:     []string{KindShardFailed, KindCoverageBelow},
		},
		{
			name: "canopy version skew warns",
			mutate: func(f *joinFixture) {
				for i := range f.receipts {
					f.receipts[i].CanopyVersion = "v0.8.0"
				}
			},
			wantExit: ExitPass,
			check: func(t *testing.T, r Report) {
				assert.Equal(t, []string{"join is canopy v0.9.0 but shards ran v0.8.0; pin the same version on every job"}, r.Warnings)
			},
		},
		{
			name: "shards on different platforms still verify but skip metrics",
			mutate: func(f *joinFixture) {
				f.receipts[2].Metrics.Env = Env{GOOS: "darwin", GOARCH: "arm64"}
			},
			wantExit: ExitPass,
			check: func(t *testing.T, r Report) {
				assert.False(t, r.Checks.Metrics.Written)
				assert.Contains(t, r.Checks.Metrics.Warning, "shards disagree on env")
				// the suggestion comes from the units the shards planned with, not the merge
				assert.False(t, r.Suggestion.Static)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			shardDir := filepath.Join(".canopy", "shard")
			require.NoError(t, os.MkdirAll(shardDir, 0o755))

			f := newJoinFixture(writeOldMetrics(t, shardDir))
			if tt.mutate != nil {
				tt.mutate(f)
			}
			f.seal()
			loaded := f.write(t, shardDir)

			r, err := Join(JoinInput{
				Receipts:      loaded,
				OutDir:        OutDir(shardDir),
				ShardDir:      shardDir,
				CoverMin:      tt.coverMin,
				WriteMetrics:  true,
				Overhead:      time.Second,
				CanopyVersion: "v0.9.0",
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantExit, r.ExitCode, "problems: %+v", r.Problems)
			assert.Equal(t, tt.want, kinds(r))
			assert.Len(t, r.Shards, r.Total)
			if tt.check != nil {
				tt.check(t, r)
			}
			if tt.golden != "" {
				assertGolden(t, r, tt.golden)
			}
		})
	}
}

// drop removes the receipt for shard i.
func (f *joinFixture) drop(i int) {
	f.inputs = slices.Delete(f.inputs, i-1, i)
	f.receipts = slices.Delete(f.receipts, i-1, i)
	f.profiles = slices.Delete(f.profiles, i-1, i)
}

// setCoverMin sets each shard's recorded covermin and its [gates] lines.
func (f *joinFixture) setCoverMin(v ...*float64) {
	for i, c := range v {
		f.receipts[i].Gates.CoverMin = c
		f.inputs[i].Gates = nil
		if c != nil {
			f.inputs[i].Gates = []string{fmt.Sprintf("covermin %v", *c)}
		}
	}
}

// assertGolden compares the report JSON with testdata/join/<name>.json. The renderers reuse these
// files as their input fixtures.
func assertGolden(t *testing.T, r Report, name string) {
	t.Helper()
	got, err := json.MarshalIndent(r, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')

	// t.Chdir moved us into the temp dir, so resolve testdata from the package dir
	path := filepath.Join(pkgDir, "testdata", "join", name+".json")
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))

	// the golden must round-trip, since renderers load it back into a Report
	var back Report
	require.NoError(t, json.Unmarshal(want, &back))
	assert.Equal(t, r, back)
}

var pkgDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}()

func TestIsBareFileName(t *testing.T) {
	for name, want := range map[string]bool{
		"shard-1.coverprofile": true,
		"../x.coverprofile":    false,
		"/etc/passwd":          false,
		"sub/x":                false,
		`..\x`:                 false,
		"..":                   false,
	} {
		if got := isBareFileName(name); got != want {
			t.Errorf("isBareFileName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestJoin_UnitsMustMatchWeightsDigest(t *testing.T) {
	dir := t.TempDir()
	f := newJoinFixture("")
	f.seal()
	f.receipts[1].Units[0].Weight = 999
	r, err := Join(JoinInput{Receipts: f.write(t, dir), OutDir: OutDir(dir), ShardDir: dir})
	require.NoError(t, err)
	assert.Equal(t, ExitUnverified, r.ExitCode)
	p := problemOf(t, r, KindInputMismatch)
	assert.Equal(t, "[weights] shard 2: its units don't match its weights digest", p.Message)
	assert.Nil(t, r.Suggestion)
}

func TestJoin_SuggestionMatchesPlan(t *testing.T) {
	dir := t.TempDir()
	f := newJoinFixture("")
	f.seal()
	r, err := Join(JoinInput{Receipts: f.write(t, dir), OutDir: OutDir(dir), ShardDir: dir, Overhead: time.Second})
	require.NoError(t, err)
	// what `shard plan` computes from the same units and runner cpus
	assert.Equal(t, NewSuggestion(f.receipts[0].Units, 3, time.Second, SuggestionP(nil, 2)), r.Suggestion)
}
