package blame

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// run is a compact synthetic run builder for the history tables below.
func run(commit string, passed bool, fingerprint string) RunPoint {
	return RunPoint{Commit: commit, Branch: "main", Passed: passed, Fingerprint: fingerprint}
}

// adjacentDist treats commits whose names differ by one (c1->c2) as adjacent (distance 0) and
// anything else as a gap sized by the numeric delta, so the tables can exercise exact vs range
// without a real git graph. commits are named "c<N>".
func adjacentDist(good, bad string) int {
	g, b := commitNum(good), commitNum(bad)
	if g < 0 || b < 0 {
		return -1
	}
	return b - g - 1 // commits strictly between; 0 when adjacent
}

func commitNum(c string) int {
	if len(c) < 2 || c[0] != 'c' {
		return -1
	}
	n := 0
	for _, r := range c[1:] {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func TestDetectPreExisting(t *testing.T) {
	const fp = "boom"
	tests := []struct {
		name    string
		history []RunPoint
		dist    CommitDistanceFunc
		want    *Since
	}{
		{
			name:    "exact adjacent good to bad",
			history: []RunPoint{run("c1", true, ""), run("c2", false, fp)},
			dist:    adjacentDist,
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceExact},
		},
		{
			name:    "gap between good and bad is a range",
			history: []RunPoint{run("c1", true, ""), run("c5", false, fp)},
			dist:    adjacentDist,
			want:    &Since{Commit: "c5", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceRange},
		},
		{
			name:    "never seen good is unknown",
			history: []RunPoint{run("c3", false, fp), run("c4", false, fp)},
			dist:    adjacentDist,
			want:    &Since{Commit: "c3", Branch: "main", Confidence: ConfidenceUnknown},
		},
		{
			name:    "prior failure with a different fingerprint bounds the onset",
			history: []RunPoint{run("c1", false, "other"), run("c2", false, fp)},
			dist:    adjacentDist,
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceExact},
		},
		{
			name:    "unknown commit distance degrades to range",
			history: []RunPoint{run("c1", true, ""), run("c2", false, fp)},
			dist:    func(_, _ string) int { return -1 },
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceRange},
		},
		{
			name:    "nil distance func is honest as range",
			history: []RunPoint{run("c1", true, ""), run("c2", false, fp)},
			dist:    nil,
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceRange},
		},
		{
			name:    "fingerprint never appears yields no annotation",
			history: []RunPoint{run("c1", true, ""), run("c2", false, "other")},
			dist:    adjacentDist,
			want:    nil,
		},
		{
			// passed and failed on the same commit: environmental flake, not an exact culprit
			name:    "same commit good and bad is a range not exact",
			history: []RunPoint{run("c2", true, ""), run("c2", false, fp)},
			dist:    adjacentDist,
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c2", Confidence: ConfidenceRange},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPreExisting(tt.history, fp, tt.dist)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("DetectPreExisting mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDetectFlaky(t *testing.T) {
	tests := []struct {
		name    string
		history []RunPoint
		dist    CommitDistanceFunc
		want    *Since
	}{
		{
			name:    "onset at first fail after a pass",
			history: []RunPoint{run("c1", true, ""), run("c2", false, "boom"), run("c3", true, "")},
			dist:    adjacentDist,
			want:    &Since{Commit: "c2", Branch: "main", LastGoodCommit: "c1", Confidence: ConfidenceExact},
		},
		{
			name:    "onset across a commit gap is a range",
			history: []RunPoint{run("c1", true, ""), run("c2", true, ""), run("c6", false, "boom")},
			dist:    adjacentDist,
			want:    &Since{Commit: "c6", Branch: "main", LastGoodCommit: "c2", Confidence: ConfidenceRange},
		},
		{
			name:    "all failures never flip so no onset",
			history: []RunPoint{run("c1", false, "boom"), run("c2", false, "boom")},
			dist:    adjacentDist,
			want:    nil,
		},
		{
			name:    "only passes so no onset",
			history: []RunPoint{run("c1", true, ""), run("c2", true, "")},
			dist:    adjacentDist,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFlaky(tt.history, tt.dist)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("DetectFlaky mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
