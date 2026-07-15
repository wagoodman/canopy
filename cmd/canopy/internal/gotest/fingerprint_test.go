package gotest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCaptureReproEnv_AllowlistOnly(t *testing.T) {
	// a secret-looking var that is NOT allowlisted and NOT named must never be captured.
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "super-secret")
	t.Setenv("MY_FEATURE_FLAG", "on")

	tests := []struct {
		name  string
		extra []string
		want  map[string]string
	}{
		{
			name:  "only the built-in allowlist is captured, secrets excluded",
			extra: nil,
			want:  map[string]string{"GOFLAGS": "-mod=mod", "CGO_ENABLED": "0"},
		},
		{
			name:  "user-named key is added, still no unnamed secret",
			extra: []string{"MY_FEATURE_FLAG"},
			want:  map[string]string{"GOFLAGS": "-mod=mod", "CGO_ENABLED": "0", "MY_FEATURE_FLAG": "on"},
		},
		{
			name:  "naming a set secret opts it in explicitly (the documented upgrade path)",
			extra: []string{"AWS_SECRET_ACCESS_KEY"},
			want:  map[string]string{"GOFLAGS": "-mod=mod", "CGO_ENABLED": "0", "AWS_SECRET_ACCESS_KEY": "super-secret"},
		},
		{
			name:  "empty and blank names are ignored",
			extra: []string{"", "  "},
			want:  map[string]string{"GOFLAGS": "-mod=mod", "CGO_ENABLED": "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CaptureReproEnv(tt.extra)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CaptureReproEnv() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCaptureReproEnv_UnsetOmitted(t *testing.T) {
	// unset allowlisted vars are omitted (an absent var is a non-default the repro must not fabricate).
	// clear the allowlist entries for this test so nothing leaks in from the caller's environment.
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "")
	// t.Setenv sets to empty string (still "set"); to prove omission we name a definitely-unset key.
	if got := CaptureReproEnv([]string{"CANOPY_DEFINITELY_UNSET_XYZ"}); got != nil {
		if _, ok := got["CANOPY_DEFINITELY_UNSET_XYZ"]; ok {
			t.Errorf("CaptureReproEnv() captured an unset var: %v", got)
		}
	}
}
