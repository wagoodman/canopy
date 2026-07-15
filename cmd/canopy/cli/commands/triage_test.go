package commands

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/localize"
)

func TestDropRedundantParents(t *testing.T) {
	// refs() extracts the surviving references in order for compact assertions.
	refs := func(fs []runFailure) []string {
		out := make([]string, len(fs))
		for i, f := range fs {
			out[i] = f.ref.String(false)
		}
		return out
	}

	tests := []struct {
		name string
		in   []runFailure
		want []string
	}{
		{
			name: "generic parent aggregate dropped, failing subtests kept",
			in: []runFailure{
				fail("pkg", "TestParent", "fp-parent"), // generic, no location -> pure aggregate
				assertFail("pkg", "TestParent/a", "1", "2", "fp-a"),
				assertFail("pkg", "TestParent/b", "3", "4", "fp-b"),
			},
			want: []string{"pkg/TestParent/a", "pkg/TestParent/b"},
		},
		{
			name: "parent with its own assertion is not an aggregate, kept",
			in: []runFailure{
				assertFail("pkg", "TestParent", "1", "2", "fp-parent"),
				assertFail("pkg", "TestParent/a", "3", "4", "fp-a"),
			},
			want: []string{"pkg/TestParent", "pkg/TestParent/a"},
		},
		{
			name: "lone generic failure with no failing subtests is kept",
			in: []runFailure{
				fail("pkg", "TestSolo", "fp-solo"),
			},
			want: []string{"pkg/TestSolo"},
		},
		{
			name: "nested: intermediate aggregate with a failing descendant dropped",
			in: []runFailure{
				fail("pkg", "TestParent", "fp-parent"), // aggregate over A/B/C
				fail("pkg", "TestParent/A", "fp-a"),    // aggregate over A/B/C
				assertFail("pkg", "TestParent/A/B/C", "1", "2", "fp-c"),
			},
			want: []string{"pkg/TestParent/A/B/C"},
		},
		{
			name: "same func different package is not a descendant",
			in: []runFailure{
				fail("pkg/one", "TestParent", "fp-1"),
				assertFail("pkg/two", "TestParent/a", "1", "2", "fp-2"),
			},
			want: []string{"pkg/one/TestParent", "pkg/two/TestParent/a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, refs(dropRedundantParents(tt.in)))
		})
	}
}

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

func TestFuseGroups_Correlation(t *testing.T) {
	// two symptom groups; a changed symbol reaches every member of group A and the lone member of
	// group B (globally top), plus a low-ranked symbol reached only by group B's member. the fusion
	// must attach the verdicts and the globally-ranked candidates to each group, narrowing each
	// candidate's references to the members it actually reaches.
	clusters := clusterResultJSON{
		Clusters: []clusterJSON{
			{Symptom: "assertion failure", Location: "a_test.go:1", Count: 2,
				References: []string{"pkg/TestA1", "pkg/TestA2"}},
			{Symptom: "assertion: expected 1, got 3", Location: "a_test.go:9", Count: 1,
				References: []string{"pkg/TestB"}},
		},
	}
	verdictByRef := map[string]Verdict{
		"pkg/TestA1": VerdictFlaky,
		"pkg/TestA2": VerdictFlaky,
		"pkg/TestB":  VerdictFlaky,
	}
	loc := &localize.Result{
		Candidates: []localize.Candidate{
			// global top: reaches all three failures across both groups
			{Symbol: "pkg.calc", Location: "pkg/a.go:5", ReachedBy: 3,
				References: []string{"pkg/TestA1", "pkg/TestA2", "pkg/TestB"}},
			// low-ranked tail: only reached by group B's member
			{Symbol: "pkg.other", Location: "pkg/a.go:20", ReachedBy: 1,
				References: []string{"pkg/TestB"}},
		},
	}

	groups := fuseGroups(clusters, verdictByRef, loc, nil)

	require.Len(t, groups, 2)

	// group A: only the global-top candidate reaches it, narrowed to A's members.
	require.Equal(t, []Verdict{VerdictFlaky}, groups[0].verdicts)
	wantA := []localize.Candidate{
		{Symbol: "pkg.calc", Location: "pkg/a.go:5", ReachedBy: 2,
			References: []string{"pkg/TestA1", "pkg/TestA2"}},
	}
	if diff := cmp.Diff(wantA, groups[0].candidates); diff != "" {
		t.Errorf("group A candidates mismatch (-want +got):\n%s", diff)
	}

	// group B: both candidates reach its member, in global rank order (calc leads via global
	// reached_by even though both tie at 1 within the group).
	wantB := []localize.Candidate{
		{Symbol: "pkg.calc", Location: "pkg/a.go:5", ReachedBy: 1, References: []string{"pkg/TestB"}},
		{Symbol: "pkg.other", Location: "pkg/a.go:20", ReachedBy: 1, References: []string{"pkg/TestB"}},
	}
	if diff := cmp.Diff(wantB, groups[1].candidates); diff != "" {
		t.Errorf("group B candidates mismatch (-want +got):\n%s", diff)
	}

	// both symptoms attribute to the one changed symbol → Z=1.
	distinct := distinctTopCauses(groups)
	require.Len(t, distinct, 1)
	require.Equal(t, "pkg.calc", distinct[0].Symbol)
	require.Equal(t, "2 failures across 2 distinct symptoms → 1 root cause: pkg.calc (a.go:5)",
		fusedSummary("2 failures across 2 distinct symptoms", distinct))
}

