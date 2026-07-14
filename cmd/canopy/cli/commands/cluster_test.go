package commands

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
	"gorm.io/datatypes"
)

// panicFail builds a panic runFailure whose stack has topFrame as its most-recent user frame.
func panicFail(pkg, test, message string, topFrame failure.StackFrame, tail ...failure.StackFrame) runFailure {
	frames := append([]failure.StackFrame{
		{Function: "runtime.gopanic", File: "runtime/panic.go", Line: 1, IsUser: false},
		topFrame,
	}, tail...)
	data, _ := json.Marshal(failure.PanicInfo{Message: message, Frames: frames})
	return runFailure{
		ref:    gotest.NewReference(pkg, test),
		detail: db.FailedTestDetails{Type: string(failure.PanicFailure), Details: datatypes.JSON(data)},
	}
}

// assertFail builds an assertion runFailure with the given expected/actual and fingerprint.
func assertFail(pkg, test, expected, actual, fingerprint string) runFailure {
	data, _ := json.Marshal(failure.AssertionInfo{
		Version: failure.AssertionInfoVersion, Library: "testify", Function: "Equal",
		Expected: expected, Actual: actual,
	})
	return runFailure{
		ref: gotest.NewReference(pkg, test),
		detail: db.FailedTestDetails{
			Type: string(failure.AssertionFailure), Details: datatypes.JSON(data),
			LocationFile: "handler_test.go", LocationLine: 42, Fingerprint: fingerprint,
		},
	}
}

func TestClusterFailures_FingerprintGrouping(t *testing.T) {
	// 37 failures sharing a fingerprint collapse to one cluster; a distinct cause stays split.
	var failures []runFailure
	for i := 0; i < 37; i++ {
		failures = append(failures, fail("pkg/db", fmt.Sprintf("Test%02d", i), "fp-shared"))
	}
	failures = append(failures, fail("pkg/handler", "TestLogin", "fp-other"))

	res := clusterFailures(failures)

	require.Len(t, res.Clusters, 2)
	// sorted by count desc: the fan-out symptom first
	require.Equal(t, 37, res.Clusters[0].Count)
	require.Len(t, res.Clusters[0].References, 37)
	require.Equal(t, 1, res.Clusters[1].Count)
	require.Equal(t, "38 failures across 2 distinct symptoms", res.Summary)
}

func TestClusterFailures_Ordering(t *testing.T) {
	// a mixed run: a 3-way panic fan-out, a 2-way assertion, and a lone assertion. the golden asserts
	// the full JSON shape AND the count-desc ordering.
	site := failure.StackFrame{Function: "pkg/db.(*Store).Write", File: "db/store.go", Line: 212, IsUser: true}
	failures := []runFailure{
		assertFail("pkg/handler", "TestH2", "200", "500", "fp-assert"),
		panicFail("pkg/db", "TestP3", "assignment to entry in nil map", site,
			failure.StackFrame{Function: "pkg/db.TestP3", File: "db/c_test.go", Line: 9, IsUser: true}),
		assertFail("pkg/handler", "TestH3", "1", "2", "fp-other"),
		panicFail("pkg/db", "TestP1", "assignment to entry in nil map", site,
			failure.StackFrame{Function: "pkg/db.TestP1", File: "db/a_test.go", Line: 7, IsUser: true}),
		assertFail("pkg/handler", "TestH1", "200", "500", "fp-assert"),
		panicFail("pkg/db", "TestP2", "assignment to entry in nil map", site,
			failure.StackFrame{Function: "pkg/db.TestP2", File: "db/b_test.go", Line: 8, IsUser: true}),
	}

	got := clusterFailures(failures)

	want := clusterResultJSON{
		Clusters: []clusterJSON{
			{
				Symptom: "panic: assignment to entry in nil map", Location: "db/store.go:212", Count: 3,
				References:  []string{"pkg/db/TestP1", "pkg/db/TestP2", "pkg/db/TestP3"},
				SampleRepro: "go test pkg/db -run '^(TestP1|TestP2|TestP3)$'",
			},
			{
				Symptom: "assertion: expected 200, got 500", Location: "handler_test.go:42", Count: 2,
				References:  []string{"pkg/handler/TestH1", "pkg/handler/TestH2"},
				SampleRepro: "go test pkg/handler -run '^(TestH1|TestH2)$'",
			},
			{
				Symptom: "assertion: expected 1, got 2", Location: "handler_test.go:42", Count: 1,
				References:  []string{"pkg/handler/TestH3"},
				SampleRepro: "go test pkg/handler -run '^TestH3$'",
			},
		},
		Summary: "6 failures across 3 distinct symptoms",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("clusterFailures() mismatch (-want +got):\n%s", diff)
	}
}

