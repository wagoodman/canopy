package gotest

import (
	"go/token"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
)

func TestFindDefinitions(t *testing.T) {

	tests := []struct {
		name    string
		pkg     golist.Package
		want    []Definition
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "struct literal with name field supplied to t.Run",
			pkg: golist.Package{
				Dir: "./testdata/definition/case-1",
			},
			want: []Definition{
				{
					FnName: "TestIsPalindrome",
					Start: token.Position{
						Filename: "testdata/definition/case-1/go_test.go",
						Offset:   147,
						Line:     11,
						Column:   1,
					},
					End: token.Position{
						Filename: "testdata/definition/case-1/go_test.go",
						Offset:   837,
						Line:     35,
						Column:   2,
					},
					Cases: []string{
						"single_word_palindrome",
						"not_palindrome",
						"single_word_palindrome#01",
						"mix_case_palindrome_(oops)",
						"lower_case_palindrome",
					},
				},
			},
		},
		{
			name: "struct literal with NO name field and NO t.Run",
			pkg: golist.Package{
				Dir: "./testdata/definition/case-2",
			},
			want: []Definition{
				{
					FnName: "TestGetCommonHobbies",
					Start: token.Position{
						Filename: "testdata/definition/case-2/go_test.go",
						Offset:   147,
						Line:     11,
						Column:   1,
					},
					End: token.Position{
						Filename: "testdata/definition/case-2/go_test.go",
						Offset:   1250,
						Line:     48,
						Column:   2,
					},
					Cases: []string{
						"#01",
						"#02",
						"#03",
					},
				},
			},
		},
		{
			// TODO: could we cover this one day by looking at multiple files?
			name: "struct literal embedded in range with name field supplied to t.Run",
			pkg: golist.Package{
				Dir: "./testdata/definition/case-3",
			},
			want: []Definition{
				{
					FnName: "TestFactorial",
					Start: token.Position{
						Filename: "testdata/definition/case-3/go_test.go",
						Offset:   34,
						Line:     5,
						Column:   1,
					},
					End: token.Position{
						Filename: "testdata/definition/case-3/go_test.go",
						Offset:   417,
						Line:     23,
						Column:   2,
					},
					Cases: []string{
						"test1",
						"test2",
						"test3",
						"test4",
					},
				},
			},
		},
		{
			name: "loop variable out of scope while looking for t.Run",
			pkg: golist.Package{
				Dir: "./testdata/definition/case-4",
			},
			want: []Definition{
				{
					FnName: "TestNewZip64FileManifest",
					Start: token.Position{
						Filename: "testdata/definition/case-4/go_test.go",
						Offset:   70,
						Line:     10,
						Column:   1,
					},
					End: token.Position{
						Filename: "testdata/definition/case-4/go_test.go",
						Offset:   1003,
						Line:     44,
						Column:   2,
					},
					Cases: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			pkgs := golist.NewPackageCollection(tt.pkg)

			got, err := FindDefinitions(pkgs)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if cmp.Diff(tt.want, got) != "" {
				t.Errorf("FindDefinitions() mismatch (-want +got):\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestDefinitions_References(t *testing.T) {
	tests := []struct {
		name string
		d    Definitions
		want []Reference
	}{
		{
			name: "single test",
			d: Definitions{
				{
					ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector",
					FnName:     "TestFilter",
					Start:      token.Position{},
					End:        token.Position{},
					Cases:      nil,
				},
			},
			want: []Reference{
				{
					// generic package reference
					Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector",
				},
				{
					Package:  "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector",
					FuncName: "TestFilter",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.d.References(), "References()")
		})
	}
}
