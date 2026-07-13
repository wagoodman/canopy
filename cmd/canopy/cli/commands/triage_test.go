package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
)

func TestDeriveVerdict(t *testing.T) {
	tests := []struct {
		name        string
		analysis    *flaky.Analysis
		fingerprint string
		priorPrints map[string]bool
		want        Verdict
	}{
		{
			// flaky dominates even when the failure fingerprint is brand new
			name:        "flaky dominates over new",
			analysis:    &flaky.Analysis{PassCount: 3, FailCount: 2},
			fingerprint: "newfp",
			priorPrints: map[string]bool{},
			want:        VerdictFlaky,
		},
		{
			// flaky dominates even when the fingerprint was seen before
			name:        "flaky dominates over pre-existing",
			analysis:    &flaky.Analysis{PassCount: 1, FailCount: 1},
			fingerprint: "seenfp",
			priorPrints: map[string]bool{"seenfp": true},
			want:        VerdictFlaky,
		},
		{
			name:        "pre-existing when fingerprint seen earlier and not flaky",
			analysis:    &flaky.Analysis{PassCount: 0, FailCount: 4},
			fingerprint: "seenfp",
			priorPrints: map[string]bool{"seenfp": true},
			want:        VerdictPreExisting,
		},
		{
			name:        "new-regression when fingerprint unseen and not flaky",
			analysis:    &flaky.Analysis{PassCount: 5, FailCount: 0},
			fingerprint: "newfp",
			priorPrints: map[string]bool{},
			want:        VerdictNewRegression,
		},
		{
			// boundary: only failures (no passes) is NOT flaky, so a seen fingerprint
			// stays pre-existing rather than flipping to flaky
			name:        "boundary all-fail is not flaky",
			analysis:    &flaky.Analysis{PassCount: 0, FailCount: 1},
			fingerprint: "seenfp",
			priorPrints: map[string]bool{"seenfp": true},
			want:        VerdictPreExisting,
		},
		{
			// boundary: one pass added to the all-fail case flips it to flaky
			name:        "boundary one pass flips to flaky",
			analysis:    &flaky.Analysis{PassCount: 1, FailCount: 1},
			fingerprint: "seenfp",
			priorPrints: map[string]bool{"seenfp": true},
			want:        VerdictFlaky,
		},
		{
			name:        "nil analysis with unseen fingerprint is new-regression",
			analysis:    nil,
			fingerprint: "newfp",
			priorPrints: map[string]bool{},
			want:        VerdictNewRegression,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveVerdict(tt.analysis, tt.fingerprint, tt.priorPrints)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestTriageBreakdown(t *testing.T) {
	// counts are [new-regression, pre-existing, flaky, unused]
	tests := []struct {
		name   string
		total  int
		counts [4]int
		want   string
	}{
		{"single category collapses to all", 55, [4]int{0, 55, 0, 0}, "55 failures (all pre-existing)"},
		{"mixed lists only non-zero", 56, [4]int{0, 55, 1, 0}, "56 failures (55 pre-existing, 1 flaky)"},
		{"all three categories", 6, [4]int{1, 2, 3, 0}, "6 failures (1 new-regression, 2 pre-existing, 3 flaky)"},
		{"singular noun", 1, [4]int{1, 0, 0, 0}, "1 failure (all new-regression)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, triageBreakdown(tt.total, tt.counts))
		})
	}
}