func TestClusterKey(t *testing.T) {
	site := failure.StackFrame{Function: "pkg/db.(*Store).Write", File: "db/store.go", Line: 212, IsUser: true}
	// two panics at the same user site reached through different stacks (and with transient
	// addresses in the message) share a key; a panic at a different site does not.
	a := panicFail("pkg/a", "TestA", "runtime error at 0xc0001", site,
		failure.StackFrame{Function: "pkg/a.TestA", File: "a_test.go", Line: 1, IsUser: true})
	b := panicFail("pkg/b", "TestB", "runtime error at 0xffee2", site,
		failure.StackFrame{Function: "pkg/b.TestB", File: "b_test.go", Line: 2, IsUser: true})
	other := panicFail("pkg/c", "TestC", "runtime error at 0xabcd3",
		failure.StackFrame{Function: "pkg/c.other", File: "other.go", Line: 99, IsUser: true})

	require.Equal(t, clusterKey(a.detail), clusterKey(b.detail))
	require.NotEqual(t, clusterKey(a.detail), clusterKey(other.detail))

	// a panic with no user frame falls back to the fingerprint.
	noUser := runFailure{detail: db.FailedTestDetails{
		Type:        string(failure.PanicFailure),
		Details:     mustJSON(failure.PanicInfo{Message: "boom", Frames: []failure.StackFrame{{Function: "runtime.x", File: "runtime/x.go", IsUser: false}}}),
		Fingerprint: "fp-fallback",
	}}
	require.Equal(t, "fp-fallback", clusterKey(noUser.detail))

	// assertions always key on the fingerprint.
	as := assertFail("pkg/h", "TestH", "1", "2", "fp-assert")
	require.Equal(t, "fp-assert", clusterKey(as.detail))
}

func TestBuildClusterRepro(t *testing.T) {
	ref := gotest.NewReference
	tests := []struct {
		name string
		refs []gotest.Reference
		want string
	}{
		{
			name: "subtests sharing a parent collapse to the parent func",
			refs: []gotest.Reference{
				ref("pkg/flaky", "TestScore/a"),
				ref("pkg/flaky", "TestScore/b"),
				ref("pkg/flaky", "TestScore/c"),
			},
			want: "go test pkg/flaky -run '^TestScore$'",
		},
		{
			name: "distinct top-level tests in one package join into one regex",
			refs: []gotest.Reference{
				ref("pkg/db", "TestB"),
				ref("pkg/db", "TestA"),
			},
			want: "go test pkg/db -run '^(TestA|TestB)$'",
		},
		{
			name: "cross-package cluster emits one line per package, sorted",
			refs: []gotest.Reference{
				ref("pkg/z", "TestZ"),
				ref("pkg/a", "TestA/sub"),
				ref("pkg/a", "TestB"),
			},
			want: "go test pkg/a -run '^(TestA|TestB)$'\ngo test pkg/z -run '^TestZ$'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, buildClusterRepro(tt.refs)); diff != "" {
				t.Errorf("buildClusterRepro() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func mustJSON(v any) datatypes.JSON {
	data, _ := json.Marshal(v)
	return datatypes.JSON(data)
}
