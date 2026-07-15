package gotest

import "os"

// ExecFingerprint captures the execution conditions of a run so a repro can recreate them.
// It is persisted as part of RunnerConfig (the run's Config JSON blob), not a separate table.
// A nil fingerprint on an older run means "not captured"; repro builders fall back to the plain
// `go test ... -run` form in that case.
type ExecFingerprint struct {
	// ShuffleSeed is the explicit -shuffle seed used for this run, or nil when shuffle was off.
	// canopy generates the seed and passes -shuffle=<seed> rather than parsing the seed go prints,
	// so the recorded value is authoritative.
	ShuffleSeed *int64 `json:"shuffle_seed,omitempty"`
	// Race records whether -race was enabled.
	Race bool `json:"race,omitempty"`
	// Count records the -count value (0 means go's default of 1).
	Count int `json:"count,omitempty"`
	// Tags records the comma-separated build -tags.
	Tags string `json:"tags,omitempty"`
	// GoVersion is runtime.Version() of the toolchain, so a repro under a different toolchain is visible.
	GoVersion string `json:"go_version,omitempty"`
	// GOOS is the target operating system the run happened on.
	GOOS string `json:"goos,omitempty"`
	// GOARCH is the target architecture the run happened on.
	GOARCH string `json:"goarch,omitempty"`
	// Env holds only allowlisted plus user-named env vars that were set, never the whole environment.
	Env map[string]string `json:"env,omitempty"`
}

// reproEnvAllowlist is the fixed set of env vars known to affect test execution. it is deliberately
// tiny: capturing the whole environment is noisy and can leak secrets.
//
// ponytail: allowlist + user-named keys only, never os.Environ(). known ceiling: a test that depends
// on an un-listed env var will not reproduce; upgrade path is naming that var in --repro-env, not
// auto-capturing everything.
var reproEnvAllowlist = []string{"GOFLAGS", "CGO_ENABLED"}

// CaptureReproEnv returns the values of the allowlisted vars plus any extra user-named keys that are
// actually set in the environment. an unset var is omitted (an absent var is a non-default the repro
// must not fabricate). returns nil when nothing was captured so the fingerprint JSON stays empty.
func CaptureReproEnv(extra []string) map[string]string {
	out := map[string]string{}
	for _, k := range append(append([]string{}, reproEnvAllowlist...), extra...) {
		if k == "" {
			continue
		}
		if v, ok := os.LookupEnv(k); ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
