package shard

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func sampleInputs() Inputs {
	return Inputs{
		Plan:      PlanLines("v0.9.0", 4),
		Selection: []string{"specifier ./...", "affected false"},
		Run:       []string{"test-flag -race", "cover true"},
		Gates:     []string{"covermin 80"},
		Go:        []string{"goversion go1.27.1", "goos linux"},
		Source:    []string{"commit abc", "dirty false"},
		Weights:   WeightLines("static", []Unit{{Package: "b", Weight: 2, Estimated: true}, {Package: "a", Weight: 7}}),
	}
}

func TestInputs_Text(t *testing.T) {
	in := sampleInputs()

	want := `canopy-shard-inputs v1
[plan]
canopy v0.9.0
planner 1
total 4
[selection]
specifier ./...
affected false
[run]
test-flag -race
cover true
[gates]
covermin 80
[go]
goversion go1.27.1
goos linux
[source]
commit abc
dirty false
[weights]
source static
weight 7 a
weight 2 b est
`
	assert.Equal(t, want, in.Text())
	assert.Equal(t, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(want))), in.Digest())
	assert.Equal(t, in.Digest(), sampleInputs().Digest())
}

func TestInputs_Digests(t *testing.T) {
	base := sampleInputs()

	// a line moving between groups changes the overall digest
	moved := sampleInputs()
	moved.Run = []string{"test-flag -race"}
	moved.Gates = []string{"cover true", "covermin 80"}
	assert.NotEqual(t, base.Digest(), moved.Digest())

	// group digests only move for the group that changed
	changed := sampleInputs()
	changed.Go[0] = "goversion go1.26.4"
	for i, g := range changed.Groups() {
		b := base.Groups()[i]
		if g.Name == "go" {
			assert.NotEqual(t, b.Digest(), g.Digest())
		} else {
			assert.Equal(t, b.Digest(), g.Digest(), g.Name)
		}
	}

	// the same lines under a different group name hash differently
	assert.NotEqual(t, Group{"run", []string{"x"}}.Digest(), Group{"gates", []string{"x"}}.Digest())
}

func TestWeightLines_OrderIndependent(t *testing.T) {
	a := []Unit{{Package: "x", Weight: 1}, {Package: "y", Weight: 2, Estimated: true}}
	b := []Unit{a[1], a[0]}

	assert.Equal(t, []string{"source metrics", "weight 1 x", "weight 2 y est"}, WeightLines("metrics", a))
	assert.Equal(t, WeightLines("metrics", a), WeightLines("metrics", b))
}

func TestDiffLines(t *testing.T) {
	tests := []struct {
		name   string
		shards map[int][]string
		want   LineDiff
	}{
		{
			name:   "all agree",
			shards: map[int][]string{1: {"cover true"}, 2: {"cover true"}},
			want:   LineDiff{Baseline: []int{1, 2}},
		},
		{
			name: "majority baseline with an extra line",
			shards: map[int][]string{
				1: {"test-flag -race"},
				2: {"test-flag -race", "test-flag -run=TestFoo"},
				3: {"test-flag -race"},
			},
			want: LineDiff{
				Baseline:  []int{1, 3},
				Differing: []int{2},
				OnlyIn:    map[int][]string{2: {"test-flag -run=TestFoo"}},
			},
		},
		{
			name: "baseline has a line the other lacks",
			shards: map[int][]string{
				1: {"test-flag -race", "cover true"},
				2: {"test-flag -race", "cover true"},
				3: {"cover true"},
			},
			want: LineDiff{
				Baseline:  []int{1, 2},
				Differing: []int{3},
				OnlyIn:    map[int][]string{1: {"test-flag -race"}, 2: {"test-flag -race"}},
			},
		},
		{
			name: "single valued key",
			shards: map[int][]string{
				1: {"goversion go1.27.1", "goos linux"},
				2: {"goversion go1.27.1", "goos linux"},
				3: {"goversion go1.26.4", "goos linux"},
				4: {"goversion go1.27.1", "goos linux"},
			},
			want: LineDiff{
				Baseline:  []int{1, 2, 4},
				Differing: []int{3},
				Values:    map[string]map[string][]int{"goversion": {"go1.27.1": {1, 2, 4}, "go1.26.4": {3}}},
			},
		},
		{
			name: "no majority, 2 vs 2",
			shards: map[int][]string{
				1: {"commit aaa"},
				2: {"commit bbb"},
				3: {"commit aaa"},
				4: {"commit bbb"},
			},
			want: LineDiff{
				Differing: []int{1, 2, 3, 4},
				Values:    map[string]map[string][]int{"commit": {"aaa": {1, 3}, "bbb": {2, 4}}},
			},
		},
		{
			name: "key missing on one shard is not single valued",
			shards: map[int][]string{
				1: {"env GOFLAGS=-mod=mod"},
				2: {"env GOFLAGS=-mod=mod"},
				3: {},
			},
			want: LineDiff{
				Baseline:  []int{1, 2},
				Differing: []int{3},
				OnlyIn:    map[int][]string{1: {"env GOFLAGS=-mod=mod"}, 2: {"env GOFLAGS=-mod=mod"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DiffLines(tt.shards))
		})
	}
}
