package shard

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// JoinInput is everything the join needs. It reads no config itself.
type JoinInput struct {
	Receipts []LoadedReceipt // from LoadDir
	OutDir   string          // where the receipts and their coverprofiles are
	ShardDir string          // where coverage.out and metrics.json are written
	// CoverMin is the covermin set on the join itself (flag, env or config), nil when unset. It wins
	// over whatever the receipts recorded.
	CoverMin      *float64
	WriteMetrics  bool
	Overhead      time.Duration // per-job overhead for the suggestion
	P             int           // go test -p for the suggestion, 0 means the shards' runner cpus
	CanopyVersion string        // the join's own version, compared with the shards'
}

var groupHints = map[string]string{
	"plan":      "shards ran different canopy versions (or a different shard count); install the same version on every job",
	"selection": "different specifiers or excludes, or --affected-since resolved to different commits",
	"run":       "a flag or CANOPY_TEST_* env var is set on some jobs only",
	"gates":     "a flag or CANOPY_TEST_* env var is set on some jobs only",
	"go":        "a different toolchain, GOFLAGS or CGO setting, or a second matrix dimension",
	"source":    "shards checked out different commits",
	"weights":   "a cache race, or a partial rerun after main saved new metrics; rerun all jobs",
}

// Join verifies the receipts, merges coverage and metrics, applies the gates and suggests a shard
// count. Problems are reported in the Report; the error is only for failing to write coverage.out
// or metrics.json, and the Report is complete up to that point.
func Join(in JoinInput) (Report, error) {
	r := Report{Version: ReportVersion, Digests: []DigestGroup{}, Problems: []Problem{}, Warnings: []string{}, Shards: []ShardReport{}}
	err := join(&r, in)
	r.finish()
	return r, err
}

