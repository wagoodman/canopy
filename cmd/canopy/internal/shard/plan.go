// Package shard splits a package set across CI shards deterministically, records what each shard ran
// (receipts), and verifies and merges shard results in the join.
package shard

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// PlannerVersion is recorded in the [plan] input group. Bump it whenever NewPlan could assign
// the same units differently, so shards on different planners fail verification.
const PlannerVersion = 1

// Unit is one schedulable piece of work: a package and its estimated cost.
type Unit struct {
	Package   string `json:"package"`   // import path
	Weight    int64  `json:"weight"`    // estimated ms with metrics, static count without (see weights.go)
	Estimated bool   `json:"estimated"` // weight came from test counts, not a measurement
}

// Packages returns the import paths of units, in order.
func Packages(units []Unit) []string {
	out := make([]string, len(units))
	for i, u := range units {
		out[i] = u.Package
	}
	return out
}

// Plan is the assignment of units to shards.
type Plan struct {
	Total  int
	Units  []Unit     // sorted by Package
	Shards [][]string // Shards[i] = import paths, sorted
	Loads  []int64
}

// NewPlan splits units across total shards (total must be >= 1) with a greedy longest-first
// assignment. The result depends only on the set of units, never on their input order.
func NewPlan(units []Unit, total int) Plan {
	byPath := slices.Clone(units)
	slices.SortFunc(byPath, func(a, b Unit) int { return strings.Compare(a.Package, b.Package) })

	// heaviest first; the stable sort keeps path order among equal weights
	order := slices.Clone(byPath)
	slices.SortStableFunc(order, func(a, b Unit) int { return cmp.Compare(b.Weight, a.Weight) })

	p := Plan{
		Total:  total,
		Units:  byPath,
		Shards: make([][]string, total),
		Loads:  make([]int64, total),
	}
	for i := range p.Shards {
		p.Shards[i] = []string{}
	}

	for _, u := range order {
		// least loaded shard, ties to fewer units, then lower index
		best := 0
		for i := 1; i < total; i++ {
			if p.Loads[i] < p.Loads[best] || (p.Loads[i] == p.Loads[best] && len(p.Shards[i]) < len(p.Shards[best])) {
				best = i
			}
		}
		p.Shards[best] = append(p.Shards[best], u.Package)
		p.Loads[best] += u.Weight
	}

	for _, s := range p.Shards {
		slices.Sort(s)
	}
	return p
}

// PlanLines returns the [plan] input group for the given canopy version and shard total.
func PlanLines(canopyVersion string, total int) []string {
	return []string{
		"canopy " + canopyVersion,
		fmt.Sprintf("planner %d", PlannerVersion),
		fmt.Sprintf("total %d", total),
	}
}
