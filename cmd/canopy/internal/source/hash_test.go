package source

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/require"
)

func TestHashFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "known content",
			content: "hello world\n",
		},
		{
			name:    "empty file",
			content: "",
		},
		{
			name:    "binary-like content",
			content: "\x00\x01\x02\x03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			dir := t.TempDir()
			path := filepath.Join(dir, "testfile")
			err := os.WriteFile(path, []byte(tt.content), 0o644)
			require.NoError(t, err)

			got, err := hashFile(path)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			// verify against direct xxhash computation
			expected := xxhash.Sum64([]byte(tt.content))
			var buf [8]byte
			h := xxhash.New()
			h.Write([]byte(tt.content))
			sum := h.Sum(buf[:0])
			want := hex.EncodeToString(sum)
			_ = expected

			require.Equal(t, want, got)
		})
	}
}

func TestHashFile_MissingFile(t *testing.T) {
	_, err := hashFile("/nonexistent/path/to/file")
	require.Error(t, err)
}
