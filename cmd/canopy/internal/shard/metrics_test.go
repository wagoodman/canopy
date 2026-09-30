package shard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	env := Env{GOOS: "linux", GOARCH: "arm64"}
	obs := func(ms map[string]int64) Observations {
		return Observations{Env: env, Profile: "p", CPUs: 4, MS: ms}
	}

	t.Run("a 6th sample drops the oldest", func(t *testing.T) {
		var m *Metrics
		for i := int64(1); i <= 6; i++ {
			var warn string
			m, warn = Merge(m, []Observations{obs(map[string]int64{"a": i})}, []string{"a"})
			require.Empty(t, warn)
		}
		assert.Equal(t, []int64{2, 3, 4, 5, 6}, m.Packages["a"].MS)
	})

	t.Run("observations from several shards", func(t *testing.T) {
		m, _ := Merge(nil, []Observations{obs(map[string]int64{"a": 1}), obs(map[string]int64{"b": 2})}, []string{"a", "b", "c"})
		assert.Equal(t, map[string]PackageMetrics{"a": {MS: []int64{1}}, "b": {MS: []int64{2}}}, m.Packages)
	})

	t.Run("a package missing from 11 saves is dropped", func(t *testing.T) {
		m, _ := Merge(nil, []Observations{obs(map[string]int64{"a": 1, "gone": 1})}, []string{"a", "gone"})
		for i := 1; i <= 11; i++ {
			m, _ = Merge(m, []Observations{obs(map[string]int64{"a": 1})}, []string{"a"})
			if i == 10 {
				assert.Equal(t, 10, m.Packages["gone"].Unseen)
			}
		}
		assert.NotContains(t, m.Packages, "gone")
	})

	t.Run("a subset run keeps unselected packages", func(t *testing.T) {
		m, _ := Merge(nil, []Observations{obs(map[string]int64{"a": 1, "b": 2})}, []string{"a", "b"})
		m, _ = Merge(m, []Observations{obs(map[string]int64{"a": 3})}, []string{"a"})
		assert.Equal(t, PackageMetrics{MS: []int64{2}, Unseen: 1}, m.Packages["b"])
		// selected again but served from the test cache: seen, no new sample
		m, _ = Merge(m, []Observations{obs(map[string]int64{"a": 3})}, []string{"a", "b"})
		assert.Equal(t, PackageMetrics{MS: []int64{2}}, m.Packages["b"])
	})

	t.Run("env reset", func(t *testing.T) {
		old := &Metrics{Version: 1, Env: Env{GOOS: "linux", GOARCH: "amd64"}, Profile: "p", Packages: map[string]PackageMetrics{"a": {MS: []int64{9}}, "x": {MS: []int64{9}}}}
		m, _ := Merge(old, []Observations{obs(map[string]int64{"a": 1})}, []string{"a"})
		assert.Equal(t, map[string]PackageMetrics{"a": {MS: []int64{1}}}, m.Packages)
		assert.Equal(t, env, m.Env)
	})

	t.Run("profile reset", func(t *testing.T) {
		old := &Metrics{Version: 1, Env: env, Profile: "other", Packages: map[string]PackageMetrics{"a": {MS: []int64{9}}}}
		m, _ := Merge(old, []Observations{obs(map[string]int64{"a": 1})}, []string{"a"})
		assert.Equal(t, "p", m.Profile)
		assert.Equal(t, []int64{1}, m.Packages["a"].MS)
	})

	t.Run("shards disagree", func(t *testing.T) {
		other := obs(nil)
		other.Env.GOARCH = "amd64"
		m, warn := Merge(nil, []Observations{obs(nil), other}, nil)
		assert.Nil(t, m)
		assert.Equal(t, "shards disagree on env (linux/arm64 vs linux/amd64), metrics not saved", warn)

		other = obs(nil)
		other.Profile = "q"
		m, warn = Merge(nil, []Observations{obs(nil), other}, nil)
		assert.Nil(t, m)
		assert.Equal(t, "shards disagree on test profile, metrics not saved", warn)
	})

	t.Run("cpus overwritten", func(t *testing.T) {
		old := &Metrics{Version: 1, Env: env, Profile: "p", CPUs: 2}
		m, _ := Merge(old, []Observations{obs(nil)}, nil)
		assert.Equal(t, 4, m.CPUs)
	})
}

func TestWriteLoadMetrics(t *testing.T) {
	env := Env{GOOS: "linux", GOARCH: "arm64"}
	build := func() *Metrics {
		m, _ := Merge(nil, []Observations{
			{Env: env, Profile: "p", CPUs: 4, MS: map[string]int64{"z": 1, "a": 2, "m": 3}},
		}, []string{"z", "a", "m"})
		return m
	}

	dir := t.TempDir()
	p1, p2 := filepath.Join(dir, "one", "metrics.json"), filepath.Join(dir, "two.json")
	require.NoError(t, WriteMetrics(p1, build()))
	require.NoError(t, WriteMetrics(p2, build()))

	b1, err := os.ReadFile(p1)
	require.NoError(t, err)
	b2, err := os.ReadFile(p2)
	require.NoError(t, err)
	assert.Equal(t, b1, b2)
	assert.Equal(t, `{
  "version": 1,
  "env": {
    "goos": "linux",
    "goarch": "arm64"
  },
  "profile": "p",
  "cpus": 4,
  "packages": {
    "a": {
      "ms": [
        2
      ],
      "unseen": 0
    },
    "m": {
      "ms": [
        3
      ],
      "unseen": 0
    },
    "z": {
      "ms": [
        1
      ],
      "unseen": 0
    }
  }
}
`, string(b1))

	m, d1, err := LoadMetrics(p1)
	require.NoError(t, err)
	assert.Equal(t, build(), m)
	_, d2, _ := LoadMetrics(p2)
	assert.Equal(t, d1, d2)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, d1)

	entries, err := os.ReadDir(filepath.Dir(p1))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "temp file left behind")
}
