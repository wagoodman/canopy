package shard

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const inputsHeader = "canopy-shard-inputs v1"

// Inputs is everything that must match across shards, as groups of `key value` lines. The CLI fills
// it from the resolved test config. Lines must not contain newlines, and multi-valued lines (packages,
// weights) should be sorted so the text is identical on every machine.
type Inputs struct {
	Plan      []string // see PlanLines
	Selection []string
	Run       []string
	Gates     []string
	Go        []string
	Source    []string
	Weights   []string // see WeightLines
}

// Group is one named section of Inputs.
type Group struct {
	Name  string
	Lines []string
}

// Groups returns the groups in their fixed order.
func (in Inputs) Groups() []Group {
	return []Group{
		{"plan", in.Plan},
		{"selection", in.Selection},
		{"run", in.Run},
		{"gates", in.Gates},
		{"go", in.Go},
		{"source", in.Source},
		{"weights", in.Weights},
	}
}

// Text is the exact text the overall digest is computed over.
func (in Inputs) Text() string {
	var sb strings.Builder
	sb.WriteString(inputsHeader + "\n")
	for _, g := range in.Groups() {
		sb.WriteString(g.text())
	}
	return sb.String()
}

// Digest is the overall input digest, "sha256:<hex>".
func (in Inputs) Digest() string {
	return digest(in.Text())
}

// Digest is the digest of this group's section of the text, "sha256:<hex>".
func (g Group) Digest() string {
	return digest(g.text())
}

func (g Group) text() string {
	var sb strings.Builder
	sb.WriteString("[" + g.Name + "]\n")
	for _, l := range g.Lines {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}

func digest(s string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s)))
}

// WeightLines returns the [weights] group: the weight source (metrics or static), then one
// `weight <n> <pkg>` line per unit sorted by package, suffixed with ` est` for estimated weights.
func WeightLines(source string, units []Unit) []string {
	sorted := slices.Clone(units)
	slices.SortFunc(sorted, func(a, b Unit) int { return strings.Compare(a.Package, b.Package) })

	lines := []string{"source " + source}
	for _, u := range sorted {
		l := fmt.Sprintf("weight %d %s", u.Weight, u.Package)
		if u.Estimated {
			l += " est"
		}
		lines = append(lines, l)
	}
	return lines
}

// LineDiff explains how one input group differed between shards.
type LineDiff struct {
	// Baseline holds the shards whose lines a strict majority share. Empty when there's no majority.
	Baseline []int `json:"baseline"`
	// Differing holds every shard not in Baseline. Empty when all shards agree.
	Differing []int `json:"differing"`
	// OnlyIn maps a shard to its lines that not every shard has.
	OnlyIn map[int][]string `json:"only_in,omitempty"`
	// Values maps a single-valued key (one line per shard, e.g. goversion) to value -> shards.
	Values map[string]map[string][]int `json:"values,omitempty"`
}

// DiffLines compares one group's lines across shards (shard index -> lines).
func DiffLines(shards map[int][]string) LineDiff {
	indexes := slices.Sorted(maps.Keys(shards))

	// cluster shards with identical lines and look for a strict majority
	clusters := map[string][]int{}
	for _, i := range indexes {
		k := strings.Join(shards[i], "\n")
		clusters[k] = append(clusters[k], i)
	}
	var d LineDiff
	for _, c := range clusters {
		if len(c)*2 > len(indexes) {
			d.Baseline = c
		}
	}
	for _, i := range indexes {
		if !slices.Contains(d.Baseline, i) {
			d.Differing = append(d.Differing, i)
		}
	}
	if len(clusters) < 2 {
		return d
	}

	// count how many shards have each line and each key
	lineCount := map[string]int{}
	keyCount := map[string]map[int]int{}
	for _, i := range indexes {
		for _, l := range dedupe(shards[i]) {
			lineCount[l]++
		}
		for _, l := range shards[i] {
			k := key(l)
			if keyCount[k] == nil {
				keyCount[k] = map[int]int{}
			}
			keyCount[k][i]++
		}
	}
	singleValued := func(k string) bool {
		if len(keyCount[k]) != len(indexes) {
			return false
		}
		for _, n := range keyCount[k] {
			if n != 1 {
				return false
			}
		}
		return true
	}

	for _, i := range indexes {
		for _, l := range dedupe(shards[i]) {
			if lineCount[l] == len(indexes) {
				continue
			}
			k := key(l)
			if singleValued(k) {
				if d.Values == nil {
					d.Values = map[string]map[string][]int{}
				}
				if d.Values[k] == nil {
					d.Values[k] = map[string][]int{}
				}
				v := strings.TrimPrefix(strings.TrimPrefix(l, k), " ")
				d.Values[k][v] = append(d.Values[k][v], i)
				continue
			}
			if d.OnlyIn == nil {
				d.OnlyIn = map[int][]string{}
			}
			d.OnlyIn[i] = append(d.OnlyIn[i], l)
		}
	}
	return d
}

func key(line string) string {
	k, _, _ := strings.Cut(line, " ")
	return k
}

func dedupe(lines []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}
