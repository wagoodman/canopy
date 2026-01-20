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
			name:   "passed with GroupPassed true",
			config: Config{GroupPassed: true},
			passed: true,
			want:   true,
		},
		{
			name:   "passed with GroupPassed false",
			config: Config{GroupPassed: false},
			passed: true,
			want:   false,
		},
		{
			name:   "failed with GroupFailed true",
			config: Config{GroupFailed: true},
			passed: false,
			want:   true,
		},
		{
			name:   "failed with GroupFailed false",
			config: Config{GroupFailed: false},
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
