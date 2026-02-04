package options

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "days",
			input: "30d",
			want:  30 * 24 * time.Hour,
		},
		{
			name:  "single day",
			input: "1d",
			want:  24 * time.Hour,
		},
		{
			name:  "go-style hours",
			input: "720h",
			want:  720 * time.Hour,
		},
		{
			name:  "go-style mixed",
			input: "1h30m",
			want:  time.Hour + 30*time.Minute,
		},
		{
			name:  "zero days",
			input: "0d",
			want:  0,
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:    "invalid string",
			input:   "abc",
			wantErr: require.Error,
		},
		{
			name:    "negative days",
			input:   "-5d",
			wantErr: require.Error,
		},
		{
			name:    "non-numeric days",
			input:   "foobard",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := ParseDuration(tt.input)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
