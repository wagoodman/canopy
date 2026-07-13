package trend

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestFailureRates(t *testing.T) {
	pkg := "example.com/foo"

	// helper to build a chronological run of records for one test
	rec := func(action gotest.Action, minute int) Record {
		return Record{
			RunID:  uuid.New(),
			Time:   time.Date(2026, 1, 1, 0, minute, 0, 0, time.UTC),
			Action: action,
		}
	}

	tests := []struct {
		name    string
		records map[gotest.Reference][]Record
		want    []FailureResult
	}{
		{
			name: "excludes package refs and never-failing tests",
			records: map[gotest.Reference][]Record{
				gotest.NewReference(pkg, ""): {
					rec(gotest.FailAction, 0), // package-level fail, must be skipped
				},
				gotest.NewReference(pkg, "TestAlwaysPass"): {
					rec(gotest.PassAction, 0),
					rec(gotest.PassAction, 1),
				},
			},
			want: nil,
		},
		{
			name: "computes fail rate excluding skips from denominator",
			records: map[gotest.Reference][]Record{
				gotest.NewReference(pkg, "TestMixed"): {
					rec(gotest.PassAction, 0),
					rec(gotest.FailAction, 1),
					rec(gotest.FailAction, 2),
					rec(gotest.SkipAction, 3),
				},
			},
			want: []FailureResult{
				{
					Reference: gotest.NewReference(pkg, "TestMixed"),
					Runs:      4,
					PassCount: 1,
					FailCount: 2,
					SkipCount: 1,
					FailRate:  2.0 / 3.0,
					Trend:     FailureTrendFlat, // only 3 non-skip points, under the 4-point threshold
					Series:    []bool{false, true, true},
				},
			},
		},
		{
			name: "trend up when failures cluster in the later half",
			records: map[gotest.Reference][]Record{
				gotest.NewReference(pkg, "TestRegressing"): {
					rec(gotest.PassAction, 0),
					rec(gotest.PassAction, 1),
					rec(gotest.FailAction, 2),
					rec(gotest.FailAction, 3),
				},
			},
			want: []FailureResult{
				{
					Reference: gotest.NewReference(pkg, "TestRegressing"),
					Runs:      4,
					PassCount: 2,
					FailCount: 2,
					SkipCount: 0,
					FailRate:  0.5,
					Trend:     FailureTrendUp,
					Series:    []bool{false, false, true, true},
				},
			},
		},
		{
			name: "sorted by fail rate desc",
			records: map[gotest.Reference][]Record{
				gotest.NewReference(pkg, "TestLow"): {
					rec(gotest.PassAction, 0),
					rec(gotest.FailAction, 1),
				},
				gotest.NewReference(pkg, "TestHigh"): {
					rec(gotest.FailAction, 0),
					rec(gotest.FailAction, 1),
				},
			},
			want: []FailureResult{
				{
					Reference: gotest.NewReference(pkg, "TestHigh"),
					Runs:      2,
					PassCount: 0,
					FailCount: 2,
					SkipCount: 0,
					FailRate:  1.0,
					Trend:     FailureTrendFlat,
					Series:    []bool{true, true},
				},
				{
					Reference: gotest.NewReference(pkg, "TestLow"),
					Runs:      2,
					PassCount: 1,
					FailCount: 1,
					SkipCount: 0,
					FailRate:  0.5,
					Trend:     FailureTrendFlat,
					Series:    []bool{false, true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FailureRates(tt.records)
			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}
