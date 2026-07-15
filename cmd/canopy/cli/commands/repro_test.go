package commands

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func seedPtr(v int64) *int64 { return &v }

// thisToolchain returns a fingerprint whose toolchain fields match the running process, so the
// toolchain note is suppressed unless a test overrides them.
func thisToolchain() *gotest.ExecFingerprint {
	return &gotest.ExecFingerprint{
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}

func TestReproCommand_Fingerprint(t *testing.T) {
	tests := []struct {
		name string
		fp   *gotest.ExecFingerprint
		want string
	}{
		{
			name: "nil fingerprint falls back to plain command",
			fp:   nil,
			want: "go test ./pkg/auth -run '^TestToken$'",
		},
		{
			name: "empty fingerprint (all defaults) stays plain",
			fp:   thisToolchain(),
			want: "go test ./pkg/auth -run '^TestToken$'",
		},
		{
			name: "race only",
			fp:   withFP(thisToolchain(), func(f *gotest.ExecFingerprint) { f.Race = true }),
			want: "go test ./pkg/auth -race -run '^TestToken$'",
		},
		{
			name: "count and tags",
			fp: withFP(thisToolchain(), func(f *gotest.ExecFingerprint) {
				f.Count = 3
				f.Tags = "integration"
			}),
			want: "go test ./pkg/auth -count=3 -tags=integration -run '^TestToken$'",
		},
		{
			name: "shuffle seed emits -shuffle=on plus the seed",
			fp:   withFP(thisToolchain(), func(f *gotest.ExecFingerprint) { f.ShuffleSeed = seedPtr(1737) }),
			want: "go test ./pkg/auth -shuffle=on -test.shuffle=1737 -run '^TestToken$'",
		},
		{
			name: "env prefix, sorted, with quoting only where needed",
			fp: withFP(thisToolchain(), func(f *gotest.ExecFingerprint) {
				f.Env = map[string]string{"GOFLAGS": "-tags=integration x", "CGO_ENABLED": "1"}
			}),
			want: "CGO_ENABLED=1 GOFLAGS='-tags=integration x' go test ./pkg/auth -run '^TestToken$'",
		},
		{
			name: "everything together",
			fp: withFP(thisToolchain(), func(f *gotest.ExecFingerprint) {
				f.Race = true
				f.Count = 1
				f.Tags = "integration"
				f.ShuffleSeed = seedPtr(42)
				f.Env = map[string]string{"CGO_ENABLED": "0"}
			}),
			want: "CGO_ENABLED=0 go test ./pkg/auth -race -count=1 -tags=integration -shuffle=on -test.shuffle=42 -run '^TestToken$'",
		},
		{
			name: "differing toolchain appends a recorded-under note",
			fp: &gotest.ExecFingerprint{
				ShuffleSeed: seedPtr(99),
				GoVersion:   "go1.20.0",
				GOOS:        "linux",
				GOARCH:      "amd64",
			},
			want: "go test ./pkg/auth -shuffle=on -test.shuffle=99 -run '^TestToken$' # recorded under go1.20.0 linux/amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reproCommand("./pkg/auth", "TestToken", tt.fp)
			if got != tt.want {
				t.Errorf("reproCommand()\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// TestBuildRepro_SeedRoundTrips proves the recorded seed survives into the emitted repro (the core
// hermetic-repro guarantee: re-running the emitted command uses the same seed).
func TestBuildRepro_SeedRoundTrips(t *testing.T) {
	const seed int64 = 1737123456789
	fp := withFP(thisToolchain(), func(f *gotest.ExecFingerprint) { f.ShuffleSeed = seedPtr(seed) })

	got := buildRepro(gotest.NewReference("./pkg/auth", "TestToken"), fp)
	want := fmt.Sprintf("go test ./pkg/auth -shuffle=on -test.shuffle=%d -run '^TestToken$'", seed)
	if got != want {
		t.Errorf("buildRepro()\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildRepro_Subtest checks a subtest reference anchors on the full TestFunc/subtest path.
func TestBuildRepro_Subtest(t *testing.T) {
	fp := withFP(thisToolchain(), func(f *gotest.ExecFingerprint) { f.Race = true })
	got := buildRepro(gotest.NewReference("./pkg/auth", "TestToken/expired"), fp)
	want := "go test ./pkg/auth -race -run '^TestToken/expired$'"
	if got != want {
		t.Errorf("buildRepro()\n got: %s\nwant: %s", got, want)
	}
}

// withFP applies mutations to a fingerprint and returns it (small builder for table rows).
func withFP(fp *gotest.ExecFingerprint, mut func(*gotest.ExecFingerprint)) *gotest.ExecFingerprint {
	mut(fp)
	return fp
}
