package commands

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/shard"
)

// shardTrailer is the line under the footer of a sharded run.
func shardTrailer(sh *shardRuntime) string {
	s := fmt.Sprintf("shard %d/%d, %d of %d pkgs", sh.Index, sh.Total, len(sh.Planned()), len(sh.Plan.Units))
	if sh.Gates.CoverMin != nil {
		s += fmt.Sprintf(", coverage threshold %.1f%% deferred to `canopy shard join`", *sh.Gates.CoverMin)
	}
	return s
}

// shardHeaderTitle is the one-line plan summary (also what the log line carries).
func shardHeaderTitle(sh *shardRuntime) string {
	return fmt.Sprintf("shard %d/%d: %d of %d packages, weights from %s, inputs %s",
		sh.Index, sh.Total, len(sh.Planned()), len(sh.Plan.Units), weightsSummary(sh.Weights), shortDigest(sh.Digest))
}

// printShardHeader writes the plan header before the run starts. Inside CI it is a collapsed group,
// elsewhere it is the title followed by the body. It goes through the writer rather than the logger,
// which is invisible at info level.
func printShardHeader(w io.Writer, format group.Formatter, sh *shardRuntime) {
	title, body := shardHeaderTitle(sh), shardHeaderBody(sh)
	if format == nil {
		fmt.Fprintf(w, "%s\n%s", title, body)
		return
	}
	fmt.Fprint(w, format(title, body))
}

// printEmptyShard writes the result of a shard that was assigned no packages.
func printEmptyShard(w io.Writer, sh *shardRuntime, color bool) {
	st := style.NewGo(color)
	fmt.Fprintf(w, "%s\tno packages assigned to this shard\n", st.Success.Render("PASS"))
	fmt.Fprintf(w, "\t%s\n", st.Aux.Render("└─ "+shardTrailer(sh)))
}

func shardHeaderBody(sh *shardRuntime) string {
	var b strings.Builder
	row := func(label, value string) { fmt.Fprintf(&b, "    %-14s %s\n", label, value) }
	more := func(value string) { fmt.Fprintf(&b, "    %-14s %s\n", "", value) }

	row("shard source", shardSource(sh))

	w := sh.Weights
	if w.Source == shard.SourceMetrics {
		row("weights", fmt.Sprintf("metrics %s (%s), env %s, profile matches", sh.MetricsPath, shortDigest(sh.Metrics.File), sh.Metrics.Env))
		if w.Estimated > 0 {
			more(fmt.Sprintf("%s estimated from test counts at %s per test", countOf(w.Estimated, "package"), weightText(estimatedPerTest(sh))))
		}
	} else {
		row("weights", fmt.Sprintf("test counts (%s)", w.Ignored))
	}

	if cfg := shardConfigSummary(sh); cfg != "" {
		row("config", cfg)
	}

	var loads []string
	for _, l := range sh.Plan.Loads {
		loads = append(loads, loadText(w, l))
	}
	row("est. load", fmt.Sprintf("%s (shards: %s)", loadText(w, sh.Plan.Loads[sh.Index-1]), strings.Join(loads, " / ")))

	units := slices.Clone(sh.Plan.Units)
	planned := map[string]bool{}
	for _, p := range sh.Planned() {
		planned[p] = true
	}
	units = slices.DeleteFunc(units, func(u shard.Unit) bool { return !planned[u.Package] })
	slices.SortStableFunc(units, func(a, b shard.Unit) int { return cmp.Compare(b.Weight, a.Weight) })

	if len(units) > 0 {
		fmt.Fprintf(&b, "    packages\n")
	}
	for _, u := range units {
		var val, note string
		switch {
		case w.Source != shard.SourceMetrics:
			val = countOf(int(sh.Counts[u.Package]), "test")
		case u.Estimated:
			val, note = "~"+weightText(u.Weight), fmt.Sprintf("   (estimated, %s)", countOf(int(sh.Counts[u.Package]), "test"))
		default:
			val = weightText(u.Weight)
		}
		if w.Source == shard.SourceMetrics {
			fmt.Fprintf(&b, "      %7s  %s%s\n", val, u.Package, note)
		} else {
			fmt.Fprintf(&b, "      %-9s  %s\n", val, u.Package)
		}
	}
	return b.String()
}

func shardSource(sh *shardRuntime) string {
	switch {
	case sh.Auto && sh.From == "":
		return fmt.Sprintf("auto, no CI parallelism detected (%d/%d)", sh.Index, sh.Total)
	case sh.Auto:
		return fmt.Sprintf("%s (auto)", sh.From)
	case sh.From == "flag":
		return fmt.Sprintf("--shard %d/%d (flag)", sh.Index, sh.Total)
	}
	return fmt.Sprintf("%s=%d/%d (env)", options.ShardEnv, sh.Index, sh.Total)
}

// shardConfigSummary is the toolchain, flags, gate and commit the digest was computed from, as far as they are known.
func shardConfigSummary(sh *shardRuntime) string {
	var goVersion, goos, goarch, commit string
	for _, l := range sh.Inputs.Go {
		k, v, _ := strings.Cut(l, " ")
		switch k {
		case "goversion":
			goVersion = v
		case "goos":
			goos = v
		case "goarch":
			goarch = v
		}
	}
	var parts []string
	if goVersion != "" {
		parts = append(parts, fmt.Sprintf("%s %s/%s", goVersion, goos, goarch))
	}
	var flags []string
	for _, l := range sh.Inputs.Run {
		if k, v, _ := strings.Cut(l, " "); strings.HasSuffix(k, "-flag") {
			flags = append(flags, v)
		}
	}
	if len(flags) > 0 {
		parts = append(parts, strings.Join(flags, " "))
	}
	if sh.Gates.CoverMin != nil {
		parts = append(parts, fmt.Sprintf("covermin %g", *sh.Gates.CoverMin))
	}
	for _, l := range sh.Inputs.Source {
		if v, ok := strings.CutPrefix(l, "commit "); ok && v != "-" {
			commit = v[:min(len(v), 7)]
		}
	}
	if commit != "" {
		parts = append(parts, "commit "+commit)
	}
	return strings.Join(parts, ", ")
}

// estimatedPerTest is the ms per test that scaled the estimated packages, recovered from the plan.
func estimatedPerTest(sh *shardRuntime) int64 {
	var ms, tests int64
	for _, u := range sh.Plan.Units {
		if u.Estimated {
			ms += u.Weight
			tests += sh.Counts[u.Package] + 1
		}
	}
	if tests == 0 {
		return 0
	}
	return ms / tests
}

// weightText renders a metrics weight (ms) like 1m12s, 41s or 0.2s.
func weightText(ms int64) string {
	switch {
	case ms < 100:
		return fmt.Sprintf("%dms", ms)
	case ms < 10_000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmtMS(ms)
}

// loadText renders a shard load: a time for metrics weights, else the bare unit count.
func loadText(w shard.WeightResult, load int64) string {
	if w.Source == shard.SourceMetrics {
		return fmtMS(load)
	}
	return fmt.Sprintf("%d", load)
}

// writesJSONToStdout reports whether stdout carries json, which a plain-text header would corrupt.
func writesJSONToStdout(ws options.FormatWriters) bool {
	return slices.ContainsFunc(ws, func(w options.FormatWriter) bool { return w.PrimaryUI && w.Name == formatJSON })
}
