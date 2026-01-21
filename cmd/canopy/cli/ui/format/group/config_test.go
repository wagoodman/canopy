package group

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestConfig_ShouldGroup(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		action gotest.Action
		want   bool
	}{
		{
			name:   "passed with GroupPassed true and formatter set",
			config: Config{Formatter: GitHub, GroupPassed: true},
			action: gotest.PassAction,
			want:   true,
		},
		{
			name:   "passed with GroupPassed false",
			config: Config{Formatter: GitHub, GroupPassed: false},
			action: gotest.PassAction,
			want:   false,
		},
		{
			name:   "failed with GroupFailed true and formatter set",
			config: Config{Formatter: GitHub, GroupFailed: true},
			action: gotest.FailAction,
			want:   true,
		},
		{
			name:   "failed with GroupFailed false",
			config: Config{Formatter: GitHub, GroupFailed: false},
			action: gotest.FailAction,
			want:   false,
		},
		{
			name:   "skipped with GroupSkipped true and formatter set",
			config: Config{Formatter: GitHub, GroupSkipped: true},
			action: gotest.SkipAction,
			want:   true,
		},
		{
			name:   "skipped with GroupSkipped false",
			config: Config{Formatter: GitHub, GroupSkipped: false},
			action: gotest.SkipAction,
			want:   false,
		},
		{
			name:   "nil formatter returns false even with GroupPassed true",
			config: Config{Formatter: nil, GroupPassed: true},
			action: gotest.PassAction,
			want:   false,
		},
		{
			name:   "nil formatter returns false even with GroupFailed true",
			config: Config{Formatter: nil, GroupFailed: true},
			action: gotest.FailAction,
			want:   false,
		},
		{
			name:   "nil formatter returns false even with GroupSkipped true",
			config: Config{Formatter: nil, GroupSkipped: true},
			action: gotest.SkipAction,
			want:   false,
		},
		{
			name:   "unknown action returns false",
			config: Config{Formatter: GitHub, GroupPassed: true, GroupFailed: true, GroupSkipped: true},
			action: gotest.RunAction,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldGroup(tt.action)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_GroupedStatusLabel(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "only passed",
			config: Config{GroupPassed: true},
			want:   "passed",
		},
		{
			name:   "only failed",
			config: Config{GroupFailed: true},
			want:   "failed",
		},
		{
			name:   "only skipped",
			config: Config{GroupSkipped: true},
			want:   "skipped",
		},
		{
			name:   "passed and skipped",
			config: Config{GroupPassed: true, GroupSkipped: true},
			want:   "passed/skipped",
		},
		{
			name:   "passed and failed",
			config: Config{GroupPassed: true, GroupFailed: true},
			want:   "passed/failed",
		},
		{
			name:   "failed and skipped",
			config: Config{GroupFailed: true, GroupSkipped: true},
			want:   "failed/skipped",
		},
		{
			name:   "all three",
			config: Config{GroupPassed: true, GroupFailed: true, GroupSkipped: true},
			want:   "passed/failed/skipped",
		},
		{
			name:   "none enabled",
			config: Config{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GroupedStatusLabel()
			assert.Equal(t, tt.want, got)
		})
	}
}
