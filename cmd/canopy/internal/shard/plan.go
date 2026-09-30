// Package shard splits a package set across CI shards deterministically, records what each shard ran
// (receipts), and verifies and merges shard results in the join.
package shard

// Unit is one schedulable piece of work: a package and its estimated cost.
type Unit struct {
	Package   string // import path
	Weight    int64  // estimated ms with metrics, static count without (see weights.go)
	Estimated bool   // weight came from test counts, not a measurement
}

// Plan is the assignment of units to shards.
type Plan struct {
	Total  int
	Units  []Unit     // sorted by Package
	Shards [][]string // Shards[i] = import paths, sorted
	Loads  []int64
}
