package gostd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestWithTestsElapsed(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pkg := gotest.NewReference("example.com/a", "")
	test := gotest.NewReference("example.com/a", "TestA")

	resultWithStartup := func(startup time.Duration) *gotest.Result {
		r := gotest.NewResult(gotest.ResultConfig{})
		for _, e := range []gotest.Event{
			{Reference: pkg, Action: gotest.StartAction, Time: base},
			{Reference: test, Action: gotest.RunAction, Time: base.Add(startup)},
			{Reference: test, Action: gotest.PassAction, Time: base.Add(startup + 21*time.Millisecond)},
			{Reference: pkg, Action: gotest.PassAction, Time: base.Add(startup + 30*time.Millisecond)},
		} {
			r.Update(e)
		}
		return r
	}

	cases := []struct {
		name    string
		startup time.Duration
		output  string
		want    string
	}{
		{
			name:    "ok line",
			startup: 7810 * time.Millisecond,
			output:  "ok  \texample.com/a\t7.832s\n",
			want:    "ok  \texample.com/a\t7.832s ●\n",
		},
		{
			name:    "keeps coverage and notes",
			startup: 7810 * time.Millisecond,
			output:  "ok  \texample.com/a\t7.832s\tcoverage: 82.1% of statements\t(started after 5.00s)\n",
			want:    "ok  \texample.com/a\t7.832s ●\tcoverage: 82.1% of statements\t(started after 5.00s)\n",
		},
		{
			name:    "fail line",
			startup: 7810 * time.Millisecond,
			output:  "FAIL\texample.com/a\t7.832s\n",
			want:    "FAIL\texample.com/a\t7.832s ●\n",
		},
		{
			name:    "quarter startup",
			startup: 1500 * time.Millisecond,
			output:  "ok  \texample.com/a\t6.000s\n",
			want:    "ok  \texample.com/a\t6.000s ◔\n",
		},
		{
			name:    "half startup",
			startup: 3 * time.Second,
			output:  "ok  \texample.com/a\t6.000s\n",
			want:    "ok  \texample.com/a\t6.000s ◑\n",
		},
		{
			name:    "three quarters startup",
			startup: 4500 * time.Millisecond,
			output:  "ok  \texample.com/a\t6.000s\n",
			want:    "ok  \texample.com/a\t6.000s ◕\n",
		},
		{
			name:    "notable startup under the nearest quarter",
			startup: 1200 * time.Millisecond,
			output:  "ok  \texample.com/a\t17.760s\n",
			want:    "ok  \texample.com/a\t17.760s\n",
		},
		{
			name:    "startup not notable",
			startup: 300 * time.Millisecond,
			output:  "ok  \texample.com/a\t0.330s\n",
			want:    "ok  \texample.com/a\t0.330s\n",
		},
		{
			name:    "no elapsed field",
			startup: 7810 * time.Millisecond,
			output:  "FAIL\texample.com/a [build failed]\n",
			want:    "FAIL\texample.com/a [build failed]\n",
		},
		{
			name:    "not a summary line",
			startup: 7810 * time.Millisecond,
			output:  "some output\n",
			want:    "some output\n",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := gotest.Event{Reference: pkg, Action: gotest.OutputAction, Output: tt.output}
			got := withTestsElapsed(resultWithStartup(tt.startup), pkg, e)
			require.Equal(t, tt.want, got.Output)
		})
	}
}

func TestFirstTestNote(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// a result where "example.com/ran" ran a test, "example.com/cached" came from the test cache, and
	// "example.com/none" had no tests to run
	result := gotest.NewResult(gotest.ResultConfig{})
	for _, e := range []gotest.Event{
		{Reference: gotest.NewReference("example.com/cached", ""), Action: gotest.StartAction, Time: base},
		{Reference: gotest.NewReference("example.com/cached", "TestA"), Action: gotest.RunAction, Time: base},
		{Reference: gotest.NewReference("example.com/cached", "TestA"), Action: gotest.PassAction, Time: base},
		{Reference: gotest.NewReference("example.com/cached", ""), Action: gotest.OutputAction, Output: "ok  \texample.com/cached\t(cached)\n", Annotations: []gotest.Annotation{gotest.Cached}, Time: base},
		{Reference: gotest.NewReference("example.com/cached", ""), Action: gotest.PassAction, Time: base},
		{Reference: gotest.NewReference("example.com/none", ""), Action: gotest.StartAction, Time: base},
		{Reference: gotest.NewReference("example.com/none", ""), Action: gotest.PassAction, Time: base},
		{Reference: gotest.NewReference("example.com/ran", ""), Action: gotest.StartAction, Time: base},
		{Reference: gotest.NewReference("example.com/ran", "TestA"), Action: gotest.RunAction, Time: base.Add(time.Second)},
		{Reference: gotest.NewReference("example.com/ran", "TestA"), Action: gotest.PassAction, Time: base.Add(time.Second)},
		{Reference: gotest.NewReference("example.com/ran", ""), Action: gotest.PassAction, Time: base.Add(time.Second)},
	} {
		result.Update(e)
	}

	line := func(pkg, out string) (gotest.Reference, gotest.Event) {
		ref := gotest.NewReference(pkg, "")
		return ref, gotest.Event{Reference: ref, Action: gotest.OutputAction, Output: out}
	}

	t.Run("noted on the first line whose tests ran, and only there", func(t *testing.T) {
		n := firstTestNote{launchedAt: base, firstTestAt: base.Add(14080 * time.Millisecond)}

		// lines before it that don't represent tests that ran are left alone
		ref, e := line("example.com/cached", "ok  \texample.com/cached\t(cached)\n")
		require.Equal(t, e.Output, n.annotate(result, ref, e).Output)
		ref, e = line("example.com/none", "ok  \texample.com/none\t0.010s [no tests to run]\n")
		require.Equal(t, e.Output, n.annotate(result, ref, e).Output)

		ref, e = line("example.com/ran", "ok  \texample.com/ran\t1.000s\tcoverage: 82.1% of statements\n")
		require.Equal(t, "ok  \texample.com/ran\t1.000s\tcoverage: 82.1% of statements\t(started after 14.08s)\n", n.annotate(result, ref, e).Output)

		// once is enough
		require.Equal(t, e.Output, n.annotate(result, ref, e).Output)
	})

	t.Run("a quick first test isn't worth a note, on that line or any later one", func(t *testing.T) {
		n := firstTestNote{launchedAt: base, firstTestAt: base.Add(300 * time.Millisecond)}

		ref, e := line("example.com/ran", "ok  \texample.com/ran\t1.000s\n")
		require.Equal(t, e.Output, n.annotate(result, ref, e).Output)
		require.True(t, n.done)
	})

	t.Run("no launch time (e.g. events arrived before the run request)", func(t *testing.T) {
		var n firstTestNote
		n.observe(gotest.Event{Reference: gotest.NewReference("example.com/ran", "TestA")})
		require.True(t, n.firstTestAt.IsZero(), "a first test with no launch to measure from must not be recorded")

		ref, e := line("example.com/ran", "ok  \texample.com/ran\t1.000s\n")
		require.Equal(t, e.Output, n.annotate(result, ref, e).Output)
	})
}
