package flaky

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnalysis_IsFlaky(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
		failCount int
		want      bool
	}{
		{
			name:      "both pass and fail counts",
			passCount: 5,
			failCount: 3,
			want:      true,
		},
		{
			name:      "only passes",
			passCount: 10,
			failCount: 0,
			want:      false,
		},
		{
			name:      "only fails",
			passCount: 0,
			failCount: 10,
			want:      false,
		},
		{
			name:      "no runs",
			passCount: 0,
			failCount: 0,
			want:      false,
		},
		{
			name:      "one of each",
			passCount: 1,
			failCount: 1,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analysis{
				PassCount: tt.passCount,
				FailCount: tt.failCount,
			}
			require.Equal(t, tt.want, a.IsFlaky())
		})
	}
}

func TestAnalysis_PassRate(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
		failCount int
		want      float64
	}{
		{
			name:      "all passes",
			passCount: 10,
			failCount: 0,
			want:      1.0,
		},
		{
			name:      "all fails",
			passCount: 0,
			failCount: 10,
			want:      0.0,
		},
		{
			name:      "half and half",
			passCount: 5,
			failCount: 5,
			want:      0.5,
		},
		{
			name:      "75% pass rate",
			passCount: 3,
			failCount: 1,
			want:      0.75,
		},
		{
			name:      "no runs",
			passCount: 0,
			failCount: 0,
			want:      0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analysis{
				PassCount: tt.passCount,
				FailCount: tt.failCount,
			}
			require.InDelta(t, tt.want, a.PassRate(), 0.001)
		})
	}
}

func TestAnalysis_FailRate(t *testing.T) {
	tests := []struct {
		name      string
		passCount int
		failCount int
		want      float64
	}{
		{
			name:      "all fails",
			passCount: 0,
			failCount: 10,
			want:      1.0,
		},
		{
			name:      "all passes",
			passCount: 10,
			failCount: 0,
			want:      0.0,
		},
		{
			name:      "half and half",
			passCount: 5,
			failCount: 5,
			want:      0.5,
		},
		{
			name:      "25% fail rate",
			passCount: 3,
			failCount: 1,
			want:      0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analysis{
				PassCount: tt.passCount,
				FailCount: tt.failCount,
			}
			require.InDelta(t, tt.want, a.FailRate(), 0.001)
		})
	}
}

func TestAnalysis_HasDistinctFailures(t *testing.T) {
	tests := []struct {
		name         string
		failureModes []FailureMode
		want         bool
	}{
		{
			name:         "no failures",
			failureModes: nil,
			want:         false,
		},
		{
			name:         "one failure type",
			failureModes: []FailureMode{{Fingerprint: "abc123"}},
			want:         false,
		},
		{
			name:         "two failure types",
			failureModes: []FailureMode{{Fingerprint: "abc123"}, {Fingerprint: "def456"}},
			want:         true,
		},
		{
			name:         "multiple failure types",
			failureModes: []FailureMode{{Fingerprint: "abc"}, {Fingerprint: "def"}, {Fingerprint: "ghi"}},
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Analysis{
				FailureModes: tt.failureModes,
			}
			require.Equal(t, tt.want, a.HasDistinctFailures())
		})
	}
}

func TestOutcome(t *testing.T) {
	now := time.Now()

	outcome := Outcome{
		Action: "pass",
		Time:   now,
	}

	require.Equal(t, "pass", string(outcome.Action))
	require.Equal(t, now, outcome.Time)
	require.Nil(t, outcome.Failure)

	// test with failure info
	outcomeWithFailure := Outcome{
		Action: "fail",
		Time:   now,
		Failure: &FailureInfo{
			Fingerprint: "abc123",
			Type:        "assertion",
		},
	}
	require.NotNil(t, outcomeWithFailure.Failure)
	require.Equal(t, "abc123", outcomeWithFailure.Failure.Fingerprint)
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	require.Equal(t, time.Duration(0), cfg.Window)
	require.Equal(t, 2, cfg.MinRuns)
	require.Equal(t, 0.0, cfg.Threshold)
}
