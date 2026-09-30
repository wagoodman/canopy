package shard

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlan_Greedy(t *testing.T) {
	units := []Unit{
		{Package: "p2", Weight: 2},
		{Package: "b", Weight: 3},
		{Package: "p5", Weight: 5},
		{Package: "a", Weight: 3},
		{Package: "p4", Weight: 4},
	}

	p := NewPlan(units, 2)

	// p5 -> 0, p4 -> 1, a (path before b) -> 1, b -> 0, p2 -> 1
	assert.Equal(t, [][]string{{"b", "p5"}, {"a", "p2", "p4"}}, p.Shards)
	assert.Equal(t, []int64{8, 9}, p.Loads)
	assert.Equal(t, []string{"a", "b", "p2", "p4", "p5"}, packages(p.Units))
}

func TestNewPlan_TieGoesToFewerUnits(t *testing.T) {
	units := []Unit{{Package: "A", Weight: 1}, {Package: "B", Weight: 1}, {Package: "C"}, {Package: "D"}}

	p := NewPlan(units, 2)

	// C ties on load and count so it takes the lower index; D then ties on load and goes to the shard with fewer units
	assert.Equal(t, [][]string{{"A", "C"}, {"B", "D"}}, p.Shards)
}

func TestNewPlan_Properties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for iter := range 500 {
		var units []Unit
		for i := range rng.Intn(40) {
			units = append(units, Unit{Package: fmt.Sprintf("pkg/%03d", i), Weight: rng.Int63n(20)})
		}
		total := 1 + rng.Intn(12)

		p := NewPlan(units, total)
		require.Len(t, p.Shards, total, "iter %d", iter)
		require.Len(t, p.Loads, total, "iter %d", iter)

		// exact cover with no overlap, and loads add up
		weights := map[string]int64{}
		for _, u := range units {
			weights[u.Package] = u.Weight
		}
		seen := map[string]int{}
		for i, s := range p.Shards {
			assert.True(t, slices.IsSorted(s), "iter %d: shard %d not sorted", iter, i)
			var load int64
			for _, pkg := range s {
				seen[pkg]++
				load += weights[pkg]
			}
			assert.Equal(t, load, p.Loads[i], "iter %d: shard %d load", iter, i)
		}
		require.Len(t, seen, len(units), "iter %d", iter)
		for pkg, n := range seen {
			assert.Equal(t, 1, n, "iter %d: %s assigned %d times", iter, pkg, n)
		}

		// same units, any order, same plan
		assert.Equal(t, p, NewPlan(units, total), "iter %d", iter)
		shuffled := slices.Clone(units)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		assert.Equal(t, p, NewPlan(shuffled, total), "iter %d", iter)

		// more shards than units leaves the extras empty
		if total >= len(units) {
			var empty int
			for _, s := range p.Shards {
				assert.LessOrEqual(t, len(s), 1, "iter %d", iter)
				if len(s) == 0 {
					empty++
				}
			}
			assert.Equal(t, total-len(units), empty, "iter %d", iter)
		}
	}
}

func TestPlanLines(t *testing.T) {
	assert.Equal(t, []string{"canopy v0.9.0", "planner 1", "total 4"}, PlanLines("v0.9.0", 4))
}

func packages(units []Unit) []string {
	var out []string
	for _, u := range units {
		out = append(out, u.Package)
	}
	return out
}
