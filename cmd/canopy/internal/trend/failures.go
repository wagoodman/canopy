package trend

import (
	"sort"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// FailureTrend classifies whether a test's failure rate is getting worse, better, or holding steady.
type FailureTrend string

const (
	// FailureTrendUp means the test failed more often in the later half of the window (regression signal).
	FailureTrendUp FailureTrend = "up"
	// FailureTrendDown means the test failed less often in the later half (fixes landing).
	FailureTrendDown FailureTrend = "down"
	// FailureTrendFlat means no meaningful change between halves.
	FailureTrendFlat FailureTrend = "flat"
)

// FailureResult is the per-test failure summary produced by FailureRates.
type FailureResult struct {
	Reference gotest.Reference
	Runs      int // total terminal observations (pass + fail + skip)
	PassCount int
	FailCount int
	SkipCount int
	FailRate  float64 // fails / (pass + fail); skips excluded from the denominator
	Trend     FailureTrend
	Series    []bool // chronological outcomes, true = fail (skips omitted)
}

// FailureRates summarizes per-test failure behavior from already-collected records.
//
// Only per-test references are considered; package refs are skipped. A test is included
// only if it failed at least once, since a never-failing test is not interesting for a
// failures report. Results are sorted by fail rate desc, then reference string for stable ties.
func FailureRates(records map[gotest.Reference][]Record) []FailureResult {
	var results []FailureResult

	for ref, recs := range records {
		if ref.IsPackage() {
			continue
		}

		// chronological order so the trend heuristic and series are time-ordered
		ordered := make([]Record, len(recs))
		copy(ordered, recs)
		sort.Slice(ordered, func(i, j int) bool {
			return ordered[i].Time.Before(ordered[j].Time)
		})

		var pass, fail, skip int
		series := make([]bool, 0, len(ordered))
		for _, r := range ordered {
			switch r.Action {
			case gotest.PassAction:
				pass++
				series = append(series, false)
			case gotest.FailAction:
				fail++
				series = append(series, true)
			case gotest.SkipAction:
				skip++
				// skips are not part of the pass/fail series or denominator
			}
		}

		// never-failed tests are not interesting for a failures report
		if fail == 0 {
			continue
		}

		var rate float64
		if denom := pass + fail; denom > 0 {
			rate = float64(fail) / float64(denom)
		}

		results = append(results, FailureResult{
			Reference: ref,
			Runs:      pass + fail + skip,
			PassCount: pass,
			FailCount: fail,
			SkipCount: skip,
			FailRate:  rate,
			Trend:     failureTrend(series),
			Series:    series,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].FailRate != results[j].FailRate {
			return results[i].FailRate > results[j].FailRate
		}
		return results[i].Reference.String(false) < results[j].Reference.String(false)
	})

	return results
}

// failureTrend compares the fail rate of the earlier half of the series against the later
// half. more failures late = "up" (regression), fewer = "down" (fixes landing).
//
// ponytail: dead-simple first-half/second-half split, no windowing or smoothing. under 4
// data points there isn't enough signal so we call it flat. upgrade path: a proper slope
// (linear fit) if the halves prove too coarse in practice.
func failureTrend(series []bool) FailureTrend {
	if len(series) < 4 {
		return FailureTrendFlat
	}

	mid := len(series) / 2
	earlyRate := failRateOf(series[:mid])
	lateRate := failRateOf(series[mid:])

	const epsilon = 0.001
	switch {
	case lateRate > earlyRate+epsilon:
		return FailureTrendUp
	case lateRate < earlyRate-epsilon:
		return FailureTrendDown
	default:
		return FailureTrendFlat
	}
}

// failRateOf is the fraction of true (fail) entries in a slice.
func failRateOf(s []bool) float64 {
	if len(s) == 0 {
		return 0
	}
	var fails int
	for _, f := range s {
		if f {
			fails++
		}
	}
	return float64(fails) / float64(len(s))
}
