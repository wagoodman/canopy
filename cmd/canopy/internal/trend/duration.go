package trend

import (
	"sort"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// DurationResult is the per-test duration trend across the in-scope runs.
type DurationResult struct {
	Reference  gotest.Reference
	Latest     float64   // most recent elapsed (seconds)
	First      float64   // earliest elapsed (seconds)
	TrendPct   float64   // percent change from first to latest ((last-first)/first*100)
	FreshRuns  int       // runs that actually executed this test (have timing)
	CachedRuns int       // runs where this test's package was served from cache (no timing)
	Series     []float64 // elapsed over time, oldest first (for sparkline/JSON)
}

// OverallSummary is the aggregate duration trend across all analyzed tests.
type OverallSummary struct {
	AvgFirst float64 // mean of each test's earliest elapsed
	AvgLast  float64 // mean of each test's latest elapsed
	TrendPct float64 // percent change from AvgFirst to AvgLast
	Tests    int     // number of tests contributing
}

// Durations computes per-test duration trends from already-collected records.
// Only per-test references are timed; per-test records are inherently fresh executions
// (a cached package emits no per-test events), so the duration numbers never mix in
// cache-hit noise. Each result also reports how many in-scope runs cached the test's
// package (CachedRuns) so the caller can show how representative the timing is.
func Durations(records map[gotest.Reference][]Record) []DurationResult {
	cachedRuns := cachedRunsByPackage(records)

	results := make([]DurationResult, 0, len(records))
	for ref, recs := range records {
		if ref.IsPackage() {
			continue
		}
		if len(recs) == 0 {
			continue
		}

		// order oldest first so "first" and "latest" are unambiguous
		ordered := append([]Record(nil), recs...)
		sort.Slice(ordered, func(i, j int) bool {
			if !ordered[i].Time.Equal(ordered[j].Time) {
				return ordered[i].Time.Before(ordered[j].Time)
			}
			return ordered[i].RunID.String() < ordered[j].RunID.String() // stable first/latest on ties
		})

		series := make([]float64, len(ordered))
		for i := range ordered {
			series[i] = ordered[i].Elapsed
		}

		first := series[0]
		latest := series[len(series)-1]

		var trendPct float64
		if first != 0 {
			trendPct = (latest - first) / first * 100
		}

		results = append(results, DurationResult{
			Reference:  ref,
			Latest:     latest,
			First:      first,
			TrendPct:   trendPct,
			FreshRuns:  len(ordered),
			CachedRuns: len(cachedRuns[ref.Package]),
			Series:     series,
		})
	}

	// slowest-latest first
	sort.Slice(results, func(i, j int) bool {
		if results[i].Latest != results[j].Latest {
			return results[i].Latest > results[j].Latest
		}
		return results[i].Reference.String(false) < results[j].Reference.String(false)
	})

	return results
}

// cachedRunsByPackage maps each package to the set of runs where it was served from
// cache. A cached package emits a lone "(cached)" package-level event and no per-test
// events, so this is how we know a test existed but wasn't timed in a given run.
func cachedRunsByPackage(records map[gotest.Reference][]Record) map[string]map[uuid.UUID]struct{} {
	byPkg := make(map[string]map[uuid.UUID]struct{})
	for ref, recs := range records {
		if !ref.IsPackage() {
			continue
		}
		for _, rec := range recs {
			if !rec.Cached {
				continue
			}
			set, ok := byPkg[ref.Package]
			if !ok {
				set = make(map[uuid.UUID]struct{})
				byPkg[ref.Package] = set
			}
			set[rec.RunID] = struct{}{}
		}
	}
	return byPkg
}

// CachedRunCount is the number of distinct in-scope runs that cached at least one
// package (and thus contributed no timing data).
func CachedRunCount(records map[gotest.Reference][]Record) int {
	runs := make(map[uuid.UUID]struct{})
	for ref, recs := range records {
		if !ref.IsPackage() {
			continue
		}
		for _, rec := range recs {
			if rec.Cached {
				runs[rec.RunID] = struct{}{}
			}
		}
	}
	return len(runs)
}

// OverallDuration folds per-test results into a single headline trend: the mean of
// each test's earliest vs latest elapsed across the window.
func OverallDuration(results []DurationResult) OverallSummary {
	if len(results) == 0 {
		return OverallSummary{}
	}

	var sumFirst, sumLast float64
	for i := range results {
		sumFirst += results[i].First
		sumLast += results[i].Latest
	}

	n := float64(len(results))
	avgFirst := sumFirst / n
	avgLast := sumLast / n

	var trendPct float64
	if avgFirst != 0 {
		trendPct = (avgLast - avgFirst) / avgFirst * 100
	}

	return OverallSummary{
		AvgFirst: avgFirst,
		AvgLast:  avgLast,
		TrendPct: trendPct,
		Tests:    len(results),
	}
}
