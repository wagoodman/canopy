package commands

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

func TestFilterPackages(t *testing.T) {
	pkgs := []db.PackageCoverage{
		{PackagePath: "github.com/org/repo/internal/auth", Percent: 82.4},
		{PackagePath: "github.com/org/repo/internal/db", Percent: 71.2},
		{PackagePath: "github.com/org/repo/cmd/server", Percent: 45.1},
		{PackagePath: "github.com/org/repo/pkg/utils", Percent: 90.0},
	}

	tests := []struct {
		name    string
		pattern string
		min     *float64
		max     *float64
		want    []string
	}{
		{
			name:    "no filter",
			pattern: "",
			min:     nil,
			max:     nil,
			want:    []string{"github.com/org/repo/internal/auth", "github.com/org/repo/internal/db", "github.com/org/repo/cmd/server", "github.com/org/repo/pkg/utils"},
		},
		{
			name:    "filter by pattern with ...",
			pattern: "github.com/org/repo/internal/...",
			min:     nil,
			max:     nil,
			want:    []string{"github.com/org/repo/internal/auth", "github.com/org/repo/internal/db"},
		},
		{
			name:    "filter by min coverage",
			pattern: "",
			min:     ptr(80.0),
			max:     nil,
			want:    []string{"github.com/org/repo/internal/auth", "github.com/org/repo/pkg/utils"},
		},
		{
			name:    "filter by max coverage",
			pattern: "",
			min:     nil,
			max:     ptr(50.0),
			want:    []string{"github.com/org/repo/cmd/server"},
		},
		{
			name:    "filter by min and max coverage",
			pattern: "",
			min:     ptr(70.0),
			max:     ptr(85.0),
			want:    []string{"github.com/org/repo/internal/auth", "github.com/org/repo/internal/db"},
		},
		{
			name:    "combined pattern and min filter",
			pattern: "github.com/org/repo/internal/...",
			min:     ptr(80.0),
			max:     nil,
			want:    []string{"github.com/org/repo/internal/auth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterPackages(pkgs, tt.pattern, tt.min, tt.max)
			got := make([]string, len(result))
			for i, pkg := range result {
				got[i] = pkg.PackagePath
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filterPackages() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFilterFunctions(t *testing.T) {
	funcs := []db.FunctionCoverage{
		{FilePath: "github.com/org/repo/internal/auth/auth.go", FuncName: "ValidateToken", Percent: 100.0},
		{FilePath: "github.com/org/repo/internal/auth/auth.go", FuncName: "RefreshToken", Percent: 50.0},
		{FilePath: "github.com/org/repo/internal/db/db.go", FuncName: "Connect", Percent: 66.7},
		{FilePath: "github.com/org/repo/cmd/server/main.go", FuncName: "main", Percent: 0.0},
	}

	tests := []struct {
		name    string
		pattern string
		min     *float64
		max     *float64
		want    []string
	}{
		{
			name:    "no filter",
			pattern: "",
			min:     nil,
			max:     nil,
			want:    []string{"ValidateToken", "RefreshToken", "Connect", "main"},
		},
		{
			name:    "filter by pattern with ...",
			pattern: "github.com/org/repo/internal/...",
			min:     nil,
			max:     nil,
			want:    []string{"ValidateToken", "RefreshToken", "Connect"},
		},
		{
			name:    "filter by max coverage (find gaps)",
			pattern: "",
			min:     nil,
			max:     ptr(50.0),
			want:    []string{"RefreshToken", "main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterFunctions(funcs, tt.pattern, tt.min, tt.max)
			got := make([]string, len(result))
			for i, fn := range result {
				got[i] = fn.FuncName
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filterFunctions() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSortPackages(t *testing.T) {
	tests := []struct {
		name   string
		sortBy string
		desc   bool
		want   []string
	}{
		{
			name:   "sort by name ascending",
			sortBy: "name",
			desc:   false,
			want:   []string{"github.com/org/repo/cmd/server", "github.com/org/repo/internal/auth", "github.com/org/repo/internal/db"},
		},
		{
			name:   "sort by name descending",
			sortBy: "name",
			desc:   true,
			want:   []string{"github.com/org/repo/internal/db", "github.com/org/repo/internal/auth", "github.com/org/repo/cmd/server"},
		},
		{
			name:   "sort by percent ascending",
			sortBy: "percent",
			desc:   false,
			want:   []string{"github.com/org/repo/cmd/server", "github.com/org/repo/internal/db", "github.com/org/repo/internal/auth"},
		},
		{
			name:   "sort by percent descending",
			sortBy: "percent",
			desc:   true,
			want:   []string{"github.com/org/repo/internal/auth", "github.com/org/repo/internal/db", "github.com/org/repo/cmd/server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := []db.PackageCoverage{
				{PackagePath: "github.com/org/repo/internal/auth", Percent: 82.4},
				{PackagePath: "github.com/org/repo/internal/db", Percent: 71.2},
				{PackagePath: "github.com/org/repo/cmd/server", Percent: 45.1},
			}
			sortPackages(pkgs, tt.sortBy, tt.desc)
			got := make([]string, len(pkgs))
			for i, pkg := range pkgs {
				got[i] = pkg.PackagePath
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sortPackages() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMatchPackagePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{
			name:    "... suffix matches subpackages",
			pattern: "github.com/org/repo/internal/...",
			value:   "github.com/org/repo/internal/auth",
			want:    true,
		},
		{
			name:    "... suffix matches nested subpackages",
			pattern: "github.com/org/repo/internal/...",
			value:   "github.com/org/repo/internal/auth/token",
			want:    true,
		},
		{
			name:    "... suffix does not match unrelated",
			pattern: "github.com/org/repo/internal/...",
			value:   "github.com/org/repo/cmd/server",
			want:    false,
		},
		{
			name:    "./internal/... prefix stripped",
			pattern: "./internal/...",
			value:   "internal/auth",
			want:    true,
		},
		{
			name:    "exact match",
			pattern: "github.com/org/repo/internal/auth",
			value:   "github.com/org/repo/internal/auth",
			want:    true,
		},
		{
			name:    "wildcard pattern",
			pattern: "*internal*",
			value:   "github.com/org/repo/internal/auth",
			want:    false, // filepath.Match doesn't support ** or substring matching
		},
		{
			name:    "exact match only",
			pattern: "internal",
			value:   "internal",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPackagePattern(tt.pattern, tt.value)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestWriteCoverageJSON(t *testing.T) {
	runInfo := test.RunInfo{
		UUID:     uuid.MustParse("12345678-1234-1234-1234-123456789abc"),
		Started:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Coverage: ptr(75.5),
	}

	testRun := db.TestRun{
		CoverageDir: "/tmp/coverage/12345678",
	}

	pkgs := []db.PackageCoverage{
		{PackagePath: "github.com/org/repo/internal/auth", Percent: 82.4},
	}

	funcs := []db.FunctionCoverage{
		{FilePath: "github.com/org/repo/internal/auth/auth.go", FuncName: "ValidateToken", Line: 45, Percent: 100.0},
	}

	var buf bytes.Buffer
	err := writeCoverageJSON(&buf, runInfo, testRun, pkgs, funcs)
	require.NoError(t, err)

	var output coverageOutput
	err = json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err)

	require.Equal(t, "12345678-1234-1234-1234-123456789abc", output.RunID)
	require.Equal(t, "/tmp/coverage/12345678", output.CovdataPath)
	require.Equal(t, 75.5, output.Total.Percent)
	require.Len(t, output.Packages, 1)
	require.Equal(t, "github.com/org/repo/internal/auth", output.Packages[0].Path)
	require.Len(t, output.Packages[0].Functions, 1)
	require.Equal(t, "ValidateToken", output.Packages[0].Functions[0].Name)
	require.Equal(t, "auth.go", output.Packages[0].Functions[0].File)
	require.Equal(t, 45, output.Packages[0].Functions[0].Line)
}

func TestExtractPackagePath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "standard go path",
			filePath: "github.com/org/repo/internal/auth/auth.go",
			want:     "github.com/org/repo/internal/auth",
		},
		{
			name:     "nested path",
			filePath: "github.com/org/repo/internal/auth/token/jwt.go",
			want:     "github.com/org/repo/internal/auth/token",
		},
		{
			name:     "root level file",
			filePath: "main.go",
			want:     ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPackagePath(tt.filePath)
			require.Equal(t, tt.want, got)
		})
	}
}

func ptr(f float64) *float64 {
	return &f
}