func join(r *Report, in JoinInput) error {
	var rs []*Receipt
	var names []string // receipt file names, parallel to rs
	for _, lr := range in.Receipts {
		if lr.Err != nil {
			r.problem(Problem{Kind: KindUnreadableReceipt, Message: fmt.Sprintf("unable to read receipt %s: %v", filepath.Base(lr.Path), lr.Err)})
			continue
		}
		rs = append(rs, lr.Receipt)
		names = append(names, strings.TrimSuffix(filepath.Base(lr.Path), ".json"))
	}
	if len(in.Receipts) == 0 {
		r.problem(Problem{Kind: KindUnreadableReceipt, Message: fmt.Sprintf("no receipts found in %s", in.OutDir)})
	}

	// 1. same total and version; the most common total sizes the report
	totals := map[int][]int{} // total -> receipt positions
	for i, rc := range rs {
		totals[rc.Total] = append(totals[rc.Total], i)
	}
	for t, at := range totals {
		if len(at) > len(totals[r.Total]) || (len(at) == len(totals[r.Total]) && t < r.Total) {
			r.Total = t
		}
	}
	for i := range max(r.Total, 0) {
		r.Shards = append(r.Shards, ShardReport{Index: i + 1})
	}
	if len(r.Problems) > 0 {
		return nil
	}
	stale := fmt.Sprintf("(stale files in %s?)", in.OutDir)
	if len(totals) > 1 {
		var says []string
		for _, t := range slices.Sorted(maps.Keys(totals)) {
			says = append(says, fmt.Sprintf("%s says %d", names[totals[t][0]], t))
		}
		r.problem(Problem{Kind: KindTotalMismatch, Message: fmt.Sprintf("receipts disagree on shard total: %s %s", strings.Join(says, ", "), stale)})
	}
	for i, rc := range rs {
		if rc.Version != rs[0].Version {
			r.problem(Problem{Kind: KindVersionMismatch, Message: fmt.Sprintf("receipts disagree on receipt version: %s is v%d, %s is v%d", names[0], rs[0].Version, names[i], rc.Version)})
			break
		}
		if rc.Index < 1 || rc.Index > rc.Total {
			r.problem(Problem{Kind: KindTotalMismatch, Message: fmt.Sprintf("%s is shard %d of %d %s", names[i], rc.Index, rc.Total, stale)})
		}
	}
	if len(r.Problems) > 0 {
		return nil
	}

	// 2. every index exactly once; the first receipt for an index stands for it
	byIndex := map[int][]int{}
	for i, rc := range rs {
		byIndex[rc.Index] = append(byIndex[rc.Index], i)
	}
	var present []*Receipt
	anyFailed := slices.ContainsFunc(rs, func(rc *Receipt) bool { return !rc.Passed })
	for i := 1; i <= r.Total; i++ {
		at := byIndex[i]
		switch {
		case len(at) == 0:
			p := Problem{Kind: KindMissingShard, Shards: []int{i}, Message: fmt.Sprintf("missing receipt for shard %d/%d (job failed before canopy finished, or artifact not uploaded)", i, r.Total)}
			if anyFailed {
				p.Hint = "cancelled? use fail-fast: false"
			}
			r.problem(p)
			continue
		case len(at) > 1:
			var files []string
			for _, j := range at {
				files = append(files, names[j])
			}
			r.problem(Problem{Kind: KindDuplicateShard, Shards: []int{i}, Message: fmt.Sprintf("shard %d/%d has %d receipts: %s %s", i, r.Total, len(at), strings.Join(files, ", "), stale)})
		}
		present = append(present, rs[at[0]])
	}
	complete := len(present) == r.Total && len(rs) == r.Total

	old, oldDigest, err := LoadMetrics(filepath.Join(in.ShardDir, "metrics.json"))
	if err != nil {
		old = nil
	}
	for _, rc := range present {
		r.Shards[rc.Index-1] = shardReport(rc, old, oldDigest)
		r.Checks.Tests.Passed += rc.Tests.Passed
		r.Checks.Tests.Failed += rc.Tests.Failed
		r.Checks.Tests.Skipped += rc.Tests.Skipped
	}
	r.Checks.Verified.Receipts = len(present)

	// 3. one digest
	byDigest := map[string][]int{}
	for _, rc := range present {
		byDigest[rc.Digest] = append(byDigest[rc.Digest], rc.Index)
	}
	for d, idx := range byDigest {
		r.Digests = append(r.Digests, DigestGroup{Digest: d, Shards: idx})
	}
	slices.SortFunc(r.Digests, func(a, b DigestGroup) int {
		if len(a.Shards) != len(b.Shards) {
			return len(b.Shards) - len(a.Shards)
		}
		return a.Shards[0] - b.Shards[0]
	})
	oneDigest := len(r.Digests) <= 1
	if !oneDigest {
		for _, p := range inputMismatches(present) {
			r.problem(p)
		}
	}

	// 4 and 5. exact cover, over planned with one plan and over reported otherwise (what actually ran)
	units := unionUnits(present)
	r.Packages = len(units)
	sameUnits := !slices.ContainsFunc(present, func(rc *Receipt) bool { return !slices.Equal(rc.Units, present[0].Units) })
	ran := map[string][]int{}
	for _, rc := range present {
		set := rc.Planned
		if !oneDigest {
			set = rc.Reported
		}
		for _, p := range set {
			ran[p] = append(ran[p], rc.Index)
		}
	}
	var never, twice []string
	var twiceShards []int
	for _, u := range units {
		switch n := len(ran[u]); {
		case n == 0:
			never = append(never, u)
		case n == 1:
			r.Checks.Verified.RanOnce++
		default:
			twice = append(twice, u)
			twiceShards = append(twiceShards, ran[u]...)
		}
	}
	if complete && sameUnits {
		if len(never) > 0 {
			r.problem(Problem{Kind: KindNeverRan, Packages: never, Message: fmt.Sprintf("%s never ran: %s", plural(len(never), "package"), listOf(never))})
		}
		if len(twice) > 0 {
			slices.Sort(twiceShards)
			r.problem(Problem{Kind: KindRanTwice, Shards: slices.Compact(twiceShards), Packages: twice, Message: fmt.Sprintf("%s ran twice: %s", plural(len(twice), "package"), listOf(twice))})
		}
		for _, rc := range present {
			var missing []string
			for _, p := range rc.Planned {
				if !slices.Contains(rc.Reported, p) {
					missing = append(missing, p)
				}
			}
			if len(missing) > 0 {
				r.problem(Problem{Kind: KindPlannedNotReported, Shards: []int{rc.Index}, Packages: missing, Message: fmt.Sprintf("shard %d planned but never reported: %s", rc.Index, listOf(missing))})
			}
		}
	}

	// 6. the tests gate
	for _, rc := range present {
		if rc.Passed {
			continue
		}
		pkgs := slices.Clone(rc.FailedPkgs)
		for _, f := range rc.Failures {
			pkgs = append(pkgs, f.Package)
		}
		slices.Sort(pkgs)
		pkgs = slices.Compact(pkgs)
		r.problem(Problem{Kind: KindShardFailed, Shards: []int{rc.Index}, Packages: pkgs, Message: fmt.Sprintf("shard %d/%d failed: %s in %s", rc.Index, r.Total, plural(rc.Tests.Failed, "failed test"), plural(len(pkgs), "package"))})
	}

	var versions []string
	for _, rc := range present {
		if in.CanopyVersion != "" && rc.CanopyVersion != in.CanopyVersion && !slices.Contains(versions, rc.CanopyVersion) {
			versions = append(versions, rc.CanopyVersion)
		}
	}
	if len(versions) > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("join is canopy %s but shards ran %s; pin the same version on every job", in.CanopyVersion, strings.Join(versions, ", ")))
	}

	if err := r.coverage(in, present); err != nil {
		return err
	}
	merged, err := r.metrics(in, present, old, units)
	if err != nil {
		return err
	}
	p := in.P
	if p < 1 {
		for _, rc := range present {
			p = max(p, rc.Runner.CPUs)
		}
	}
	r.Suggestion = suggest(merged, units, r.Total, in.Overhead, p)
	return nil
}

