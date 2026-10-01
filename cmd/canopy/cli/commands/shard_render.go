package commands

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

// labels the text and markdown tables and summaries share
const (
	colPkgs       = "pkgs"
	verdictFailed = "failed"
)

// renderJoinText writes the join report for a terminal: the verdict, what went wrong, the shards, and
// what the join did with the timings.
func renderJoinText(w io.Writer, r shard.Report, color bool) error {
	var b strings.Builder
	st := style.NewGo(color)

	b.WriteString(st.Bold.Render("canopy shard join") + "   " + st.Aux.Render(joinMeta(r)) + "\n\n")
	b.WriteString(joinVerdict(st, r) + "\n")
	for _, it := range joinProblems(r, packageTrimmer(r)) {
		b.WriteString("\n  " + st.Failed.Render("✗") + " " + it.title + "\n")
		for _, d := range it.details {
			b.WriteString("      " + d + "\n")
		}
		if it.hint != "" {
			b.WriteString("      " + st.Aux.Render("→ "+it.hint) + "\n")
		}
	}
	// the join stopped before any receipt was matched to a shard, so a table would only show placeholders
	if joinHasShards(r) {
		b.WriteString("\n")
		writeJoinTable(&b, st, r)
		writeJoinAftermath(&b, st, r)
	}
	if len(r.Warnings) > 0 {
		b.WriteString("\n")
	}
	for _, warn := range r.Warnings {
		b.WriteString("  " + st.Skipped.Render("!") + " " + warn + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// joinMeta is the header's `3 shards · 57 packages · inputs sha256:9f3c1a2b · exit 3`.
func joinMeta(r shard.Report) string {
	var parts []string
	if joinHasShards(r) {
		parts = append(parts, countOf(r.Total, "shard"), countOf(r.Packages, "package"))
	}
	if len(r.Digests) == 1 {
		parts = append(parts, "inputs "+shortDigest(r.Digests[0].Digest))
	}
	return strings.Join(append(parts, fmt.Sprintf("exit %d", r.ExitCode)), " · ")
}

// joinVerdict is the one line that answers whether the run can be trusted, and why not.
func joinVerdict(st style.Go, r shard.Report) string {
	t := r.Checks.Tests
	if r.Result == shard.ResultPass {
		parts := []string{countOf(t.Passed, "test") + " passed"}
		if t.Skipped > 0 {
			parts[0] += fmt.Sprintf(", %d skipped", t.Skipped)
		}
		parts = append(parts, "every package ran once")
		if c := r.Checks.Coverage; c.Percent != nil {
			cov := pct(*c.Percent) + " coverage"
			if c.Threshold != nil {
				cov += " (min " + pct(*c.Threshold) + ")"
			}
			parts = append(parts, cov)
		}
		return st.Success.Render("✓ PASSED") + "   " + strings.Join(parts, " · ")
	}

	tests, and := fmt.Sprintf("tests passed (%d)", t.Passed), "but"
	if !t.OK {
		_, pkgs := joinFailures(r)
		tests, and = fmt.Sprintf("%s failed in %s", countOf(t.Failed, "test"), countOf(len(pkgs), "package")), "and"
		if t.Failed == 0 {
			tests = countOf(len(pkgs), "package") + " failed"
		}
	}
	var head, why string
	switch r.ExitCode {
	case shard.ExitCannotRun:
		head, why = "CAN'T JOIN", "the shard receipts couldn't be read"
	case shard.ExitUnverified:
		head, why = "NOT VERIFIED", tests+", "+and+" the shards don't add up to one run"
		if !joinHasShards(r) {
			why = "the receipts don't come from one run"
		}
	case shard.ExitGateFailed:
		head, why = "GATE FAILED", tests+", "+and+" coverage failed its gate"
	default:
		head, why = "TESTS FAILED", fmt.Sprintf("%d passed, %d failed", t.Passed, t.Failed)
	}
	return st.Failed.Render("✗ "+head) + "   " + why
}

type joinItem struct {
	title   string
	details []string
	hint    string
}

// joinProblems is every problem as a titled item: verification first since it decides whether the
// rest means anything, then failed tests (one item for all shards), then coverage.
func joinProblems(r shard.Report, trim func(string) string) []joinItem {
	var out []joinItem
	for _, p := range r.Problems {
		if p.Check != shard.CheckVerified {
			continue
		}
		title, details := verifiedProblem(r, p)
		if p.Kind == shard.KindRanTwice && len(p.Shards) > 0 {
			title += ", in " + shardsLabel(p.Shards)
		}
		if len(p.Packages) > 0 {
			details = packageColumns(mapped(p.Packages, trim))
		}
		out = append(out, joinItem{title, details, p.Hint})
	}

	if !r.Checks.Tests.OK {
		failures, pkgs := joinFailures(r)
		it := joinItem{title: joinProblemMessage(r, shard.CheckTests)}
		if n := r.Checks.Tests.Failed; n > 0 {
			it.title = fmt.Sprintf("%s failed in %s", countOf(n, "test"), countOf(len(pkgs), "package"))
		}
		rows := make([][]string, 0, len(failures))
		for _, f := range failures {
			rows = append(rows, []string{f.shard, trim(f.pkg), f.test})
		}
		it.details = alignRows(rows)
		var names []string
		for _, p := range r.Problems {
			if p.Kind == shard.KindShardFailed {
				for _, i := range p.Shards {
					names = append(names, fmt.Sprintf("%d/%d", i, r.Total))
				}
			}
		}
		if len(names) > 0 {
			it.hint = "failure output is in the shard " + strings.Join(names, ", ") + " job log"
			if len(names) > 1 {
				it.hint += "s"
			}
		}
		out = append(out, it)
	}

	for _, p := range r.Problems {
		if p.Check == shard.CheckCoverage {
			out = append(out, joinItem{title: p.Message, hint: p.Hint})
		}
	}
	return out
}

// verifiedProblem is a verified problem's title, and for differing inputs the lines that differ.
func verifiedProblem(r shard.Report, p shard.Problem) (title string, details []string) {
	if p.Kind == shard.KindInputMismatch && p.Group != "" && p.LineDiff != nil {
		if p.Group == shard.GroupWeights {
			return "shards computed different plans", weightDetails(r)
		}
		for _, d := range p.Describe() {
			details = append(details, "["+p.Group+"] "+d)
		}
		return "shards ran with different inputs", details
	}
	return p.Message, nil
}

// writeJoinTable writes one row per shard, with the slowest one marked.
func writeJoinTable(b *strings.Builder, st style.Go, r shard.Report) {
	withEst := joinHasEstimates(r)
	head := []string{"shard", colPkgs}
	if withEst {
		head = append(head, "est.")
	}
	head = append(head, "time", "tests")

	slowest, present := -1, 0
	for i, s := range r.Shards {
		if s.Present {
			present++
			if slowest < 0 || s.ElapsedMS > r.Shards[slowest].ElapsedMS {
				slowest = i
			}
		}
	}

	rows, bad := [][]string{head}, []bool{false}
	for _, s := range r.Shards {
		row := []string{fmt.Sprintf("%d/%d", s.Index, r.Total), "-"}
		if withEst {
			row = append(row, "-")
		}
		if !s.Present {
			rows, bad = append(rows, append(row, "-", "missing")), append(bad, true)
			continue
		}
		row[1] = fmt.Sprint(len(s.Planned))
		if s.EstimatedMS != nil && withEst {
			row[2] = fmtMS(*s.EstimatedMS)
		}
		rows, bad = append(rows, append(row, fmtMS(s.ElapsedMS), tallyText(s.Tests))), append(bad, !s.Passed)
	}

	widths := make([]int, len(head))
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], len(c))
		}
	}
	last := len(head) - 1
	for i, row := range rows {
		line := "  " + row[0] + strings.Repeat(" ", widths[0]-len(row[0]))
		// numbers read best right aligned
		for j := 1; j < last; j++ {
			line += "   " + strings.Repeat(" ", widths[j]-len(row[j])) + row[j]
		}
		tests := row[last]
		if i > 0 && i-1 == slowest && present > 1 {
			tests += strings.Repeat(" ", widths[last]-len(tests)) + "   " + st.Aux.Render("← slowest")
		}
		switch {
		case i == 0:
			line = st.Aux.Render(line + "   " + tests)
		case bad[i]:
			line += "   " + st.Failed.Render(row[last]) + tests[len(row[last]):]
		default:
			line += "   " + tests
		}
		b.WriteString(line + "\n")
	}
}

