package shard

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func evenUnits(n int, weight int64) []Unit {
	var units []Unit
	for i := range n {
		units = append(units, Unit{Package: fmt.Sprintf("pkg/%02d", i), Weight: weight})
	}
	return units
}

func TestSuggest_SlowPackageSetsFloor(t *testing.T) {
	units := append(evenUnits(20, 100), Unit{Package: "slow", Weight: 10000})

	best, table := Suggest(units, 0, 1)

	require.Len(t, table, 16)
	assert.Equal(t, 12*time.Second, table[0].Wall)
	// from 2 shards on, slow sits alone and nothing gets faster
	for _, e := range table[1:] {
		assert.Equal(t, 10*time.Second, e.Wall, "n=%d", e.Shards)
	}
	assert.Equal(t, 2, best)
}

func TestSuggest_EvenPackagesKeepImproving(t *testing.T) {
	best, table := Suggest(evenUnits(16, 1000), 0, 1)

	require.Len(t, table, 16)
	for i := 1; i < len(table); i++ {
		assert.LessOrEqual(t, table[i].Wall, table[i-1].Wall, "n=%d", table[i].Shards)
	}
	assert.Equal(t, time.Second, table[15].Wall)
	assert.Equal(t, 16, best)
}

func TestSuggest_Parallelism(t *testing.T) {
	units := evenUnits(4, 1000)

	_, table := Suggest(units, 0, 4)
	assert.Equal(t, time.Second, table[0].Wall, "4 packages at once on one shard")

	_, table = Suggest(units, 0, 0)
	assert.Equal(t, 4*time.Second, table[0].Wall, "p below 1 means 1")
}

func TestSuggest_Overhead(t *testing.T) {
	best, table := Suggest(evenUnits(4, 1000), time.Minute, 1)

	// overhead dwarfs the load, so one shard is within 10% of the best
	assert.Equal(t, 1, best)
	assert.Equal(t, Estimate{Shards: 2, Wall: time.Minute + 2*time.Second, RunnerTime: 2*time.Minute + 4*time.Second}, table[1])
}

func TestSuggest_NoUnits(t *testing.T) {
	best, table := Suggest(nil, time.Minute, 1)
	assert.Zero(t, best)
	assert.Empty(t, table)
}