func (r *Report) problem(p Problem) {
	switch kindExit[p.Kind] {
	case ExitTestsFailed:
		p.Check = CheckTests
	case ExitGateFailed:
		p.Check = CheckCoverage
	default:
		p.Check = CheckVerified
	}
	r.Problems = append(r.Problems, p)
}

func shardReport(rc *Receipt, old *Metrics, oldDigest string) ShardReport {
	w := rc.Inputs["weights"]
	res := &ShardResult{
		CanopyVersion: rc.CanopyVersion,
		Digest:        rc.Digest,
		Weights: ShardWeights{
			Source: w.Source, Measured: w.Measured, Estimated: w.Estimated,
			MetricsFile: rc.Metrics.File, Ignored: rc.Metrics.Ignored,
		},
		Planned:        nonNil(rc.Planned),
		Reported:       nonNil(rc.Reported),
		Passed:         rc.Passed,
		ElapsedMS:      rc.ElapsedMS,
		Tests:          rc.Tests,
		Failures:       nonNil(rc.Failures),
		FailedPackages: rc.FailedPkgs,
		Coverprofile:   rc.Coverprofile,
	}
	// ponytail: the receipt has no planned load, so the estimate is rebuilt from the metrics file the
	// shard read, and only when every planned package was measured. Record the load in the receipt if
	// the est. column needs to cover estimated packages too.
	if w.Source == SourceMetrics && old != nil && oldDigest == rc.Metrics.File {
		var est int64
		ok := true
		for _, p := range rc.Planned {
			ms := old.Packages[p].MS
			if len(ms) == 0 {
				ok = false
				break
			}
			est += lowerMedian(ms)
		}
		if ok {
			res.EstimatedMS = &est
		}
	}
	return ShardReport{Index: rc.Index, Present: true, ShardResult: res}
}