// writeJoinAftermath writes what the join did with the shards' timings: the metrics file and the
// shard count suggestion. `–` marks something skipped on purpose.
func writeJoinAftermath(b *strings.Builder, st style.Go, r shard.Report) {
	type line struct{ glyph, label, note string }
	var lines []line
	switch m := r.Checks.Metrics; {
	case m.Written:
		lines = append(lines, line{st.Success.Render("✓"), "metrics saved for " + countOf(m.Packages, "package"), m.Path})
	case m.Warning != "":
		lines = append(lines, line{st.Aux.Render("–"), "metrics not saved", m.Warning})
	}
	var extra string
	switch s := r.Suggestion; {
	case !r.Checks.Verified.OK:
		lines = append(lines, line{st.Aux.Render("–"), "no shard count suggestion", "shards failed verification"})
	case s == nil:
	case s.Static:
		lines = append(lines, line{st.Aux.Render("–"), "no shard count suggestion", "no timing data yet"})
	case s.Best != s.Current:
		lines = append(lines, line{"→", suggestionVerdict(s), suggestionEstimates(s)})
		extra = s.Note
	}
	if len(lines) == 0 {
		return
	}
	w := 0
	for _, l := range lines {
		w = max(w, len(l.label))
	}
	b.WriteString("\n")
	for _, l := range lines {
		fmt.Fprintf(b, "  %s %-*s   %s\n", l.glyph, w, l.label, st.Aux.Render(l.note))
	}
	if extra != "" {
		b.WriteString("    " + st.Aux.Render(extra) + "\n")
	}
}

