package commands

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scylladb/go-set/strset"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// fail builds a runFailure for a package/test with the given fingerprint.
func fail(pkg, test, fingerprint string) runFailure {
	return runFailure{
		ref:    gotest.NewReference(pkg, test),
		detail: db.FailedTestDetails{Fingerprint: fingerprint},
	}
}

func TestPickRuns(t *testing.T) {
	tests := []struct {
		name             string
		runIDs           []string
		explicitBaseline string
		wantTarget       string
		wantBaseline     string
		wantReason       string
		wantErr          require.ErrorAssertionFunc
	}{
		{
			name:         "explicit baseline wins",
			runIDs:       []string{"run-c", "run-b", "run-a"},
			wantTarget:   "run-c",
			wantBaseline: "explicit-id",
			// set below
		},
		{
			name:         "default is the prior run in the session",
			runIDs:       []string{"run-c", "run-b", "run-a"},
			wantTarget:   "run-c",
			wantBaseline: "run-b",
		},
		{
			name:       "first run has no baseline (valid, not an error)",
			runIDs:     []string{"run-a"},
			wantTarget: "run-a",
			wantReason: "no baseline; treating all failures as new",
		},
		{
			name:    "no runs is an error",
			runIDs:  nil,
			wantErr: require.Error,
		},
	}
	// explicit-baseline case needs the flag set
	tests[0].explicitBaseline = "explicit-id"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, baseline, reason, err := pickRuns(tt.runIDs, tt.explicitBaseline)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantTarget, target)
			require.Equal(t, tt.wantBaseline, baseline)
			require.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestDiffRuns(t *testing.T) {
	const pkg = "pkg/auth"

	tests := []struct {
		name      string
		target    []runFailure
		baseline  []runFailure
		flakyRefs map[string]bool
		targetSet map[string]bool
		want      verifyDiff
	}{
		{
			name:     "new regression: fails now, fingerprint absent from baseline",
			target:   []runFailure{fail(pkg, "TestToken", "fp-new")},
			baseline: nil,
			want:     verifyDiff{NewRegressions: []runFailure{fail(pkg, "TestToken", "fp-new")}},
		},
		{
			name:     "still-failing: same (reference, fingerprint) in both",
			target:   []runFailure{fail(pkg, "TestToken", "fp-1")},
			baseline: []runFailure{fail(pkg, "TestToken", "fp-1")},
			want:     verifyDiff{StillFailing: []runFailure{fail(pkg, "TestToken", "fp-1")}},
		},
		{
			name:     "same reference but different fingerprint is a new regression",
			target:   []runFailure{fail(pkg, "TestToken", "fp-2")},
			baseline: []runFailure{fail(pkg, "TestToken", "fp-1")},
			want:     verifyDiff{NewRegressions: []runFailure{fail(pkg, "TestToken", "fp-2")}},
		},
		{
			name:     "fixed: failed in baseline, no longer fails in target",
			target:   nil,
			baseline: []runFailure{fail(pkg, "TestToken", "fp-1")},
			want:     verifyDiff{Fixed: []gotest.Reference{gotest.NewReference(pkg, "TestToken")}},
		},
		{
			name:      "flaky dominates a target failure over new-regression",
			target:    []runFailure{fail(pkg, "TestRace", "fp-x")},
			baseline:  nil,
			flakyRefs: map[string]bool{pkg + "/TestRace": true},
			want:      verifyDiff{Flaky: []gotest.Reference{gotest.NewReference(pkg, "TestRace")}},
		},
		{
			name:      "flaky dominates a would-be fixed reference",
			target:    nil,
			baseline:  []runFailure{fail(pkg, "TestRace", "fp-x")},
			flakyRefs: map[string]bool{pkg + "/TestRace": true},
			want:      verifyDiff{Flaky: []gotest.Reference{gotest.NewReference(pkg, "TestRace")}},
		},
		{
			name:      "a failing target ref is omitted from the diff buckets (reported via the target list)",
			target:    []runFailure{fail(pkg, "TestRace", "fp-x")},
			baseline:  nil,
			flakyRefs: map[string]bool{pkg + "/TestRace": true},
			targetSet: map[string]bool{pkg + "/TestRace": true},
			want:      verifyDiff{},
		},
		{
			name:      "a fixed target ref is omitted from the diff buckets (reported via the target list)",
			target:    nil,
			baseline:  []runFailure{fail(pkg, "TestRace", "fp-x")},
			flakyRefs: map[string]bool{pkg + "/TestRace": true},
			targetSet: map[string]bool{pkg + "/TestRace": true},
			want:      verifyDiff{},
		},
		{
			name: "collateral failures still bucket while target refs are omitted",
			target: []runFailure{
				fail(pkg, "TestTarget", "fp-t"),     // a target: omitted from buckets
				fail(pkg, "TestCollateral", "fp-c"), // not a target: a real new regression
			},
			baseline:  nil,
			targetSet: map[string]bool{pkg + "/TestTarget": true},
			want:      verifyDiff{NewRegressions: []runFailure{fail(pkg, "TestCollateral", "fp-c")}},
		},
		{
			name: "mixed buckets sorted deterministically",
			target: []runFailure{
				fail(pkg, "TestB", "fp-new"), // new regression
				fail(pkg, "TestA", "fp-1"),   // still-failing
			},
			baseline: []runFailure{
				fail(pkg, "TestA", "fp-1"), // still-failing
				fail(pkg, "TestC", "fp-9"), // fixed (not in target)
			},
			want: verifyDiff{
				NewRegressions: []runFailure{fail(pkg, "TestB", "fp-new")},
				StillFailing:   []runFailure{fail(pkg, "TestA", "fp-1")},
				Fixed:          []gotest.Reference{gotest.NewReference(pkg, "TestC")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffRuns(tt.target, tt.baseline, tt.flakyRefs, tt.targetSet)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(runFailure{})); diff != "" {
				t.Errorf("diffRuns() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTargetStatus(t *testing.T) {
	tests := []struct {
		name          string
		ran           bool
		failingNow    bool
		failingBefore bool
		want          string
	}{
		{"fixed: failed before, passes now", true, false, true, targetFixed},
		{"passing: green in both runs", true, false, false, targetPassing},
		{"still-failing: failed in both", true, true, true, targetStillFailing},
		{"regressed: passed before, fails now", true, true, false, targetRegressed},
		{"not-run: never reached a terminal outcome", false, false, false, targetNotRun},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, targetStatus(tt.ran, tt.failingNow, tt.failingBefore))
		})
	}
}

func TestBuildVerifyResult(t *testing.T) {
	const pkg = "pkg/auth"

	tests := []struct {
		name             string
		diff             verifyDiff
		targets          []verifyTargetJSON
		noBaselineReason string
		targetReason     string
		want             verifyResultJSON
	}{
		{
			name:    "clean: target fixed, no regressions -> ok",
			diff:    verifyDiff{Fixed: []gotest.Reference{gotest.NewReference(pkg, "TestLogin")}},
			targets: []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetFixed}},
			want: verifyResultJSON{
				Targets:        []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetFixed}},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{},
				FlakyIgnored:   []string{},
				Summary:        "target fixed; no new regressions",
				OK:             true,
			},
		},
		{
			name: "a new regression flips ok to false even when the target is fixed",
			diff: verifyDiff{
				NewRegressions: []runFailure{fail(pkg, "TestToken", "fp-new")},
			},
			targets: []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetFixed}},
			want: verifyResultJSON{
				Targets: []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetFixed}},
				NewRegressions: []verifyRegressionJSON{
					{Reference: pkg + "/TestToken", Fingerprint: "fp-new", Repro: "go test pkg/auth -run '^TestToken$'"},
				},
				NewRegressionClusters: []clusterJSON{
					{Symptom: "failure", Count: 1, References: []string{pkg + "/TestToken"}, SampleRepro: "go test pkg/auth -run '^TestToken$'"},
				},
				StillFailing: []verifyStillJSON{},
				FlakyIgnored: []string{},
				Summary:      "target fixed; 1 new regression",
				OK:           false,
			},
		},
		{
			name: "no targets: ok when there are no new regressions",
			diff: verifyDiff{
				StillFailing: []runFailure{fail(pkg, "TestOld", "fp-1")},
				Flaky:        []gotest.Reference{gotest.NewReference(pkg, "TestRace")},
			},
			want: verifyResultJSON{
				Targets:        []verifyTargetJSON{},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{{Reference: pkg + "/TestOld", Verdict: VerdictPreExisting}},
				FlakyIgnored:   []string{pkg + "/TestRace"},
				Summary:        "no new regressions; 1 pre-existing; 1 flaky ignored",
				OK:             true,
			},
		},
		{
			name:         "empty target set with a reason is ok, reason surfaces in the summary",
			diff:         verifyDiff{},
			targetReason: "no target tests in the changed packages",
			want: verifyResultJSON{
				Targets:        []verifyTargetJSON{},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{},
				FlakyIgnored:   []string{},
				Summary:        "no target tests in the changed packages; no new regressions",
				OK:             true,
			},
		},
		{
			name: "multiple targets: one still-failing flips ok to false",
			diff: verifyDiff{},
			targets: []verifyTargetJSON{
				{Reference: pkg + "/TestA", Status: targetFixed},
				{Reference: pkg + "/TestB", Status: targetPassing},
				{Reference: pkg + "/TestC", Status: targetStillFailing},
			},
			want: verifyResultJSON{
				Targets: []verifyTargetJSON{
					{Reference: pkg + "/TestA", Status: targetFixed},
					{Reference: pkg + "/TestB", Status: targetPassing},
					{Reference: pkg + "/TestC", Status: targetStillFailing},
				},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{},
				FlakyIgnored:   []string{},
				Summary:        "3 targets (1 fixed, 1 passing, 1 still-failing); no new regressions",
				OK:             false,
			},
		},
		{
			name:    "multiple targets: all fixed/passing is ok",
			diff:    verifyDiff{},
			targets: []verifyTargetJSON{{Reference: pkg + "/TestA", Status: targetFixed}, {Reference: pkg + "/TestB", Status: targetPassing}},
			want: verifyResultJSON{
				Targets:        []verifyTargetJSON{{Reference: pkg + "/TestA", Status: targetFixed}, {Reference: pkg + "/TestB", Status: targetPassing}},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{},
				FlakyIgnored:   []string{},
				Summary:        "2 targets (1 fixed, 1 passing); no new regressions",
				OK:             true,
			},
		},
		{
			name:             "no baseline reason surfaces in the summary",
			diff:             verifyDiff{NewRegressions: []runFailure{fail(pkg, "TestToken", "fp-new")}},
			noBaselineReason: "no baseline; treating all failures as new",
			want: verifyResultJSON{
				Targets: []verifyTargetJSON{},
				NewRegressions: []verifyRegressionJSON{
					{Reference: pkg + "/TestToken", Fingerprint: "fp-new", Repro: "go test pkg/auth -run '^TestToken$'"},
				},
				NewRegressionClusters: []clusterJSON{
					{Symptom: "failure", Count: 1, References: []string{pkg + "/TestToken"}, SampleRepro: "go test pkg/auth -run '^TestToken$'"},
				},
				StillFailing: []verifyStillJSON{},
				FlakyIgnored: []string{},
				Summary:      "no baseline; treating all failures as new; 1 new regression",
				OK:           false,
			},
		},
		{
			name:    "target still-failing is not ok",
			diff:    verifyDiff{},
			targets: []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetStillFailing}},
			want: verifyResultJSON{
				Targets:        []verifyTargetJSON{{Reference: pkg + "/TestLogin", Status: targetStillFailing}},
				NewRegressions: []verifyRegressionJSON{},
				StillFailing:   []verifyStillJSON{},
				FlakyIgnored:   []string{},
				Summary:        "target still-failing; no new regressions",
				OK:             false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVerifyResult(tt.diff, tt.targets, targetProvenance{}, tt.noBaselineReason, tt.targetReason, nil)
			// the Failure sub-struct is exercised by triage tests; compare everything else here
			if diff := cmp.Diff(tt.want, got, cmp.Comparer(func(a, b triageFailureJSON) bool { return true })); diff != "" {
				t.Errorf("buildVerifyResult() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// fakeGitSource records which discovery func the selector dispatch chose and returns canned
// results, so resolveTargetFiles is testable without a real repo.
func fakeGitSource(t *testing.T) (gitSource, *[]string) {
	t.Helper()
	var calls []string
	g := gitSource{
		changedGoFiles: func(string) ([]string, error) {
			calls = append(calls, "changedGoFiles")
			return []string{"working.go"}, nil
		},
		changedGoFilesSince: func(_ string, ref string) ([]string, error) {
			calls = append(calls, "changedGoFilesSince("+ref+")")
			return []string{"since.go"}, nil
		},
		currentBranch: func(string) string {
			calls = append(calls, "currentBranch")
			return "feature" // a feature branch by default, so @auto takes the branch path
		},
		defaultBranch: func(string) (string, error) {
			calls = append(calls, "defaultBranch")
			return "main", nil
		},
		mergeBase: func(_ string, baseRef string) (string, error) {
			calls = append(calls, "mergeBase("+baseRef+")")
			return "MB", nil
		},
	}
	return g, &calls
}

func TestResolveTargetFiles(t *testing.T) {
	t.Run("@diff diffs the working tree", func(t *testing.T) {
		g, calls := fakeGitSource(t)
		files, basis, reason, err := resolveTargetFiles(g, selectorDiff, "")
		require.NoError(t, err)
		require.Empty(t, reason)
		require.Equal(t, basisWorkingTree, basis)
		require.Equal(t, []string{"working.go"}, files)
		require.Equal(t, []string{"changedGoFiles"}, *calls)
	})

	t.Run("@auto on a feature branch diffs against the default branch's merge-base", func(t *testing.T) {
		g, calls := fakeGitSource(t)
		files, basis, reason, err := resolveTargetFiles(g, selectorAuto, "")
		require.NoError(t, err)
		require.Empty(t, reason)
		require.Equal(t, []string{"since.go"}, files)
		require.Contains(t, basis, "merge-base with main")
		require.Equal(t, []string{"currentBranch", "defaultBranch", "mergeBase(main)", "changedGoFilesSince(MB)"}, *calls)
	})

	t.Run("@auto on the default branch diffs the working tree", func(t *testing.T) {
		g, calls := fakeGitSource(t)
		g.currentBranch = func(string) string { *calls = append(*calls, "currentBranch"); return "main" }
		files, basis, reason, err := resolveTargetFiles(g, selectorAuto, "")
		require.NoError(t, err)
		require.Empty(t, reason)
		require.Equal(t, basisWorkingTree, basis)
		require.Equal(t, []string{"working.go"}, files)
		require.Equal(t, []string{"currentBranch", "defaultBranch", "changedGoFiles"}, *calls)
	})

	t.Run("@auto on detached HEAD diffs the working tree", func(t *testing.T) {
		g, _ := fakeGitSource(t)
		g.currentBranch = func(string) string { return "HEAD" }
		files, basis, _, err := resolveTargetFiles(g, selectorAuto, "")
		require.NoError(t, err)
		require.Equal(t, basisWorkingTree, basis)
		require.Equal(t, []string{"working.go"}, files)
	})

	t.Run("@auto with a base honors it via the branch path", func(t *testing.T) {
		g, calls := fakeGitSource(t)
		files, basis, reason, err := resolveTargetFiles(g, selectorAuto, "origin/main")
		require.NoError(t, err)
		require.Empty(t, reason)
		require.Equal(t, []string{"since.go"}, files)
		require.Contains(t, basis, "merge-base with origin/main")
		// no branch detection: the explicit base is used directly
		require.Equal(t, []string{"mergeBase(origin/main)", "changedGoFilesSince(MB)"}, *calls)
	})

	t.Run("@branch with no base detects the default branch", func(t *testing.T) {
		g, calls := fakeGitSource(t)
		files, basis, reason, err := resolveTargetFiles(g, selectorBranch, "")
		require.NoError(t, err)
		require.Empty(t, reason)
		require.Equal(t, []string{"since.go"}, files)
		require.Contains(t, basis, "merge-base with main")
		require.Equal(t, []string{"defaultBranch", "mergeBase(main)", "changedGoFilesSince(MB)"}, *calls)
	})

	t.Run("@branch degrades to the working tree when the default branch is unknown", func(t *testing.T) {
		g, _ := fakeGitSource(t)
		g.defaultBranch = func(string) (string, error) { return "", fmt.Errorf("no default") }
		files, basis, reason, err := resolveTargetFiles(g, selectorBranch, "")
		require.NoError(t, err)
		require.Equal(t, basisWorkingTree, basis)
		require.Equal(t, []string{"working.go"}, files)
		require.NotEmpty(t, reason)
	})

	t.Run("@branch degrades to the working tree when the merge-base is unknown", func(t *testing.T) {
		g, _ := fakeGitSource(t)
		g.mergeBase = func(string, string) (string, error) { return "", fmt.Errorf("no merge base") }
		files, basis, reason, err := resolveTargetFiles(g, selectorBranch, "feature")
		require.NoError(t, err)
		require.Equal(t, basisWorkingTree, basis)
		require.Equal(t, []string{"working.go"}, files)
		require.NotEmpty(t, reason)
	})

	t.Run("unknown selector errors", func(t *testing.T) {
		g, _ := fakeGitSource(t)
		_, _, _, err := resolveTargetFiles(g, "@bogus", "")
		require.Error(t, err)
	})
}

func TestProvenanceHeader(t *testing.T) {
	tests := []struct {
		name string
		prov targetProvenance
		want string
	}{
		{"no selector -> empty", targetProvenance{}, ""},
		{"explicit ref", targetProvenance{Selector: "explicit", Basis: "explicit reference", TargetTests: 1}, "target: explicit reference"},
		{
			"diff basis with counts",
			targetProvenance{Selector: "@auto", Basis: "working-tree changes", ChangedFiles: 1, AffectedPackages: 3, TargetTests: 5},
			"targets: @auto from working-tree changes (1 changed file, 3 affected packages)",
		},
		{"resolved to files but no basis", targetProvenance{Selector: "@auto"}, "targets: @auto (unresolved)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, provenanceHeader(tt.prov))
		})
	}
}

func TestIntersectTargets(t *testing.T) {
	const (
		inPkg  = "github.com/org/repo/auth"
		outPkg = "github.com/org/repo/other"
	)
	terminal := []gotest.Reference{
		gotest.NewReference(inPkg, "TestB"),
		gotest.NewReference(outPkg, "TestX"), // package not affected -> dropped
		gotest.NewReference(inPkg, "TestA"),
	}
	affected := strset.New(inPkg)

	got := intersectTargets(terminal, affected)
	want := []gotest.Reference{
		gotest.NewReference(inPkg, "TestA"),
		gotest.NewReference(inPkg, "TestB"),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("intersectTargets() mismatch (-want +got):\n%s", diff)
	}
}