func (r *Report) coverage(in JoinInput, present []*Receipt) error {
	c := &r.Checks.Coverage

	// gate precedence: the join's own value, else what the shards recorded (which must agree)
	recorded := map[string][]int{}
	var recordedVal *float64
	for _, rc := range present {
		k := "unset"
		if rc.Gates.CoverMin != nil {
			k, recordedVal = pct(*rc.Gates.CoverMin), rc.Gates.CoverMin
		}
		recorded[k] = append(recorded[k], rc.Index)
	}
	switch {
	case in.CoverMin != nil:
		c.Threshold, c.ThresholdSource = in.CoverMin, ThresholdJoin
		if len(recorded) == 1 && recordedVal != nil {
			c.ReceiptThreshold = recordedVal
			if *recordedVal != *in.CoverMin {
				r.Warnings = append(r.Warnings, fmt.Sprintf("coverage threshold %s set on the join overrides %s recorded by shards", pct(*in.CoverMin), pct(*recordedVal)))
			}
		}
	case len(recorded) > 1:
		var says []string
		for _, k := range slices.Sorted(maps.Keys(recorded)) {
			says = append(says, fmt.Sprintf("%s: %s", shardList(recorded[k]), k))
		}
		var all []int
		for _, rc := range present {
			all = append(all, rc.Index)
		}
		r.problem(Problem{Kind: KindGateConflict, Shards: all, Message: "shards disagree on covermin: " + strings.Join(says, ", ")})
	case recordedVal != nil:
		c.Threshold, c.ThresholdSource = recordedVal, ThresholdReceipts
	}

	var paths []string
	var without []int
	for _, rc := range present {
		switch {
		case rc.Coverprofile != "":
			paths = append(paths, filepath.Join(in.OutDir, rc.Coverprofile))
		case len(rc.Planned) > 0:
			without = append(without, rc.Index)
		}
	}
	c.Enabled = len(paths) > 0
	if c.Enabled && len(without) > 0 {
		r.problem(Problem{Kind: KindCoverageMissing, Shards: without, Message: fmt.Sprintf("coverage enabled on some shards but not others (none from %s)", shardList(without))})
	}
	if c.Enabled {
		profiles, err := MergeCoverProfiles(paths)
		if err != nil {
			r.problem(Problem{Kind: KindCoverageMissing, Message: fmt.Sprintf("unable to merge coverage: %v", err)})
		} else {
			c.Profile = filepath.Join(in.ShardDir, "coverage.out")
			if err := WriteCoverProfile(c.Profile, profiles); err != nil {
				return fmt.Errorf("unable to write merged coverage: %w", err)
			}
			percent := CoveragePercent(profiles)
			c.Percent = &percent
		}
	}

	switch {
	case c.Threshold == nil:
	case !c.Enabled:
		r.problem(Problem{Kind: KindCoverageMissing, Message: fmt.Sprintf("coverage threshold %s set but no shard collected coverage", pct(*c.Threshold))})
	case c.Percent != nil && *c.Percent < *c.Threshold:
		// the same message canopy test uses
		r.problem(Problem{Kind: KindCoverageBelow, Message: fmt.Sprintf("coverage below threshold: %2.2f%% < %2.2f%%", *c.Percent, *c.Threshold)})
	}
	return nil
}

// metrics folds the shards' observations into the metrics file and returns the merged metrics (nil
// when the shards disagree on env or profile).
func (r *Report) metrics(in JoinInput, present []*Receipt, old *Metrics, units []string) (*Metrics, error) {
	m := &r.Checks.Metrics
	obs := make([]Observations, 0, len(present))
	fresh := map[string]bool{}
	for _, rc := range present {
		obs = append(obs, Observations{Env: rc.Metrics.Env, Profile: rc.Metrics.Profile, CPUs: rc.Runner.CPUs, MS: rc.Observations})
		for p := range rc.Observations {
			fresh[p] = true
		}
	}
	m.Packages = len(fresh)

	merged, warn := Merge(old, obs, units)
	m.Warning = warn
	m.OK = warn == ""
	if merged != nil && in.WriteMetrics {
		m.Path = filepath.Join(in.ShardDir, "metrics.json")
		if err := WriteMetrics(m.Path, merged); err != nil {
			return merged, fmt.Errorf("unable to write metrics: %w", err)
		}
		m.Written = true
	}
	return merged, nil
}