// suggestionVerdict is `4 shards would be ~6s faster`, or the same wall on fewer runners.
func suggestionVerdict(s *shard.Suggestion) string {
	if saved := suggestionWall(s, s.Current) - suggestionWall(s, s.Best); saved >= 1000 {
		return fmt.Sprintf("%s would be ~%s faster", countOf(s.Best, "shard"), fmtMS(saved))
	}
	return countOf(s.Best, "shard") + " would be as fast on fewer runners"
}

// suggestionEstimates is the est. wall around the current and best counts: `3: 22s  4: 16s  5: 15s`.
func suggestionEstimates(s *shard.Suggestion) string {
	var parts []string
	for _, e := range s.Estimates {
		if e.Shards == s.Current || (e.Shards >= s.Best-1 && e.Shards <= s.Best+1) {
			parts = append(parts, fmt.Sprintf("%d: %s", e.Shards, fmtMS(e.WallMS)))
		}
	}
	return strings.Join(parts, "  ")
}

// packageTrimmer drops the path prefix every package in the report shares (usually the module path).
func packageTrimmer(r shard.Report) func(string) string {
	var paths []string
	for _, s := range r.Shards {
		if s.Present {
			paths = append(append(paths, s.Planned...), s.Reported...)
		}
	}
	for _, p := range r.Problems {
		paths = append(paths, p.Packages...)
	}
	return prefixTrimmer(paths)
}

// prefixTrimmer drops the path prefix all paths share, always keeping at least the last element.
func prefixTrimmer(paths []string) func(string) string {
	var common []string
	for i, p := range paths {
		segs := strings.Split(p, "/")
		if i == 0 {
			common = segs[:len(segs)-1]
			continue
		}
		n := 0
		for n < len(common) && n < len(segs)-1 && common[n] == segs[n] {
			n++
		}
		common = common[:n]
	}
	if len(common) == 0 {
		return func(p string) string { return p }
	}
	prefix := strings.Join(common, "/") + "/"
	return func(p string) string { return strings.TrimPrefix(p, prefix) }
}

// packageColumns lays names out top to bottom in up to three columns, like ls.
func packageColumns(names []string) []string {
	w := 0
	for _, n := range names {
		w = max(w, len(n))
	}
	w += 2
	cols := max(1, min(3, 96/w))
	rows := (len(names) + cols - 1) / cols
	out := make([]string, rows)
	for i, n := range names {
		out[i%rows] += n + strings.Repeat(" ", w-len(n))
	}
	for i := range out {
		out[i] = strings.TrimRight(out[i], " ")
	}
	return out
}

// alignRows pads every column but the last to its widest cell.
func alignRows(rows [][]string) []string {
	var widths []int
	for _, row := range rows {
		for i, c := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(c))
		}
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		for j, c := range row {
			if j < len(row)-1 {
				c += strings.Repeat(" ", widths[j]-len(c)+2)
			}
			out[i] += c
		}
	}
	return out
}

