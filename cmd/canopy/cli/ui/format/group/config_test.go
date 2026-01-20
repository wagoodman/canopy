package group

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_ShouldGroup(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		passed bool
		want   bool
	}{
		{
			name:   "passed with GroupPassed true and formatter set",
			config: Config{Formatter: GitHub, GroupPassed: true},
			passed: true,
			want:   true,
		},
		{
			name:   "passed with GroupPassed false",
			config: Config{Formatter: GitHub, GroupPassed: false},
			passed: true,
			want:   false,
		},
		{
			name:   "failed with GroupFailed true and formatter set",
			config: Config{Formatter: GitHub, GroupFailed: true},
			passed: false,
			want:   true,
		},
		{
			name:   "failed with GroupFailed false",
			config: Config{Formatter: GitHub, GroupFailed: false},
			passed: false,
			want:   false,
		},
		{
			name:   "nil formatter returns false even with GroupPassed true",
			config: Config{Formatter: nil, GroupPassed: true},
			passed: true,
			want:   false,
		},
		{
			name:   "nil formatter returns false even with GroupFailed true",
			config: Config{Formatter: nil, GroupFailed: true},
			passed: false,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldGroup(tt.passed)
			assert.Equal(t, tt.want, got)
		})
	}
}
