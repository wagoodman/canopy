package failure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceLocation_IsZero(t *testing.T) {
	tests := []struct {
		name     string
		location SourceLocation
		want     bool
	}{
		{
			name:     "empty location",
			location: SourceLocation{},
			want:     true,
		},
		{
			name: "file only",
			location: SourceLocation{
				File: "test.go",
			},
			want: false,
		},
		{
			name: "line only",
			location: SourceLocation{
				Line: 42,
			},
			want: false,
		},
		{
			name: "both set",
			location: SourceLocation{
				File: "test.go",
				Line: 42,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.location.IsZero()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFailureTypes(t *testing.T) {
	// verify all failure type constants are unique
	types := []Type{
		AssertionFailure,
		PanicFailure,
		DiffFailure,
		TimeoutFailure,
		UnknownFailure,
	}

	seen := make(map[Type]bool)
	for _, ft := range types {
		require.False(t, seen[ft], "duplicate failure type: %s", ft)
		seen[ft] = true
	}
}
