package shard

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"

	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

const (
	SourceMetrics = "metrics"
	SourceStatic  = "static"
)

// Env is the part of the environment that decides which files build, so metrics from another Env
// describe different code.
type Env struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

func (e Env) String() string {
	return e.GOOS + "/" + e.GOARCH
}

// TestCounts returns the number of test functions plus discovered t.Run and table cases for every
// package in the collection (0 for packages without tests). External test packages (package x_test)
// live in the same directory, so they count toward the package they test.
func TestCounts(collection *golist.PackageCollection) (map[string]int64, error) {
	defs, err := gotest.FindDefinitions(collection)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, collection.Size())
	for _, p := range collection.ImportPaths() {
		counts[p] = 0
	}
	for _, d := range defs {
		if _, ok := counts[d.ImportPath]; ok {
			counts[d.ImportPath] += int64(len(d.References()))
		}
	}
	return counts, nil
}

// WeightResult is the weighted unit list plus where the weights came from.
type WeightResult struct {
	Units     []Unit // sorted by Package
	Source    string // SourceMetrics or SourceStatic
	Measured  int
	Estimated int
	Ignored   string // why metrics weren't used, empty when they were
}

// Weights turns test counts into units. m and loadErr are the results of LoadMetrics; metrics are
// used only when they loaded and match env and profile. Measured packages weigh the lower median
// of their samples, the rest are test counts scaled into ms by the measured packages.
func Weights(counts map[string]int64, m *Metrics, loadErr error, env Env, profile string) WeightResult {
	pkgs := make([]string, 0, len(counts))
	for p := range counts {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	ignored := metricsIgnored(m, loadErr, env, profile)

	measured := map[string]int64{}
	var sumM, sumS int64
	if ignored == "" {
		for _, p := range pkgs {
			if e, ok := m.Packages[p]; ok && len(e.MS) > 0 {
				measured[p] = lowerMedian(e.MS)
				sumM += measured[p]
				sumS += counts[p] + 1
			}
		}
		if len(measured) == 0 {
			ignored = "no metrics for the current packages"
		}
	}

	r := WeightResult{Source: SourceStatic, Ignored: ignored}
	if ignored == "" {
		r.Source = SourceMetrics
	}
	for _, p := range pkgs {
		s := counts[p] + 1
		if w, ok := measured[p]; ok {
			r.Units = append(r.Units, Unit{Package: p, Weight: w})
			r.Measured++
			continue
		}
		w := s
		if r.Source == SourceMetrics {
			w = max(s*sumM/sumS, 1)
		}
		r.Units = append(r.Units, Unit{Package: p, Weight: w, Estimated: true})
		r.Estimated++
	}
	return r
}

func metricsIgnored(m *Metrics, loadErr error, env Env, profile string) string {
	switch {
	case errors.Is(loadErr, fs.ErrNotExist):
		return "no metrics file"
	case loadErr != nil:
		return loadErr.Error()
	case m == nil:
		return "no metrics file"
	case m.Env != env:
		return fmt.Sprintf("metrics recorded on %s, this is %s", m.Env, env)
	case m.Profile != profile:
		return "metrics recorded with a different test profile"
	}
	return ""
}

func lowerMedian(ms []int64) int64 {
	s := slices.Clone(ms)
	slices.Sort(s)
	return s[(len(s)-1)/2]
}