// suggest weighs units by their lower median in m. Packages without samples are assumed to take
// the median of the measured ones; with nothing measured only package counts are shown.
func suggest(m *Metrics, units []string, total int, overhead time.Duration, p int) *Suggestion {
	if len(units) == 0 {
		return nil
	}
	s := &Suggestion{Current: total, Estimates: []SuggestionEstimate{}}
	var measured []int64
	weights := map[string]int64{}
	for _, u := range units {
		if m != nil && len(m.Packages[u].MS) > 0 {
			weights[u] = lowerMedian(m.Packages[u].MS)
			measured = append(measured, weights[u])
			if weights[u] > s.SlowestMS {
				s.Slowest, s.SlowestMS = u, weights[u]
			}
		}
	}

	fill := int64(1)
	switch {
	case len(measured) == 0:
		s.Static, s.Note = true, StaticSuggestionNote
	case len(measured) < len(units):
		fill = lowerMedian(measured)
		s.Note = fmt.Sprintf("%s without timing data assumed to take the median package time", plural(len(units)-len(measured), "package"))
	}
	us := make([]Unit, 0, len(units))
	for _, u := range units {
		w, ok := weights[u]
		if !ok {
			w = fill
		}
		us = append(us, Unit{Package: u, Weight: w, Estimated: !ok})
	}

	var table []Estimate
	if !s.Static {
		s.Best, table = Suggest(us, overhead, p)
	}
	for n := 1; n <= min(maxSuggestedShards, len(us)); n++ {
		e := SuggestionEstimate{Shards: n}
		for _, pkgs := range NewPlan(us, n).Shards {
			e.Packages = append(e.Packages, len(pkgs))
		}
		if table != nil {
			e.WallMS, e.RunnerMS = table[n-1].Wall.Milliseconds(), table[n-1].RunnerTime.Milliseconds()
		}
		s.Estimates = append(s.Estimates, e)
	}
	return s
}

// inputMismatches explains a digest mismatch group by group.
func inputMismatches(present []*Receipt) []Problem {
	var out []Problem
	for _, g := range (Inputs{}).Groups() {
		digests := map[string]bool{}
		for _, rc := range present {
			digests[rc.Inputs[g.Name].Digest] = true
		}
		if len(digests) < 2 {
			continue
		}
		lines := map[int][]string{}
		for _, rc := range present {
			lines[rc.Index] = groupLines(g.Name, rc)
		}
		d := DiffLines(lines)
		msg := describeDiff(d)
		if g.Name == "weights" {
			msg = describeWeights(present)
		}
		out = append(out, Problem{Kind: KindInputMismatch, Shards: d.Differing, Group: g.Name, LineDiff: &d, Hint: groupHints[g.Name], Message: fmt.Sprintf("[%s] %s", g.Name, msg)})
	}
	if len(out) == 0 {
		// the overall digests differ but no group does (receipts without inputs)
		var differing []int
		for _, rc := range present[1:] {
			if rc.Digest != present[0].Digest {
				differing = append(differing, rc.Index)
			}
		}
		out = append(out, Problem{Kind: KindInputMismatch, Shards: differing, Message: "shards ran with different inputs"})
	}
	return out
}

// groupLines is what a receipt says about one input group. Receipts leave out the selection
// package lines and the weight lines, so those come from units and the weights summary instead.
func groupLines(group string, rc *Receipt) []string {
	in := rc.Inputs[group]
	switch group {
	case "selection":
		lines := slices.Clone(in.Lines)
		for _, u := range rc.Units {
			lines = append(lines, "package "+u)
		}
		return lines
	case "weights":
		file := rc.Metrics.File
		if rc.Metrics.Ignored != "" {
			file = "ignored: " + rc.Metrics.Ignored
		}
		return []string{
			"source " + in.Source,
			"metrics " + file,
			fmt.Sprintf("measured %d", in.Measured),
			fmt.Sprintf("estimated %d", in.Estimated),
		}
	}
	return in.Lines
}

