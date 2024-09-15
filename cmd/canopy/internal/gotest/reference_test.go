package gotest

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestReference_String(t *testing.T) {

	tests := []struct {
		name  string
		ref   Reference
		clean bool
		want  string
	}{
		{
			name: "package only",
			ref: Reference{
				Package: "github.com/wagoodman/canopy",
			},
			want: "github.com/wagoodman/canopy",
		},
		{
			name: "package and test function",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
			},
			want: "github.com/wagoodman/canopy/TestReference_String",
		},
		{
			name: "package, test function, and test name",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package only",
			},
			want: "github.com/wagoodman/canopy/TestReference_String/package only",
		},
		{
			name: "package, test function, and test + sub-test name",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package only/sub test",
			},
			want: "github.com/wagoodman/canopy/TestReference_String/package only/sub test",
		},
		{
			name: "package, test function, and test name (w/ clean)",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package only",
			},
			clean: true, // IMPORTANT!
			want:  "github.com/wagoodman/canopy/TestReference_String/package_only",
		},
		{
			name: "package, test function, and test + sub-test name (w/ clean)",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package only/sub test",
			},
			clean: true, // IMPORTANT!
			want:  "github.com/wagoodman/canopy/TestReference_String/package_only/sub_test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.ref.String(tt.clean), tt.want)
		})
	}
}

func TestReference_Parent(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want *Reference
	}{
		{
			name: "package only",
			ref: Reference{
				Package: "github.com/wagoodman/canopy",
			},
			want: nil,
		},
		{
			name: "package and test function",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
			},
			want: &Reference{
				Package: "github.com/wagoodman/canopy",
			},
		},
		{
			name: "package, test function, and test name",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package_only",
			},
			want: &Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
			},
		},
		{
			name: "package, test function, and test + sub-test name",
			ref: Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package_only/sub_test",
			},
			want: &Reference{
				Package:  "github.com/wagoodman/canopy",
				FuncName: "TestReference_String",
				TRunName: "package_only",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.ref.ParentRef(), tt.want)
		})
	}
}

func TestReference_rewriteTestName(t *testing.T) {
	tests := []struct {
		name string
		ref  Reference
		want string
	}{
		{
			name: "package only",
			want: "package_only",
		},
		{
			name: "package only/sub test",
			want: "package_only/sub_test",
		},
		{
			name: "with all sorts of wrenches {!@#$%^&*`\\ ~-| \"'* :;(/)[]?<+>.}",
			want: "with_all_sorts_of_wrenches_{!@#$%^&*`\\_~-|_\"'*_:;(/)[]?<+>.}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, rewriteTestName(tt.name), tt.want)
		})
	}
}
