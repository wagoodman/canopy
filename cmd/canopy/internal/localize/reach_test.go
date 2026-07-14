package localize

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/go-set/strset"
	"github.com/stretchr/testify/require"
)

// synthetic graph mirroring the plan's fixture: TestA and TestB both reach a changed helper;
// TestC reaches only an unchanged path. Nodes are named by their id; "helper" is the one changed
// symbol.
func fixtureGraph() (edges map[string][]string, symbolByNode map[string]string, symbols map[string]symbolInfo) {
	edges = map[string][]string{
		"TestA":     {"helper"},
		"TestB":     {"mid"},
		"mid":       {"helper"},
		"TestC":     {"unchanged"},
		"helper":    {"leaf"},
		"leaf":      {},
		"unchanged": {},
	}
	symbolByNode = map[string]string{"helper": "sym-helper"}
	symbols = map[string]symbolInfo{"sym-helper": {display: "pkg.helper", location: "pkg/helper.go:10"}}
	return
}

func TestInvertReachability(t *testing.T) {
	edges, symbolByNode, _ := fixtureGraph()

	got := invertReachability([]string{"TestA", "TestB", "TestC"}, edges, symbolByNode)

	require.True(t, got["TestA"].Has("sym-helper"))
	require.True(t, got["TestB"].Has("sym-helper")) // reached transitively through mid
	require.Equal(t, 0, got["TestC"].Size())        // only reaches an unchanged path
}

func TestRankCandidates_DistinctTestsReaching(t *testing.T) {
	_, _, symbols := fixtureGraph()
	edges, symbolByNode, _ := fixtureGraph()

	failures := []Failure{
		{Reference: "pkg/TestA", RootNode: "TestA"},
		{Reference: "pkg/TestB", RootNode: "TestB"},
		{Reference: "pkg/TestC", RootNode: "TestC"},
	}
	reach := invertReachability([]string{"TestA", "TestB", "TestC"}, edges, symbolByNode)

	got := rankCandidates(failures, reach, symbols, "cha")

	want := Result{
		Candidates: []Candidate{
			{Symbol: "pkg.helper", Location: "pkg/helper.go:10", ReachedBy: 2,
				References: []string{"pkg/TestA", "pkg/TestB"}},
		},
		Unattributed: []Unattributed{
			{Reference: "pkg/TestC", Note: noRootCauseNote},
		},
		CallGraph: "cha",
		Summary:   "2 of 3 failures attributed to 1 changed symbol; 1 unattributed",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("rankCandidates() mismatch (-want +got):\n%s", diff)
	}
}

func TestRankCandidates_UnresolvedRootIsUnattributed(t *testing.T) {
	_, _, symbols := fixtureGraph()

	// a failure whose entrypoint never resolved (RootNode "") must land in unattributed, never
	// force-ranked against an unrelated diff.
	failures := []Failure{{Reference: "pkg/TestGhost", RootNode: ""}}
	got := rankCandidates(failures, map[string]*strset.Set{}, symbols, "cha")

	require.Empty(t, got.Candidates)
	require.Len(t, got.Unattributed, 1)
	require.Equal(t, "pkg/TestGhost", got.Unattributed[0].Reference)
	require.Equal(t, "0 of 1 failure attributed to 0 changed symbols; 1 unattributed", got.Summary)
}
