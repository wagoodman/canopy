package gotest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMinimalSelection(t *testing.T) {
	tests := []struct {
		name     string
		defs     Definitions
		refs     References
		expected References
	}{
		{
			name: "all functions in package selected - returns package reference",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc2"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc3"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc3"},
			},
			expected: References{
				{Package: "github.com/example/pkg"},
			},
		},
		{
			name: "partial functions selected - returns individual function references",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc2"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc3"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
			},
			expected: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
			},
		},
		{
			name: "package reference provided - returns package reference",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc2"},
			},
			refs: References{
				{Package: "github.com/example/pkg"},
			},
			expected: References{
				{Package: "github.com/example/pkg"},
			},
		},
		{
			name: "subtests are ignored in minimal selection",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc2"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest2"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
			},
			expected: References{
				{Package: "github.com/example/pkg"},
			},
		},
		{
			name: "mixed packages - some complete, some partial",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg1", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg1", FnName: "TestFunc2"},
				{ImportPath: "github.com/example/pkg2", FnName: "TestFuncA"},
				{ImportPath: "github.com/example/pkg2", FnName: "TestFuncB"},
				{ImportPath: "github.com/example/pkg2", FnName: "TestFuncC"},
			},
			refs: References{
				{Package: "github.com/example/pkg1", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg1", FuncName: "TestFunc2"},
				{Package: "github.com/example/pkg2", FuncName: "TestFuncA"},
				{Package: "github.com/example/pkg2", FuncName: "TestFuncB"},
			},
			expected: References{
				{Package: "github.com/example/pkg1"},
				{Package: "github.com/example/pkg2", FuncName: "TestFuncA"},
				{Package: "github.com/example/pkg2", FuncName: "TestFuncB"},
			},
		},
		{
			name: "complex example from documentation",
			defs: Definitions{
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FnName: "TestMultiPackageHandler_Handle"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FnName: "TestMultiPackageHandler_OnGoTestEvent"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FnName: "TestMultiPackageHandler_String"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FnName: "TestNewMultiPackageHandler"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FnName: "TestQuietHandler"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FnName: "TestQuietPackage"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FnName: "TestVerboseHandler"},
				{ImportPath: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FnName: "TestVerbosePackage"},
			},
			refs: References{
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FuncName: "TestMultiPackageHandler_Handle"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FuncName: "TestMultiPackageHandler_OnGoTestEvent"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FuncName: "TestMultiPackageHandler_String"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler", FuncName: "TestNewMultiPackageHandler"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FuncName: "TestQuietHandler"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FuncName: "TestQuietPackage"},
			},
			expected: References{
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FuncName: "TestQuietHandler"},
				{Package: "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx", FuncName: "TestQuietPackage"},
			},
		},
		{
			name:     "empty inputs",
			defs:     Definitions{},
			refs:     References{},
			expected: nil,
		},
		{
			name: "no definitions but refs provided",
			defs: Definitions{},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
			},
			expected: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
			},
		},
		{
			name: "only subtests provided - filtered out",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1", TRunName: "subtest2"},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinimalSelection(tt.defs, tt.refs)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("MinimalSelection() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMinimalSelection_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		defs     Definitions
		refs     References
		expected References
	}{
		{
			name: "function selected but not defined - should still be included",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"}, // not defined
			},
			expected: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
			},
		},
		{
			name: "duplicate references - should be handled correctly",
			defs: Definitions{
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc1"},
				{ImportPath: "github.com/example/pkg", FnName: "TestFunc2"},
			},
			refs: References{
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"},
				{Package: "github.com/example/pkg", FuncName: "TestFunc1"}, // duplicate
				{Package: "github.com/example/pkg", FuncName: "TestFunc2"},
			},
			expected: References{
				{Package: "github.com/example/pkg"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinimalSelection(tt.defs, tt.refs)

			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("MinimalSelection() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
