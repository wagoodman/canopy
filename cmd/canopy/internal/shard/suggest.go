package shard

import "time"

const maxSuggestedShards = 16

// StaticSuggestionNote is shown instead of time estimates when no package was measured.
const StaticSuggestionNote = "no timing data yet; showing package counts only"

// Estimate is the predicted cost of running the units across a given number of shards.
type Estimate struct {
	Shards     int
	Wall       time.Duration // overhead + slowest shard's estimated wall
	RunnerTime time.Duration // n*overhead + total load
}

// Suggest estimates wall and runner time for 1 to min(16, len(units)) shards, where weights are ms,
// overhead is the fixed per-job cost, and p is how many packages go test runs at once (values
// below 1 mean 1). best is the fewest shards whose wall time is within 10% of the fastest.
func Suggest(units []Unit, overhead time.Duration, p int) (best int, table []Estimate) {
	p = max(p, 1)
	var total int64
	weight := map[string]int64{}
	for _, u := range units {
		total += u.Weight
		weight[u.Package] = u.Weight
	}

	var fastest time.Duration
	for n := 1; n <= min(maxSuggestedShards, len(units)); n++ {
		plan := NewPlan(units, n)

		// ponytail: assumes perfect packing within a shard, a rough lower bound that's fine for comparing n
		var wall int64
		for i, pkgs := range plan.Shards {
			shardWall := plan.Loads[i] / int64(p)
			for _, pkg := range pkgs {
				shardWall = max(shardWall, weight[pkg])
			}
			wall = max(wall, shardWall)
		}

		e := Estimate{
			Shards:     n,
			Wall:       overhead + ms(wall),
			RunnerTime: time.Duration(n)*overhead + ms(total),
		}
		table = append(table, e)
		if n == 1 || e.Wall < fastest {
			fastest = e.Wall
		}
	}

	for _, e := range table {
		if e.Wall*10 <= fastest*11 {
			return e.Shards, table
		}
	}
	return 0, table
}

func ms(v int64) time.Duration {
	return time.Duration(v) * time.Millisecond
}