func mapped(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

// weightDetails groups the present shards by how they weighted the packages and the inputs they
// ran with, one aligned line per group: `shards 1,2,4  weights from metrics (sha256:77d0e4c1)   inputs sha256:9f3c1a2b`.
func weightDetails(r shard.Report) []string {
	type variant struct {
		shards       []int
		desc, digest string
	}
	var variants []variant
	for _, s := range r.Shards {
		if !s.Present {
			continue
		}
		desc := "weights from " + weightsLabel(s.Weights.Source)
		switch {
		case s.Weights.Ignored != "":
			desc += " (" + s.Weights.Ignored + ")"
		case s.Weights.MetricsFile != "":
			desc += " (" + shortDigest(s.Weights.MetricsFile) + ")"
		}
		i := slices.IndexFunc(variants, func(v variant) bool { return v.desc == desc && v.digest == s.Digest })
		if i < 0 {
			variants = append(variants, variant{desc: desc, digest: s.Digest})
			i = len(variants) - 1
		}
		variants[i].shards = append(variants[i].shards, s.Index)
	}

	var labelW, descW int
	labels := make([]string, len(variants))
	for i, v := range variants {
		labels[i] = shardsLabel(v.shards)
		labelW, descW = max(labelW, len(labels[i])), max(descW, len(v.desc))
	}
	out := make([]string, len(variants))
	for i, v := range variants {
		out[i] = fmt.Sprintf("%-*s  %-*s   inputs %s", labelW, labels[i], descW, v.desc, shortDigest(v.digest))
	}
	return out
}

// shardsLabel renders "shard 3" or "shards 1,2,4".
func shardsLabel(idx []int) string {
	s := make([]string, len(idx))
	for i, v := range idx {
		s[i] = strconv.Itoa(v)
	}
	if len(idx) == 1 {
		return "shard " + s[0]
	}
	return "shards " + strings.Join(s, ",")
}

// weightsLabel names where weights came from: timings from the metrics file, or test counts.
func weightsLabel(source string) string {
	if source == shard.SourceStatic {
		return "test counts"
	}
	return source
}

// renderJoinMarkdown writes the join report for the GitHub step summary.
func renderJoinMarkdown(w io.Writer, r shard.Report) error {
	var b strings.Builder
	icon := func(ok bool) string {
		if ok {
			return "✅"
		}
		return "❌"
	}

	verdict := "passed"
	if r.Result != shard.ResultPass {
		verdict = verdictFailed
	}
	fmt.Fprintf(&b, "### canopy shard join: %s %s\n\n", icon(r.Result == shard.ResultPass), verdict)

	if joinHasShards(r) {
		writeJoinMarkdownTable(&b, r, icon)
		b.WriteString("\n")
	}

	if !r.Checks.Tests.OK {
		failures, _ := joinFailures(r)
		for _, f := range failures {
			fmt.Fprintf(&b, "- ❌ **tests**: `%s` in `%s` (shard %s)\n", f.test, f.pkg, f.shard)
		}
		if len(failures) == 0 {
			fmt.Fprintf(&b, "- ❌ **tests**: %s\n", joinProblemMessage(r, shard.CheckTests))
		}
	}
	if r.Checks.Verified.OK {
		fmt.Fprintf(&b, "- ✅ **verified**: %d of %d packages ran exactly once\n", r.Checks.Verified.RanOnce, r.Packages)
	} else {
		b.WriteString("- ❌ **verified**\n")
	}
	for _, p := range r.Problems {
		if p.Check != shard.CheckVerified {
			continue
		}
		line := "  - " + p.Message
		if p.Hint != "" {
			line += " (" + p.Hint + ")"
		}
		b.WriteString(line + "\n")
		for _, pkg := range p.Packages {
			fmt.Fprintf(&b, "    - `%s`\n", pkg)
		}
	}
	joinCoverage(r, func(ok bool, msg string) { fmt.Fprintf(&b, "- %s **coverage**: %s\n", icon(ok), msg) })
	for _, warn := range r.Warnings {
		fmt.Fprintf(&b, "- ⚠️ %s\n", warn)
	}

	if s := r.Suggestion; s != nil {
		writeJoinMarkdownSuggestion(&b, s)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// writeJoinMarkdownTable writes the shard table, with icon turning a verdict into an emoji.
func writeJoinMarkdownTable(b *strings.Builder, r shard.Report, icon func(bool) string) {
	withEst := joinHasEstimates(r)
	head, sep := "| shard | pkgs |", "|---|---|"
	if withEst {
		head, sep = head+" est. |", sep+"---|"
	}
	b.WriteString(head + " actual | tests | result |\n" + sep + "---|---|---|\n")
	for _, s := range r.Shards {
		cells := []string{fmt.Sprintf("%d/%d", s.Index, r.Total), "-"}
		if withEst {
			cells = append(cells, "-")
		}
		cells = append(cells, "-", "-", "❌ missing")
		if s.Present {
			cells[1] = fmt.Sprint(len(s.Planned))
			if s.EstimatedMS != nil && withEst {
				cells[2] = fmtMS(*s.EstimatedMS)
			}
			cells = append(cells[:len(cells)-3], fmtMS(s.ElapsedMS), tallyText(s.Tests), icon(s.Passed))
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
}

// writeJoinMarkdownSuggestion writes the shard count suggestion in a collapsed section.
func writeJoinMarkdownSuggestion(b *strings.Builder, s *shard.Suggestion) {
	b.WriteString("\n<details><summary>shard count suggestion</summary>\n\n")
	if s.Static {
		b.WriteString(s.Note + "\n")
	} else {
		b.WriteString("| shards | est. wall | runner time |\n|---|---|---|\n")
		for _, e := range s.Estimates {
			row := fmt.Sprintf("| %d | %s | %s |", e.Shards, fmtMS(e.WallMS), fmtMS(e.RunnerMS))
			if e.Shards == s.Best {
				row = fmt.Sprintf("| **%d** | **%s** | **%s** |", e.Shards, fmtMS(e.WallMS), fmtMS(e.RunnerMS))
			}
			b.WriteString(row + "\n")
		}
	}
	b.WriteString("\n</details>\n")
}

// joinCoverage reports the coverage check: each problem, or the merged percentage when coverage ran.
func joinCoverage(r shard.Report, emit func(ok bool, msg string)) {
	c := r.Checks.Coverage
	for _, p := range r.Problems {
		if p.Check == shard.CheckCoverage {
			emit(false, p.Message)
		}
	}
	if !c.OK || c.Percent == nil {
		return
	}
	msg := pct(*c.Percent)
	if c.Threshold != nil {
		// the join's own value can come from the flag, env or config
		source := map[string]string{
			shard.ThresholdJoin:     "test.covermin",
			shard.ThresholdReceipts: "test.covermin, recorded by shards",
		}[c.ThresholdSource]
		msg = fmt.Sprintf("%s >= %s (%s)", msg, pct(*c.Threshold), source)
	}
	emit(true, msg)
}

type joinFailure struct{ shard, pkg, test string }

// joinFailures lists failed tests, and the distinct failed packages.
func joinFailures(r shard.Report) (out []joinFailure, pkgs []string) {
	for _, s := range r.Shards {
		if !s.Present {
			continue
		}
		label := fmt.Sprintf("%d/%d", s.Index, r.Total)
		for _, f := range s.Failures {
			out = append(out, joinFailure{label, f.Package, f.Test})
			pkgs = append(pkgs, f.Package)
		}
		// packages that failed without a failing test (build or setup failures)
		for _, p := range s.FailedPackages {
			out = append(out, joinFailure{label, p, "(package failed)"})
			pkgs = append(pkgs, p)
		}
	}
	slices.Sort(pkgs)
	return out, slices.Compact(pkgs)
}

func joinProblemMessage(r shard.Report, check string) string {
	for _, p := range r.Problems {
		if p.Check == check {
			return p.Message
		}
	}
	return ""
}

// joinHasShards is true when at least one receipt was matched to a shard. It's false when the join
// stopped early (unreadable receipts, or receipts from different runs), leaving only empty placeholders.
func joinHasShards(r shard.Report) bool {
	return slices.ContainsFunc(r.Shards, func(s shard.ReportShard) bool { return s.Present })
}

// joinHasEstimates is true when any shard planned with metrics and has an estimated load.
func joinHasEstimates(r shard.Report) bool {
	return slices.ContainsFunc(r.Shards, func(s shard.ReportShard) bool { return s.Present && s.EstimatedMS != nil })
}

func tallyText(t shard.TestTally) string {
	out := fmt.Sprintf("%d passed", t.Passed)
	if t.Failed > 0 {
		out += fmt.Sprintf(" / %d failed", t.Failed)
	}
	if t.Skipped > 0 {
		out += fmt.Sprintf(" / %d skipped", t.Skipped)
	}
	return out
}

// fmtMS renders a duration like 3m02s, or 48s under a minute.
func fmtMS(ms int64) string {
	d := (time.Duration(ms) * time.Millisecond).Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func pct(v float64) string { return fmt.Sprintf("%.1f%%", v) }

func countOf(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
