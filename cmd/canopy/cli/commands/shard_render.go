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

// renderJoinText writes the join report as plain text in the same PASS/FAIL style as the go test footer.
func renderJoinText(w io.Writer, r shard.Report, color bool) error {
	var b strings.Builder
	st := style.NewGo(color)

	b.WriteString(st.Bold.Render(joinHeader(r)) + "\n\n")
	writeJoinTable(&b, st, r)
	b.WriteString("\n")

	// a passing tests check is implied by the PASS footer, so only failures get a line
	if !r.Checks.Tests.OK {
		failures, pkgs := joinFailures(r)
		var failed []string
		for _, f := range failures {
			failed = append(failed, fmt.Sprintf("shard %s  %s  %s", f.shard, f.pkg, f.test))
		}
		msg := joinProblemMessage(r, shard.CheckTests)
		if r.Checks.Tests.Failed > 0 {
			msg = fmt.Sprintf("%s in %s", countOf(r.Checks.Tests.Failed, "failed test"), countOf(len(pkgs), "package"))
		}
		writeJoinCheck(&b, st, false, "tests", msg, failed, "")
	}
	writeJoinVerified(&b, st, r)
	joinCoverage(r, func(ok bool, msg string) { writeJoinCheck(&b, st, ok, "coverage", msg, nil, "") })
	if m := r.Checks.Metrics; m.Written {
		writeJoinCheck(&b, st, true, "metrics", fmt.Sprintf("%s updated in %s", countOf(m.Packages, "package"), m.Path), nil, "")
	} else if m.Warning != "" {
		writeJoinCheck(&b, st, true, "metrics", "not updated: "+m.Warning, nil, "")
	}

	writeJoinFooter(&b, st, r)

	if len(r.Warnings) > 0 {
		b.WriteString("\n")
	}
	for _, warn := range r.Warnings {
		fmt.Fprintf(&b, "%s %s\n", st.Skipped.Render("warning:"), warn)
	}
	if s := r.Suggestion; s != nil {
		b.WriteString("\n" + st.Aux.Render("suggestion: "+suggestionText(s)) + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func joinHeader(r shard.Report) string {
	header := fmt.Sprintf("canopy shard join: %s, %s", countOf(r.Total, "shard"), countOf(r.Packages, "package"))
	switch {
	case len(r.Digests) == 1:
		header += ", inputs " + shortDigest(r.Digests[0].Digest)
		if src := joinWeightSource(r); src != "" {
			header += " (weights from " + weightsLabel(src) + ")"
		}
	case len(r.Digests) > 1:
		header += ", inputs differ between shards"
	}
	return header
}

// writeJoinTable writes the shard table. The last column is never padded, so styled cells don't affect widths.
func writeJoinTable(b *strings.Builder, st style.Go, r shard.Report) {
	withEst := joinHasEstimates(r)
	head := []string{"shard", colPkgs}
	if withEst {
		head = append(head, "est.")
	}
	rows := [][]string{append(head, "actual", "tests", "result")}
	for _, s := range r.Shards {
		label := fmt.Sprintf("%d/%d", s.Index, r.Total)
		row := []string{label, "-"}
		if withEst {
			row = append(row, "-")
		}
		if !s.Present {
			rows = append(rows, append(row, "-", "missing", st.Failed.Render("FAIL")))
			continue
		}
		row[1] = fmt.Sprint(len(s.Planned))
		if s.EstimatedMS != nil && withEst {
			row[2] = fmtMS(*s.EstimatedMS)
		}
		result := st.Success.Render("ok")
		if !s.Passed {
			result = st.Failed.Render("FAIL")
		}
		rows = append(rows, append(row, fmtMS(s.ElapsedMS), tallyText(s.Tests), result))
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, c := range row[:len(row)-1] {
			widths[i] = max(widths[i], len(c))
		}
	}
	for i, row := range rows {
		line := ""
		for j, c := range row {
			if j < len(row)-1 {
				c += strings.Repeat(" ", widths[j]-len(c))
			}
			line += "  " + c
		}
		if i == 0 {
			line = st.Aux.Render(line)
		}
		b.WriteString(line + "\n")
	}
}

// writeJoinCheck writes one `ok`/`FAIL` check line, its tree of details and a note below them.
func writeJoinCheck(b *strings.Builder, st style.Go, ok bool, name, msg string, details []string, note string) {
	status := st.Success.Render("ok") + "    "
	if !ok {
		status = st.Failed.Render("FAIL") + "  "
	}
	fmt.Fprintf(b, "%s  %-11s%s\n", status, name, msg)
	for i, d := range details {
		glyph := "├─"
		if i == len(details)-1 {
			glyph = "└─"
		}
		fmt.Fprintf(b, "        %s %s\n", st.Aux.Render(glyph), d)
	}
	if note != "" {
		fmt.Fprintf(b, "           %s\n", st.Aux.Render(note))
	}
}

// writeJoinVerified writes the verified check: one line per problem, or the ok line.
func writeJoinVerified(b *strings.Builder, st style.Go, r shard.Report) {
	verified := true
	for _, p := range r.Problems {
		if p.Check != shard.CheckVerified {
			continue
		}
		verified = false
		title, details := p.Message, []string(nil)
		if p.Kind == shard.KindInputMismatch && p.Group != "" && p.LineDiff != nil {
			title = "shards ran with different inputs"
			if p.Group == shard.GroupWeights {
				title, details = "shards computed different plans", weightDetails(r)
			} else {
				for _, d := range p.Describe() {
					details = append(details, "["+p.Group+"] "+d)
				}
			}
		}
		writeJoinCheck(b, st, false, "verified", title, details, p.Hint)
	}
	if verified {
		v := r.Checks.Verified
		writeJoinCheck(b, st, true, "verified", fmt.Sprintf("every package ran exactly once (%d of %d, %d of %d receipts, %s)", v.RanOnce, r.Packages, v.Receipts, r.Total, countOf(len(r.Digests), "plan")), nil, "")
	}
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

// writeJoinFooter writes the PASS/FAIL line, and where to find the output of failed shards.
func writeJoinFooter(b *strings.Builder, st style.Go, r shard.Report) {
	counts := []string{fmt.Sprintf("%d passed", r.Checks.Tests.Passed)}
	if n := r.Checks.Tests.Failed; n > 0 {
		counts = append(counts, fmt.Sprintf("%d failed", n))
	}
	if n := r.Checks.Tests.Skipped; n > 0 {
		counts = append(counts, fmt.Sprintf("%d skipped", n))
	}
	status := st.Success.Render("PASS")
	if r.Result != shard.ResultPass {
		status = st.Failed.Render("FAIL")
	}
	var slowest int64
	for _, s := range r.Shards {
		if s.Present {
			slowest = max(slowest, s.ElapsedMS)
		}
	}
	aux := fmtMS(slowest) + " slowest shard"
	if p := r.Checks.Coverage.Percent; p != nil {
		aux += "   " + pct(*p) + " covered"
	}
	fmt.Fprintf(b, "\n%s    %s tests   %s\n", status, strings.Join(counts, " / "), st.Aux.Render(aux))
	if r.Checks.Tests.OK {
		return
	}
	var names []string
	for _, p := range r.Problems {
		if p.Kind == shard.KindShardFailed {
			for _, i := range p.Shards {
				names = append(names, fmt.Sprintf("%d/%d", i, r.Total))
			}
		}
	}
	if len(names) > 0 {
		where := "the shard " + strings.Join(names, ", ") + " job log"
		if len(names) > 1 {
			where += "s"
		}
		fmt.Fprintf(b, "        %s failure output is in %s\n", st.Aux.Render("└─"), where)
	}
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

	writeJoinMarkdownTable(&b, r, icon)
	b.WriteString("\n")

	if !r.Checks.Tests.OK {
		failures, _ := joinFailures(r)
		for _, f := range failures {
			fmt.Fprintf(&b, "- ❌ **tests**: `%s` in `%s` (shard %s)\n", f.test, f.pkg, f.shard)
		}
		if len(failures) == 0 {
			fmt.Fprintf(&b, "- ❌ **tests**: %s\n", joinProblemMessage(r, shard.CheckTests))
		}
	}
	verified := true
	for _, p := range r.Problems {
		if p.Check != shard.CheckVerified {
			continue
		}
		verified = false
		line := fmt.Sprintf("- ❌ **verified**: %s", p.Message)
		if p.Hint != "" {
			line += " (" + p.Hint + ")"
		}
		b.WriteString(line + "\n")
	}
	if verified {
		fmt.Fprintf(&b, "- ✅ **verified**: %d of %d packages ran exactly once\n", r.Checks.Verified.RanOnce, r.Packages)
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
		cells = append(cells, "-", "missing", "❌")
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

// joinHasEstimates is true when any shard planned with metrics and has an estimated load.
func joinHasEstimates(r shard.Report) bool {
	return slices.ContainsFunc(r.Shards, func(s shard.ReportShard) bool { return s.Present && s.EstimatedMS != nil })
}

// joinWeightSource is the weights source every present shard agreed on, or empty.
func joinWeightSource(r shard.Report) string {
	var src string
	for _, s := range r.Shards {
		if !s.Present {
			continue
		}
		if src != "" && src != s.Weights.Source {
			return ""
		}
		src = s.Weights.Source
	}
	return src
}

// suggestionText looks like `4 shards is right (3 would be 4m23s, 4 is 3m32s, 5 is 3m29s wall)`.
func suggestionText(s *shard.Suggestion) string {
	if s.Static {
		return s.Note
	}
	verdict := countOf(s.Current, "shard") + " is right"
	if s.Best != s.Current {
		verdict = fmt.Sprintf("%s would be better than %d", countOf(s.Best, "shard"), s.Current)
	}
	var parts []string
	for _, e := range s.Estimates {
		if e.Shards == s.Current || (e.Shards >= s.Best-1 && e.Shards <= s.Best+1) {
			verb := "is"
			if e.Shards < s.Current {
				verb = "would be"
			}
			parts = append(parts, fmt.Sprintf("%d %s %s", e.Shards, verb, fmtMS(e.WallMS)))
		}
	}
	text := fmt.Sprintf("%s (%s wall)", verdict, strings.Join(parts, ", "))
	if s.Note != "" {
		text += "; " + s.Note
	}
	return text
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
