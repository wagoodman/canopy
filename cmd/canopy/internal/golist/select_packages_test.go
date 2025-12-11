package golist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "relative path with dot slash",
			path: "./pkg",
			want: true,
		},
		{
			name: "relative path with double dot",
			path: "../pkg",
			want: true,
		},
		{
			name: "absolute path",
			path: "/usr/local/go",
			want: true,
		},
		{
			name: "current directory",
			path: ".",
			want: true,
		},
		{
			name: "parent directory",
			path: "..",
			want: true,
		},
		{
			name: "recursive pattern",
			path: "./...",
			want: true,
		},
		{
			name: "nested recursive pattern",
			path: "./pkg/...",
			want: true,
		},
		{
			name: "module path",
			path: "github.com/anchore/syft/pkg",
			want: false,
		},
		{
			name: "module path with recursive",
			path: "github.com/anchore/syft/...",
			want: false,
		},
		{
			name: "simple package name",
			path: "fmt",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalPath(tt.path)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateLocalPaths(t *testing.T) {
	// create a temp directory structure for testing
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	require.NoError(t, os.MkdirAll(existingDir, 0755))

	existingFile := filepath.Join(tmpDir, "file.txt")
	require.NoError(t, os.WriteFile(existingFile, []byte("test"), 0644))

	tests := []struct {
		name    string
		paths   []string
		wantErr require.ErrorAssertionFunc
		errMsg  string
	}{
		{
			name:  "valid existing directory",
			paths: []string{existingDir},
		},
		{
			name:  "current directory",
			paths: []string{"."},
		},
		{
			name:  "recursive pattern with existing base",
			paths: []string{existingDir + "/..."},
		},
		{
			name:  "module path is skipped",
			paths: []string{"github.com/nonexistent/module"},
		},
		{
			name:  "mixed valid paths",
			paths: []string{".", "github.com/foo/bar", existingDir},
		},
		{
			name:    "non-existent directory",
			paths:   []string{filepath.Join(tmpDir, "nonexistent")},
			wantErr: require.Error,
			errMsg:  "no such directory",
		},
		{
			name:    "non-existent with recursive pattern",
			paths:   []string{filepath.Join(tmpDir, "nonexistent") + "/..."},
			wantErr: require.Error,
			errMsg:  "no such directory",
		},
		{
			name:    "file instead of directory",
			paths:   []string{existingFile},
			wantErr: require.Error,
			errMsg:  "path is not a directory",
		},
		{
			name:    "one bad path among good ones",
			paths:   []string{".", filepath.Join(tmpDir, "nonexistent"), "github.com/foo/bar"},
			wantErr: require.Error,
			errMsg:  "no such directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			err := validateLocalPaths(tt.paths)
			tt.wantErr(t, err)

			if err != nil && tt.errMsg != "" {
				require.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}
