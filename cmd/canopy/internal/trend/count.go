package trend

import (
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// CountPoint is the distinct per-test count observed in a single run, forming one
// point in the suite-size-over-time series.
type CountPoint struct {
	RunID        uuid.UUID
	Time         time.Time
	TestCount    int
	PackageCount int // distinct packages the counted tests span in this run
}

// Counts inverts the collected records into a per-run count series: for each run,
// the number of distinct per-test references observed. Points are sorted by Time
// ascending so the slice reads as a timeline.
//
// ponytail: cached packages emit a lone "(cached)" package event and zero per-test
// events, so a cached run has no observable per-test refs and UNDERCOUNTS here. The
// count is only trustworthy on --no-cache runs; upgrade path is folding package-level
// def counts back in once the store records them.
func Counts(records map[gotest.Reference][]Record) []CountPoint {
	// runID -> set of distinct test refs and distinct packages, plus earliest time seen
	seen := make(map[uuid.UUID]map[gotest.Reference]struct{})
	pkgs := make(map[uuid.UUID]map[string]struct{})
	runTime := make(map[uuid.UUID]time.Time)

	for ref, recs := range records {
		if ref.IsPackage() {
			continue
		}
		for _, rec := range recs {
			set, ok := seen[rec.RunID]
			if !ok {
				set = make(map[gotest.Reference]struct{})
				seen[rec.RunID] = set
			}
			set[ref] = struct{}{}

			pset, ok := pkgs[rec.RunID]
			if !ok {
				pset = make(map[string]struct{})
				pkgs[rec.RunID] = pset
			}
			pset[ref.Package] = struct{}{}

			if t, ok := runTime[rec.RunID]; !ok || rec.Time.Before(t) {
				runTime[rec.RunID] = rec.Time
			}
		}
	}

	points := make([]CountPoint, 0, len(seen))
	for runID, set := range seen {
		points = append(points, CountPoint{
			RunID:        runID,
			Time:         runTime[runID],
			TestCount:    len(set),
			PackageCount: len(pkgs[runID]),
		})
	}

	sort.Slice(points, func(i, j int) bool {
		if !points[i].Time.Equal(points[j].Time) {
			return points[i].Time.Before(points[j].Time)
		}
		return points[i].RunID.String() < points[j].RunID.String() // stable order on ties
	})
	return points
}

// CountDelta summarizes the change in suite size across a count series.
type CountDelta struct {
	First int // count at the earliest point (0 if no points)
	Last  int // count at the latest point (0 if no points)
	Runs  int // number of points in the series
}

// Delta reduces a count series to its first-vs-last summary.
func Delta(points []CountPoint) CountDelta {
	d := CountDelta{Runs: len(points)}
	if len(points) == 0 {
		return d
	}
	d.First = points[0].TestCount
	d.Last = points[len(points)-1].TestCount
	return d
}

// Change is the signed difference between the last and first counts.
func (d CountDelta) Change() int {
	return d.Last - d.First
}

// DistinctPackages counts the packages with at least one per-test record across the
// whole collected set. This is the breadth the count series spans, which matters
// because per-run counts are only comparable when scoped to a stable set of packages.
func DistinctPackages(records map[gotest.Reference][]Record) int {
	pkgs := make(map[string]struct{})
	for ref := range records {
		if ref.IsPackage() {
			continue
		}
		pkgs[ref.Package] = struct{}{}
	}
	return len(pkgs)
}
