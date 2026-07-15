package localize

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/ssa"
)

const callGraphCHA = "cha"

// chaResolver is the sound-but-imprecise baseline the RTA default improves on. It lives in the
// test because production only ships the RTA path; here it drives the interface-dispatch contrast
// (CHA ties every candidate, RTA distinguishes the culprit).
func chaResolver(prog *ssa.Program, _ []*ssa.Function) (*callgraph.Graph, string) {
	return cha.CallGraph(prog), callGraphCHA
}

// flakyPkg is the in-tree fixture package. analyzer.go carries the intentional one-character bug
// in calculateFlakyScore (4*p*(1+p) instead of 4*p*(1-p)) that fails TestCalculateFlakyScore and
// TestAnalyzer_WindowFilter — the end-to-end localization demo.
const flakyPkg = "github.com/wagoodman/canopy/cmd/canopy/internal/flaky"

func fixtureFailures() []gotest.Reference {
	return []gotest.Reference{
		{Package: flakyPkg, FuncName: "TestCalculateFlakyScore", TRunName: "50/50"},
		{Package: flakyPkg, FuncName: "TestCalculateFlakyScore", TRunName: "75/25"},
		{Package: flakyPkg, FuncName: "TestCalculateFlakyScore", TRunName: "90/10"},
		{Package: flakyPkg, FuncName: "TestAnalyzer_WindowFilter"},
	}
}

// TestLocalize_UnresolvableRootDegrades guards the nil-root crash: when a failing test's package is
// not in the loaded/scoped set, no SSA entrypoint resolves, so rta.Analyze gets empty roots and
// returns a nil call graph. localize must degrade to an all-unattributed result instead of
// dereferencing that nil. Panics before the rtaResolver/indexGraph nil-guards.
func TestLocalize_UnresolvableRootDegrades(t *testing.T) {
	if testing.Short() {
		t.Skip("loads real packages and builds SSA; skipped in -short")
	}

	abs, err := filepath.Abs("../flaky/analyzer.go")
	require.NoError(t, err)
	changed, err := ChangedSymbols([]string{abs})
	require.NoError(t, err)
	require.NotEmpty(t, changed)

	// the failure lives in a package flakyPkg does not import, so its entrypoint is absent from the
	// loaded set and no root resolves.
	failures := []gotest.Reference{
		{Package: "github.com/wagoodman/canopy/internal/test-fixtures/simple", FuncName: "TestFail"},
	}

	res, err := localizeWith(rtaResolver, []string{flakyPkg}, changed, failures)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Empty(t, res.Candidates)
	require.Len(t, res.Unattributed, 1)
	require.Equal(t, failures[0].String(false), res.Unattributed[0].Reference)
}

// TestLocalize_FixtureRTARanksRootCause is the end-to-end guard for the demo and for the CHA->RTA
// upgrade (plan verification item 3). Over the real fixture: RTA ranks calculateFlakyScore as the
// UNIQUE top candidate reaching every failure, whereas CHA over-attributes so badly (every test
// reaches every method through testify/t.Run dispatch) that its top score is a tie — proving the
// resolver upgrade does something concrete.
func TestLocalize_FixtureRTARanksRootCause(t *testing.T) {
	if testing.Short() {
		t.Skip("loads real packages and builds SSA; skipped in -short")
	}

	abs, err := filepath.Abs("../flaky/analyzer.go")
	require.NoError(t, err)
	changed, err := ChangedSymbols([]string{abs})
	require.NoError(t, err)
	require.NotEmpty(t, changed)

	failures := fixtureFailures()

	// RTA: calculateFlakyScore is the sole top candidate, reaching all four failures.
	rtaRes, err := localizeWith(rtaResolver, []string{flakyPkg}, changed, failures)
	require.NoError(t, err)
	require.NotNil(t, rtaRes)
	require.Equal(t, callGraphRTA, rtaRes.CallGraph)
	require.NotEmpty(t, rtaRes.Candidates)

	top := rtaRes.Candidates[0]
	require.Contains(t, top.Symbol, "calculateFlakyScore")
	require.Equal(t, len(failures), top.ReachedBy, "the root cause should be reached by every failure")
	require.Empty(t, rtaRes.Unattributed)
	// the top score is a STRICT maximum: RTA distinguishes the culprit from the rest.
	if len(rtaRes.Candidates) > 1 {
		require.Greater(t, top.ReachedBy, rtaRes.Candidates[1].ReachedBy,
			"RTA should rank the culprit strictly above the rest, not tie")
	}

	// CHA: the same query ties the top score across candidates (no unique culprit), which is the
	// imprecision RTA fixes.
	chaRes, err := localizeWith(chaResolver, []string{flakyPkg}, changed, failures)
	require.NoError(t, err)
	require.Equal(t, callGraphCHA, chaRes.CallGraph)
	require.Greater(t, len(chaRes.Candidates), 1)
	require.Equal(t, chaRes.Candidates[0].ReachedBy, chaRes.Candidates[1].ReachedBy,
		"CHA over-attributes: its top score should be tied, showing why the RTA upgrade matters")
}
