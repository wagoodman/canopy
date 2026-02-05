package cover

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestParsePercentOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []PackageResult
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "multiple packages",
			input: `	example.com/pkg1		coverage: 41.1% of statements
	example.com/pkg2		coverage: 87.5% of statements`,
			want: []PackageResult{
				{PackagePath: "example.com/pkg1", Percent: 41.1},
				{PackagePath: "example.com/pkg2", Percent: 87.5},
			},
		},
		{
			name:  "single package",
			input: `	example.com/main		coverage: 100.0% of statements`,
			want: []PackageResult{
				{PackagePath: "example.com/main", Percent: 100.0},
			},
		},
		{
			name:  "zero coverage",
			input: `	example.com/pkg		coverage: 0.0% of statements`,
			want: []PackageResult{
				{PackagePath: "example.com/pkg", Percent: 0.0},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:    "malformed line - no percentage",
			input:   `example.com/pkg	no-coverage-here`,
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := parsePercentOutput(tt.input)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("parsePercentOutput mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestParseFuncOutput(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantFuncs      []FunctionResult
		wantOverall    float64
		wantErr        require.ErrorAssertionFunc
	}{
		{
			name: "typical output",
			input: `example.com/pkg1/file.go:12:	hello		100.0%
example.com/pkg1/file.go:18:	unused		0.0%
total:					(statements)	41.1%`,
			wantFuncs: []FunctionResult{
				{FilePath: "example.com/pkg1/file.go", Line: 12, FuncName: "hello", Percent: 100.0},
				{FilePath: "example.com/pkg1/file.go", Line: 18, FuncName: "unused", Percent: 0.0},
			},
			wantOverall: 41.1,
		},
		{
			name: "single function with total",
			input: `example.com/main.go:5:	main		75.5%
total:					(statements)	75.5%`,
			wantFuncs: []FunctionResult{
				{FilePath: "example.com/main.go", Line: 5, FuncName: "main", Percent: 75.5},
			},
			wantOverall: 75.5,
		},
		{
			name:        "only total line",
			input:       `total:					(statements)	50.0%`,
			wantFuncs:   nil,
			wantOverall: 50.0,
		},
		{
			name:        "empty output",
			input:       "",
			wantFuncs:   nil,
			wantOverall: 0,
		},
		{
			name:    "malformed function line - no line number",
			input:   `example.com/file.go	hello	100.0%`,
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			gotFuncs, gotOverall, err := parseFuncOutput(tt.input)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			if d := cmp.Diff(tt.wantFuncs, gotFuncs); d != "" {
				t.Errorf("parseFuncOutput funcs mismatch (-want +got):\n%s", d)
			}
			require.InDelta(t, tt.wantOverall, gotOverall, 0.01)
		})
	}
}

func TestExtractPercent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "standard coverage string",
			input: "coverage: 41.1% of statements",
			want:  41.1,
		},
		{
			name:  "100 percent",
			input: "coverage: 100.0% of statements",
			want:  100.0,
		},
		{
			name:  "zero percent",
			input: "coverage: 0.0% of statements",
			want:  0.0,
		},
		{
			name:    "no percent sign",
			input:   "no percentage here",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := extractPercent(tt.input)
			tt.wantErr(t, err)

			if err != nil {
				return
			}
			require.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestParseFuncLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FunctionResult
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "standard function line",
			input: "example.com/pkg/file.go:12:\thello\t\t100.0%",
			want:  FunctionResult{FilePath: "example.com/pkg/file.go", Line: 12, FuncName: "hello", Percent: 100.0},
		},
		{
			name:  "zero coverage function",
			input: "example.com/pkg/file.go:42:\tunused\t\t0.0%",
			want:  FunctionResult{FilePath: "example.com/pkg/file.go", Line: 42, FuncName: "unused", Percent: 0.0},
		},
		{
			name:    "too few fields",
			input:   "onlyonefield",
			wantErr: require.Error,
		},
		{
			name:    "invalid line number",
			input:   "file.go:abc:\thello\t100.0%",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := parseFuncLine(tt.input)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("parseFuncLine mismatch (-want +got):\n%s", d)
			}
		})
	}
}
