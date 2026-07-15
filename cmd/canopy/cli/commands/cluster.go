package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
)

// clusterJSON is one cluster: N failures sharing a cluster key, collapsed to a single
// representative symptom/location/repro plus every member reference. this is the payload the fan-out
// case needs — "37 failures, 1 symptom, fix here" — instead of N near-identical failures. the symptom
// is what the failures have in common (same fingerprint / panic site), not their root cause.
type clusterJSON struct {
	Symptom     string   `json:"symptom"`
	Location    string   `json:"location,omitempty"`
	Count       int      `json:"count"`
	References  []string `json:"references"`
	SampleRepro string   `json:"sample_repro"`
}

// clusterResultJSON is the triage --cluster payload: clusters sorted by count descending plus a
// one-line summary.
type clusterResultJSON struct {
	Clusters []clusterJSON `json:"clusters"`
	Summary  string        `json:"summary"`
}

// clusterKey derives the within-run grouping key for a failure. the default is the failure
// fingerprint (already normalizes literal expected/actual values, so the same assertion with
// different literals collapses). panics get a coarser key — the panic site (top user-code frame)
// plus the normalized message — so one panic reached through different call stacks from dozens of
// tests collapses into a single cluster instead of over-splitting on the full stack.
//
// ponytail: fingerprint + panic-site only, exact-key grouping (no similarity model). the plan's
// build-failure refinement is deferred: there is no distinct build-failure type in the parsed
// failure data to key on today. upgrade path: when build failures are parsed into their own
// structured type, key them by failing package + compiler message signature.
func clusterKey(detail db.FailedTestDetails) string {
	if failure.Type(detail.Type) == failure.PanicFailure {
		if key, ok := panicSiteKey(detail); ok {
			return key
		}
	}
	return detail.Fingerprint
}

// panicSiteKey builds a cluster key from a panic's normalized message and its top user-code frame
// (the panic site). it returns false when the details don't parse or carry no user frame, so the
// caller falls back to the fingerprint.
func panicSiteKey(detail db.FailedTestDetails) (string, bool) {
	file, line, ok := topUserFrame(detail)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("panic\x00%s\x00%s:%d", panicMessage(detail), file, line), true
}

// topUserFrame returns the file/line of the most-recent user-code frame in a panic's stack (the
// panic site). frames are ordered most-recent first, so the first user frame is the deepest point
// in the caller's own code.
func topUserFrame(detail db.FailedTestDetails) (file string, line int, ok bool) {
	pi, ok := parsePanic(detail)
	if !ok {
		return "", 0, false
	}
	for _, f := range pi.Frames {
		if f.IsUser && f.File != "" {
			return f.File, f.Line, true
		}
	}
	return "", 0, false
}

func panicMessage(detail db.FailedTestDetails) string {
	pi, ok := parsePanic(detail)
	if !ok {
		return ""
	}
	return failure.Normalize(pi.Message)
}

func parsePanic(detail db.FailedTestDetails) (failure.PanicInfo, bool) {
	if len(detail.Details) == 0 {
		return failure.PanicInfo{}, false
	}
	var pi failure.PanicInfo
	if err := json.Unmarshal(detail.Details, &pi); err != nil {
		return failure.PanicInfo{}, false
	}
	return pi, true
}