func TestFuseGroups_NoReachingCandidate(t *testing.T) {
	// a symptom whose members reach no changed symbol gets no root cause, and the summary says so
	// honestly rather than force-ranking against an unrelated diff.
	clusters := clusterResultJSON{
		Clusters: []clusterJSON{
			{Symptom: "panic", Location: "x_test.go:3", Count: 1, References: []string{"pkg/TestX"}},
		},
	}
	loc := &localize.Result{
		Candidates: []localize.Candidate{
			{Symbol: "pkg.calc", Location: "pkg/a.go:5", ReachedBy: 1, References: []string{"pkg/TestOther"}},
		},
	}

	groups := fuseGroups(clusters, map[string]Verdict{"pkg/TestX": VerdictNewRegression}, loc, nil)

	require.Len(t, groups, 1)
	require.Empty(t, groups[0].candidates)
	require.Equal(t, []Verdict{VerdictNewRegression}, groups[0].verdicts)

	distinct := distinctTopCauses(groups)
	require.Empty(t, distinct)
	require.Equal(t, "1 failure across 1 distinct symptom → no root cause in the diff",
		fusedSummary("1 failure across 1 distinct symptom", distinct))
}

func TestFusedSummary_MultipleRootCauses(t *testing.T) {
	distinct := []localize.Candidate{
		{Symbol: "pkg.calc", Location: "pkg/a.go:5"},
		{Symbol: "pkg.gate", Location: "pkg/b.go:9"},
	}
	require.Equal(t, "5 failures across 3 distinct symptoms → 2 root causes",
		fusedSummary("5 failures across 3 distinct symptoms", distinct))
}

func TestSinceText(t *testing.T) {
	tests := []struct {
		name  string
		since *sinceJSON
		want  string
	}{
		{
			name:  "pre-existing exact with changed files",
			since: &sinceJSON{Verdict: VerdictPreExisting, Commit: "abc1234def", ChangedFiles: []string{"pkg/handler/handler.go", "pkg/auth/token.go"}, Confidence: "exact"},
			want:  "since abc1234 (handler.go, token.go)",
		},
		{
			name:  "range folds the good..bad bracket honestly",
			since: &sinceJSON{Verdict: VerdictPreExisting, Commit: "abc1234def", LastGood: "def4567abc", ChangedFiles: []string{"pkg/handler.go"}, Confidence: "range"},
			want:  "since def4567..abc1234 (handler.go)",
		},
		{
			name:  "flaky is just the commit, no files",
			since: &sinceJSON{Verdict: VerdictFlaky, Commit: "abc1234def", LastGood: "def4567abc", Confidence: "exact"},
			want:  "since abc1234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, sinceText(tt.since))
		})
	}
}

func TestSinceForGroup(t *testing.T) {
	sinceByRef := map[string]*sinceJSON{
		"pkg/TestB": {Verdict: VerdictPreExisting, Commit: "abc1234"},
	}
	// the first member with a since represents the group.
	got := sinceForGroup([]string{"pkg/TestA", "pkg/TestB"}, sinceByRef)
	require.NotNil(t, got)
	require.Equal(t, "abc1234", got.Commit)

	// no member has a since -> nil (silently absent).
	require.Nil(t, sinceForGroup([]string{"pkg/TestA"}, sinceByRef))
	require.Nil(t, sinceForGroup([]string{"pkg/TestA"}, nil))
}

func TestDistinctVerdicts_MixedMostActionableFirst(t *testing.T) {
	// a symptom cluster whose members disagree lists distinct verdicts, most-actionable first.
	refs := []string{"pkg/TestA", "pkg/TestB", "pkg/TestC"}
	verdictByRef := map[string]Verdict{
		"pkg/TestA": VerdictFlaky,
		"pkg/TestB": VerdictNewRegression,
		"pkg/TestC": VerdictFlaky,
	}
	require.Equal(t, []Verdict{VerdictNewRegression, VerdictFlaky}, distinctVerdicts(refs, verdictByRef))
}
