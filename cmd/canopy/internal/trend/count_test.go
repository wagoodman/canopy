package trend

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestCounts(t *testing.T) {
	run1 := uuid.New()
	run2 := uuid.New()

	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)

	testA := gotest.Reference{Package: "pkg", FuncName: "TestA"}
	testB := gotest.Reference{Package: "pkg", FuncName: "TestB"}
	testC := gotest.Reference{Package: "pkg", FuncName: "TestC"}
	testD := gotest.Reference{Package: "pkg2", FuncName: "TestD"}
	pkgRef := gotest.Reference{Package: "pkg"} // package-level, must be skipped

	tests := []struct {
		name    string
		records map[gotest.Reference][]Record
		want    []CountPoint
	}{
		{
			name:    "empty",
			records: map[gotest.Reference][]Record{},
			want:    []CountPoint{},
		},
		{
			name: "package refs are skipped",
			records: map[gotest.Reference][]Record{
				pkgRef: {{RunID: run1, Time: t0, Action: gotest.PassAction, Cached: true}},
				testA:  {{RunID: run1, Time: t0, Action: gotest.PassAction}},
			},
			want: []CountPoint{
				{RunID: run1, Time: t0, TestCount: 1, PackageCount: 1},
			},
		},
		{
			name: "package count reflects distinct packages per run",
			records: map[gotest.Reference][]Record{
				testA: {{RunID: run1, Time: t0, Action: gotest.PassAction}},
				testD: {{RunID: run1, Time: t0.Add(time.Minute), Action: gotest.PassAction}},
			},
			want: []CountPoint{
				{RunID: run1, Time: t0, TestCount: 2, PackageCount: 2},
			},
		},
		{
			name: "distinct tests counted per run, sorted by time, run time is min",
			records: map[gotest.Reference][]Record{
				// run1 (earlier): A, B
				testA: {
					{RunID: run1, Time: t0.Add(time.Minute), Action: gotest.PassAction},
					// duplicate observation of same ref in same run counts once
					{RunID: run1, Time: t0, Action: gotest.FailAction},
					// also present in run2
					{RunID: run2, Time: t1, Action: gotest.PassAction},
				},
				testB: {
					{RunID: run1, Time: t0.Add(2 * time.Minute), Action: gotest.PassAction},
				},
				// run2 (later): A, C
				testC: {
					{RunID: run2, Time: t1.Add(time.Minute), Action: gotest.SkipAction},
				},
			},
			want: []CountPoint{
				{RunID: run1, Time: t0, TestCount: 2, PackageCount: 1},
				{RunID: run2, Time: t1, TestCount: 2, PackageCount: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Counts(tt.records)
			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDistinctPackages(t *testing.T) {
	run1 := uuid.New()
	testA := gotest.Reference{Package: "pkg", FuncName: "TestA"}
	testB := gotest.Reference{Package: "pkg", FuncName: "TestB"}
	testD := gotest.Reference{Package: "pkg2", FuncName: "TestD"}
	pkgRef := gotest.Reference{Package: "pkg3"} // package-level, must not count

	records := map[gotest.Reference][]Record{
		testA:  {{RunID: run1, Action: gotest.PassAction}},
		testB:  {{RunID: run1, Action: gotest.PassAction}},
		testD:  {{RunID: run1, Action: gotest.PassAction}},
		pkgRef: {{RunID: run1, Action: gotest.PassAction}},
	}

	// pkg + pkg2 = 2; the package-level ref in pkg3 is skipped
	assert.Equal(t, 2, DistinctPackages(records))
	assert.Equal(t, 0, DistinctPackages(map[gotest.Reference][]Record{}))
}

func TestDelta(t *testing.T) {
	tests := []struct {
		name       string
		points     []CountPoint
		want       CountDelta
		wantChange int
	}{
		{
			name:       "empty",
			points:     nil,
			want:       CountDelta{},
			wantChange: 0,
		},
		{
			name: "growth",
			points: []CountPoint{
				{TestCount: 42},
				{TestCount: 45},
				{TestCount: 47},
			},
			want:       CountDelta{First: 42, Last: 47, Runs: 3},
			wantChange: 5,
		},
		{
			name: "shrink",
			points: []CountPoint{
				{TestCount: 10},
				{TestCount: 7},
			},
			want:       CountDelta{First: 10, Last: 7, Runs: 2},
			wantChange: -3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Delta(tt.points)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantChange, got.Change())
		})
	}
}