// describeDiff renders a LineDiff as `shard 3 only: ...` and `shards 1,2,4: key a / shard 3: key b`
// pieces. Differing package lines are summarized rather than listed.
func describeDiff(d LineDiff) string {
	var pieces []string

	// invert only_in to find which shards share each line
	holders := map[string][]int{}
	for i, ls := range d.OnlyIn {
		for _, l := range ls {
			holders[l] = append(holders[l], i)
		}
	}
	type side struct {
		shards     []int
		lines, pkg []string
	}
	sides := map[string]*side{}
	for _, l := range slices.Sorted(maps.Keys(holders)) {
		sh := holders[l]
		slices.Sort(sh)
		k := fmt.Sprint(sh)
		if sides[k] == nil {
			sides[k] = &side{shards: sh}
		}
		if p, ok := strings.CutPrefix(l, "package "); ok {
			sides[k].pkg = append(sides[k].pkg, p)
		} else {
			sides[k].lines = append(sides[k].lines, l)
		}
	}
	ordered := slices.SortedFunc(maps.Values(sides), func(a, b *side) int { return slices.Compare(a.shards, b.shards) })
	for _, s := range ordered {
		if len(s.lines) > 0 {
			pieces = append(pieces, fmt.Sprintf("%s only: %s", shardList(s.shards), strings.Join(s.lines, ", ")))
		}
		if len(s.pkg) > 0 {
			verb := "has"
			if len(s.shards) > 1 {
				verb = "have"
			}
			pieces = append(pieces, fmt.Sprintf("%s %s %s the others don't: %s", shardList(s.shards), verb, plural(len(s.pkg), "package"), listOf(s.pkg)))
		}
	}

	for _, k := range slices.Sorted(maps.Keys(d.Values)) {
		byValue := d.Values[k]
		vals := slices.SortedFunc(maps.Keys(byValue), func(a, b string) int { return byValue[a][0] - byValue[b][0] })
		var alts []string
		for _, v := range vals {
			alts = append(alts, fmt.Sprintf("%s: %s %s", shardList(byValue[v]), k, v))
		}
		pieces = append(pieces, strings.Join(alts, " / "))
	}
	return strings.Join(pieces, "; ")
}

// describeWeights groups shards by where their weights came from, instead of listing weight lines.
func describeWeights(present []*Receipt) string {
	var order []string
	groups := map[string][]int{}
	for _, rc := range present {
		w := rc.Inputs["weights"]
		desc := w.Source
		switch {
		case rc.Metrics.Ignored != "":
			desc += " (" + rc.Metrics.Ignored + ")"
		case w.Source == SourceMetrics:
			desc = fmt.Sprintf("%s %s (%d measured, %d estimated)", desc, shortDigest(rc.Metrics.File), w.Measured, w.Estimated)
		}
		if groups[desc] == nil {
			order = append(order, desc)
		}
		groups[desc] = append(groups[desc], rc.Index)
	}
	var alts []string
	for _, desc := range order {
		alts = append(alts, fmt.Sprintf("%s: %s", shardList(groups[desc]), desc))
	}
	return strings.Join(alts, " / ")
}

func unionUnits(present []*Receipt) []string {
	var units []string
	for _, rc := range present {
		units = append(units, rc.Units...)
	}
	slices.Sort(units)
	return slices.Compact(units)
}

// shardList renders "shard 3" or "shards 1,2,4".
func shardList(idx []int) string {
	s := make([]string, len(idx))
	for i, v := range idx {
		s[i] = strconv.Itoa(v)
	}
	if len(idx) == 1 {
		return "shard " + s[0]
	}
	return "shards " + strings.Join(s, ",")
}

// listOf shows the first few items and how many more there are.
func listOf(items []string) string {
	const show = 3
	if len(items) <= show {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d)", strings.Join(items[:show], ", "), len(items)-show)
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func pct(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64) + "%"
}

func shortDigest(d string) string {
	if len(d) > len("sha256:")+8 {
		return d[:len("sha256:")+8]
	}
	return d
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