// clusterFailures groups a run's failures by cluster key and returns them sorted by count
// descending (the fan-out cause first), ties broken by the representative reference so the order is
// stable. each cluster carries one representative symptom/location/repro (its first member in
// reference order) and every member reference — not N copies. pure (no DB/IO) so it is trivially
// testable.
func clusterFailures(failures []runFailure, fp *gotest.ExecFingerprint) clusterResultJSON {
	groups := map[string][]runFailure{}
	var keys []string
	for _, f := range failures {
		k := clusterKey(f.detail)
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], f)
	}

	clusters := make([]clusterJSON, 0, len(groups))
	for _, k := range keys {
		members := groups[k]
		sortFailures(members) // stable member + representative order
		rep := members[0]
		refs := make([]string, len(members))
		memberRefs := make([]gotest.Reference, len(members))
		for i, m := range members {
			refs[i] = m.ref.String(false)
			memberRefs[i] = m.ref
		}
		clusters = append(clusters, clusterJSON{
			Symptom:     describeSymptom(rep.detail),
			Location:    symptomLocation(rep.detail),
			Count:       len(members),
			References:  refs,
			SampleRepro: buildClusterRepro(memberRefs, fp),
		})
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		if clusters[i].Count != clusters[j].Count {
			return clusters[i].Count > clusters[j].Count
		}
		return clusters[i].References[0] < clusters[j].References[0]
	})

	return clusterResultJSON{Clusters: clusters, Summary: clusterSummary(clusters, len(failures))}
}

// buildClusterRepro renders `go test` command(s) that reproduce the WHOLE cluster, not just one
// member. members are grouped by package and collapsed to their parent function name (go's
// `-run '^TestParent$'` runs the parent and all its subtests), so a cluster of subtests sharing one
// parent becomes a single `-run '^TestParent$'`. a cluster spanning multiple packages cannot be run
// by one `go test` invocation, so it emits one line per package joined by newlines — the honest
// representation of a cross-package fan-out.
func buildClusterRepro(refs []gotest.Reference, fp *gotest.ExecFingerprint) string {
	funcsByPkg := map[string]map[string]bool{}
	var pkgs []string
	for _, r := range refs {
		if _, ok := funcsByPkg[r.Package]; !ok {
			funcsByPkg[r.Package] = map[string]bool{}
			pkgs = append(pkgs, r.Package)
		}
		funcsByPkg[r.Package][r.FuncName] = true
	}
	sort.Strings(pkgs)

	lines := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		funcs := make([]string, 0, len(funcsByPkg[pkg]))
		for fn := range funcsByPkg[pkg] {
			funcs = append(funcs, fn)
		}
		sort.Strings(funcs)
		pattern := funcs[0]
		if len(funcs) > 1 {
			pattern = "(" + strings.Join(funcs, "|") + ")"
		}
		lines = append(lines, reproCommand(pkg, pattern, fp))
	}
	return strings.Join(lines, "\n")
}

// describeSymptom renders a short human-readable symptom, mirroring the plan's examples
// ("panic: nil map write", "assertion: expected 200, got 500").
func describeSymptom(detail db.FailedTestDetails) string {
	switch failure.Type(detail.Type) {
	case failure.AssertionFailure:
		f := buildFailure(detail)
		if f.Expected != "" || f.Actual != "" {
			return fmt.Sprintf("assertion: expected %s, got %s", f.Expected, f.Actual)
		}
		return "assertion failure"
	case failure.PanicFailure:
		if msg := strings.TrimPrefix(panicMessage(detail), "panic: "); msg != "" {
			return "panic: " + msg
		}
		return "panic"
	case failure.DiffFailure:
		return "diff mismatch"
	case failure.TimeoutFailure:
		return "timeout"
	default:
		return "failure"
	}
}

// symptomLocation renders the representative source location. for panics it points at the panic site
// (top user frame) to match the cluster key; other failures use the stored location.
func symptomLocation(detail db.FailedTestDetails) string {
	if failure.Type(detail.Type) == failure.PanicFailure {
		if file, line, ok := topUserFrame(detail); ok {
			return fmt.Sprintf("%s:%d", file, line)
		}
	}
	if detail.LocationFile != "" {
		return fmt.Sprintf("%s:%d", detail.LocationFile, detail.LocationLine)
	}
	return ""
}

// clusterSummary renders the plan's one-liner: "38 failures across 2 distinct symptoms".
func clusterSummary(clusters []clusterJSON, total int) string {
	return fmt.Sprintf("%d %s across %d distinct %s",
		total, plural(total, "failure", "failures"),
		len(clusters), plural(len(clusters), "symptom", "symptoms"))
}
