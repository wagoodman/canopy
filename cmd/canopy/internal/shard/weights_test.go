package shard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestTestCounts(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}

	// 2 functions, one with a 3 case table
	write("internal/a_test.go", `package internal
import "testing"
func TestOne(t *testing.T) {}
func TestTable(t *testing.T) {
	for _, tt := range []struct{ name string }{{name: "x"}, {name: "y"}, {name: "z"}} {
		t.Run(tt.name, func(t *testing.T) {})
	}
}
`)
	// external test package
	write("external/e.go", "package external\n")
	write("external/e_test.go", `package external_test
import "testing"
func TestA(t *testing.T) {}
func TestB(t *testing.T) {}
`)
	write("notests/n.go", "package notests\n")

	c := golist.NewPackageCollection(
		golist.Package{Dir: filepath.Join(root, "internal"), ImportPath: "m/internal"},
		golist.Package{Dir: filepath.Join(root, "external"), ImportPath: "m/external"},
		golist.Package{Dir: filepath.Join(root, "notests"), ImportPath: "m/notests"},
	)

	got, err := TestCounts(c)
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"m/internal": 5, "m/external": 2, "m/notests": 0}, got)
}

func TestWeights(t *testing.T) {
	env := Env{GOOS: "linux", GOARCH: "arm64"}
	counts := map[string]int64{"a": 9, "b": 1, "c": 199, "d": 0}
	metrics := func(pkgs map[string][]int64) *Metrics {
		m := &Metrics{Version: 1, Env: env, Profile: []string{"p"}, CPUs: 4, Packages: map[string]PackageMetrics{}}
		for p, ms := range pkgs {
			m.Packages[p] = PackageMetrics{MS: ms}
		}
		return m
	}

	t.Run("static without metrics", func(t *testing.T) {
		r := Weights(counts, nil, nil, env, []string{"p"})
		assert.Equal(t, SourceStatic, r.Source)
		assert.Equal(t, "no metrics file", r.Ignored)
		assert.Equal(t, []Unit{
			{Package: "a", Weight: 10, Estimated: true},
			{Package: "b", Weight: 2, Estimated: true},
			{Package: "c", Weight: 200, Estimated: true},
			{Package: "d", Weight: 1, Estimated: true},
		}, r.Units)
		assert.Equal(t, 4, r.Estimated)
	})

	t.Run("one measured package keeps the test-count ratios for the rest", func(t *testing.T) {
		// a: s=10, lower median of [300 100 200 400] is 200, so 20ms per unit of s
		r := Weights(counts, metrics(map[string][]int64{"a": {300, 100, 200, 400}}), nil, env, []string{"p"})
		assert.Equal(t, SourceMetrics, r.Source)
		assert.Empty(t, r.Ignored)
		assert.Equal(t, 1, r.Measured)
		assert.Equal(t, 3, r.Estimated)
		assert.Equal(t, []Unit{
			{Package: "a", Weight: 200},
			{Package: "b", Weight: 40, Estimated: true},
			{Package: "c", Weight: 4000, Estimated: true},
			{Package: "d", Weight: 20, Estimated: true},
		}, r.Units)
	})

	t.Run("new package with more tests is estimated heavier", func(t *testing.T) {
		r := Weights(counts, metrics(map[string][]int64{"a": {5000}, "d": {10}}), nil, env, []string{"p"})
		w := map[string]int64{}
		for _, u := range r.Units {
			w[u.Package] = u.Weight
		}
		assert.Greater(t, w["c"], w["b"])
		assert.GreaterOrEqual(t, w["b"], int64(1))
	})

	t.Run("estimates are at least 1", func(t *testing.T) {
		r := Weights(counts, metrics(map[string][]int64{"c": {0}}), nil, env, []string{"p"})
		for _, u := range r.Units {
			if u.Estimated {
				assert.Equal(t, int64(1), u.Weight, u.Package)
			}
		}
	})

	t.Run("different cpu count still uses metrics", func(t *testing.T) {
		m := metrics(map[string][]int64{"a": {100}})
		m.CPUs = 64
		assert.Equal(t, SourceMetrics, Weights(counts, m, nil, env, []string{"p"}).Source)
	})

	dir := t.TempDir()
	file := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}
	fallbacks := []struct {
		name   string
		path   string
		env    Env
		reason string
	}{
		{name: "missing", path: filepath.Join(dir, "nope.json"), env: env, reason: "no metrics file"},
		{name: "corrupt", path: file("corrupt.json", "{"), env: env, reason: "metrics ignored, corrupt file: unexpected end of JSON input"},
		{name: "version", path: file("v2.json", `{"version": 2}`), env: env, reason: "metrics ignored, unsupported version 2"},
		{name: "env", path: file("env.json", `{"version": 1, "env": {"goos": "linux", "goarch": "amd64"}, "profile": ["p"], "packages": {"a": {"ms": [1]}}}`), env: env, reason: "metrics ignored, recorded on linux/amd64 but this is linux/arm64"},
		{name: "profile", path: file("profile.json", `{"version": 1, "env": {"goos": "linux", "goarch": "arm64"}, "profile": ["cover true", "p"], "packages": {"a": {"ms": [1]}}}`), env: env, reason: `metrics ignored, recorded with "cover true" but this run has none of those`},
		{name: "no overlap", path: file("overlap.json", `{"version": 1, "env": {"goos": "linux", "goarch": "arm64"}, "profile": ["p"], "packages": {"gone": {"ms": [1]}}}`), env: env, reason: "metrics ignored, none of these packages were measured"},
	}
	t.Run("profile diff", func(t *testing.T) {
		m := &Metrics{Version: 1, Env: env, Profile: []string{"cover true", "test-flag -race"}}
		r := Weights(counts, m, nil, env, []string{"cover false", "test-flag -race"})
		assert.Equal(t, `metrics ignored, recorded with "cover true" but this run has "cover false"`, r.Ignored)
	})

	for _, tt := range fallbacks {
		t.Run("fallback "+tt.name, func(t *testing.T) {
			m, _, err := LoadMetrics(tt.path)
			r := Weights(counts, m, err, tt.env, []string{"p"})
			assert.Equal(t, SourceStatic, r.Source)
			assert.Equal(t, tt.reason, r.Ignored)
			assert.Equal(t, tt.name != "missing", r.FellBack())
			assert.Equal(t, Weights(counts, nil, nil, env, []string{"p"}).Units, r.Units)
		})
	}
}
