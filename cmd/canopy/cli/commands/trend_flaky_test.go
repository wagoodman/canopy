package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestTrendSpark(t *testing.T) {
	pass := gotest.PassAction
	fail := gotest.FailAction
	skip := gotest.SkipAction

	pad := func(bars string) string {
		return bars + strings.Repeat(" ", sparkWidth-len([]rune(bars)))
	}

	tests := []struct {
		name string
		seq  []gotest.Action
		want string
	}{
		{name: "empty", seq: nil, want: strings.Repeat(" ", sparkWidth)},
		{name: "all pass is a flat low line", seq: []gotest.Action{pass, pass, pass}, want: pad("▁▁▁")},
		{name: "all fail is a flat high line", seq: []gotest.Action{fail, fail, fail}, want: pad("███")},
		{name: "skips are ignored", seq: []gotest.Action{skip, fail, skip}, want: pad("█")},
		{
			// 40 runs, healthy then failing, bins into 20 slices: low half then high half
			name: "recent failures spike on the right",
			seq:  append(makeSeq(20, pass), makeSeq(20, fail)...),
			want: pad(strings.Repeat("▁", 10) + strings.Repeat("█", 10)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, trendSpark(tt.seq))
		})
	}
}

func makeSeq(n int, a gotest.Action) []gotest.Action {
	s := make([]gotest.Action, n)
	for i := range s {
		s[i] = a
	}
	return s
}
