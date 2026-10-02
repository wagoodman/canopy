package shard

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

const (
	SourceMetrics = "metrics"
	SourceStatic  = "static"

	// NoMetricsFile is why a run without a metrics file weighs by test count. Unlike the other reasons it isn't a
	// fallback, it's how every setup starts.
	NoMetricsFile = "no metrics file"
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

// FellBack reports whether there were metrics that couldn't be used, so the split fell back to test counts.
func (w WeightResult) FellBack() bool {
	return w.Source == SourceStatic && w.Ignored != NoMetricsFile
}

// Weights turns test counts into units. m and loadErr are the results of LoadMetrics; metrics are
// used only when they loaded and match env and profile. Measured packages weigh the lower median
// of their samples, the rest are test counts scaled into ms by the measured packages.
func Weights(counts map[string]int64, m *Metrics, loadErr error, env Env, profile []string) WeightResult {
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
			ignored = "metrics ignored, none of these packages were measured"
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

// metricsIgnored says why metrics can't be used. Every reason but NoMetricsFile starts with "metrics ignored", so
// wherever it's shown it reads as a fallback.
func metricsIgnored(m *Metrics, loadErr error, env Env, profile []string) string {
	switch {
	case errors.Is(loadErr, fs.ErrNotExist), loadErr == nil && m == nil:
		return NoMetricsFile
	case loadErr != nil:
		return "metrics ignored, " + loadErr.Error()
	case m.Env != env:
		return fmt.Sprintf("metrics ignored, recorded on %s but this is %s", m.Env, env)
	case !slices.Equal(m.Profile, profile):
		return fmt.Sprintf("metrics ignored, recorded with %s but this run has %s", onlyIn(m.Profile, profile), onlyIn(profile, m.Profile))
	}
	return ""
}

// onlyIn quotes the lines of a that aren't in b, for saying how two profiles differ.
func onlyIn(a, b []string) string {
	var out []string
	for _, l := range a {
		if !slices.Contains(b, l) {
			out = append(out, strconv.Quote(l))
		}
	}
	if len(out) == 0 {
		return "none of those"
	}
	return strings.Join(out, ", ")
}

func lowerMedian(ms []int64) int64 {
	s := slices.Clone(ms)
	slices.Sort(s)
	return s[(len(s)-1)/2]
}
