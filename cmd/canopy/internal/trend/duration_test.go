package trend

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestDurations(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	slow := gotest.Reference{Package: "pkg", FuncName: "TestSlow"}
	fast := gotest.Reference{Package: "pkg", FuncName: "TestFast"}
	pkg := gotest.Reference{Package: "pkg"}

	run1 := uuid.New()

	tests := []struct {
		name    string
		records map[gotest.Reference][]Record
		want    []DurationResult
	}{
		{
			name: "trend and ordering",
			records: map[gotest.Reference][]Record{
				// out of time order on purpose to prove sorting
				slow: {
					{Time: base.Add(2 * time.Hour), Action: gotest.PassAction, Elapsed: 10},
					{Time: base, Action: gotest.PassAction, Elapsed: 8},
				},
				fast: {
					{Time: base, Action: gotest.PassAction, Elapsed: 2},
					{Time: base.Add(time.Hour), Action: gotest.PassAction, Elapsed: 1},
				},
				// package refs are ignored for timing
				pkg: {
					{Time: base, Action: gotest.PassAction, Elapsed: 99},
				},
			},
			want: []DurationResult{
				{Reference: slow, Latest: 10, First: 8, TrendPct: 25, FreshRuns: 2, Series: []float64{8, 10}},
				{Reference: fast, Latest: 1, First: 2, TrendPct: -50, FreshRuns: 2, Series: []float64{2, 1}},
			},
		},
		{
			name: "zero first guards divide-by-zero",
			records: map[gotest.Reference][]Record{
				fast: {
					{Time: base, Action: gotest.PassAction, Elapsed: 0},
					{Time: base.Add(time.Hour), Action: gotest.PassAction, Elapsed: 5},
				},
			},
			want: []DurationResult{
				{Reference: fast, Latest: 5, First: 0, TrendPct: 0, FreshRuns: 1 + 1, Series: []float64{0, 5}},
			},
		},
		{
			name: "cached package run is counted, not timed",
			records: map[gotest.Reference][]Record{
				fast: {
					{Time: base, Action: gotest.PassAction, Elapsed: 3},
				},
				// same package cached in another run: no per-test record, just the pkg event
				pkg: {
					{RunID: run1, Time: base.Add(time.Hour), Action: gotest.PassAction, Cached: true},
				},
			},
			want: []DurationResult{
				{Reference: fast, Latest: 3, First: 3, TrendPct: 0, FreshRuns: 1, CachedRuns: 1, Series: []float64{3}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Durations(tt.records)
			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOverallDuration(t *testing.T) {
	results := []DurationResult{
		{First: 8, Latest: 10},
		{First: 2, Latest: 1},
	}
	got := OverallDuration(results)
	assert.Equal(t, 5.0, got.AvgFirst)
	assert.Equal(t, 5.5, got.AvgLast)
	assert.InDelta(t, 10.0, got.TrendPct, 0.0001)
	assert.Equal(t, 2, got.Tests)

	// empty is safe
	assert.Equal(t, OverallSummary{}, OverallDuration(nil))
}
