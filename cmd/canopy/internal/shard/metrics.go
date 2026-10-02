package shard

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	metricsVersion = 1
	maxSamples     = 5
	maxUnseen      = 10
)

// Metrics is the on-disk record of how long packages took, keyed to the environment and test
// profile they were measured in. It holds observations only, never weights or assignments.
type Metrics struct {
	Version  int                       `json:"version"`
	Env      Env                       `json:"env"`
	Profile  []string                  `json:"profile"` // the [run] lines that affect timing (the CLI decides which)
	CPUs     int                       `json:"cpus"`    // informational, never matched
	Packages map[string]PackageMetrics `json:"packages"`
}

type PackageMetrics struct {
	MS     []int64 `json:"ms"`     // oldest first, at most maxSamples
	Unseen int     `json:"unseen"` // consecutive saves this package wasn't part of
}

// Observations is what one shard measured: fresh, passing package times in ms.
type Observations struct {
	Env     Env
	Profile []string
	CPUs    int
	MS      map[string]int64
}

// MetricsPath is where the metrics for one env and profile live under the shard dir. Each identity
// gets its own file, so matrix jobs that share a cache (a -race and a plain run) keep separate
// histories instead of resetting each other's. It's the only place the path is built.
// ponytail: files for profiles nobody runs anymore stay until the cache key prefix is bumped.
func MetricsPath(shardDir string, env Env, profile []string) string {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(profile, "\n"))))[:12]
	return filepath.Join(shardDir, "metrics", fmt.Sprintf("%s-%s-%s.json", env.GOOS, env.GOARCH, key))
}

// LoadMetrics reads a metrics file and returns it with the sha256 digest of its bytes. The digest
// is set whenever the file could be read, even if it doesn't parse. A missing file wraps
// fs.ErrNotExist; parse and version errors read as the reason the metrics were ignored.
func LoadMetrics(path string) (*Metrics, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(b))

	var m Metrics
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, digest, fmt.Errorf("corrupt file: %w", err)
	}
	if m.Version != metricsVersion {
		return nil, digest, fmt.Errorf("unsupported version %d", m.Version)
	}
	return &m, digest, nil
}

// WriteMetrics writes m atomically. The same metrics always produce the same bytes (json sorts map
// keys, and there are no timestamps).
func WriteMetrics(path string, m *Metrics) error {
	return writeJSON(path, m)
}

// writeJSON writes v as indented JSON through a temp file and a rename, so a crash never leaves a
// half-written file behind.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // no-op after a successful rename
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Merge folds this join's observations into old (nil when there is no usable file). units is the
// join's full unit list: packages in it have their unseen counter reset, the rest age and are
// dropped after maxUnseen saves. When the shards disagree on env or profile nothing should be
// written, so Merge returns nil and a warning.
func Merge(old *Metrics, obs []Observations, units []string) (*Metrics, string) {
	if len(obs) == 0 {
		return nil, "no observations to merge"
	}
	env, profile := obs[0].Env, obs[0].Profile
	cpus := 0
	for _, o := range obs {
		if o.Env != env {
			return nil, fmt.Sprintf("shards disagree on env (%s vs %s), metrics not saved", env, o.Env)
		}
		if !slices.Equal(o.Profile, profile) {
			return nil, "shards disagree on test profile, metrics not saved"
		}
		cpus = max(cpus, o.CPUs)
	}

	// a different env or profile means the old samples describe different code or a different run
	if old == nil || old.Env != env || !slices.Equal(old.Profile, profile) {
		old = &Metrics{}
	}

	seen := map[string]bool{}
	for _, u := range units {
		seen[u] = true
	}
	samples := map[string][]int64{}
	for _, o := range obs {
		for p, ms := range o.MS {
			seen[p] = true
			samples[p] = append(samples[p], ms)
		}
	}

	next := &Metrics{Version: metricsVersion, Env: env, Profile: profile, CPUs: cpus, Packages: map[string]PackageMetrics{}}
	for p, e := range old.Packages {
		if !seen[p] {
			e.Unseen++
			if e.Unseen > maxUnseen {
				continue
			}
			next.Packages[p] = e
		}
	}
	for p := range seen {
		// ponytail: packages without any sample get no entry, they carry nothing Weights can use
		ms := append(append([]int64{}, old.Packages[p].MS...), samples[p]...)
		if len(ms) == 0 {
			continue
		}
		next.Packages[p] = PackageMetrics{MS: ms[max(0, len(ms)-maxSamples):]}
	}
	return next, ""
}
