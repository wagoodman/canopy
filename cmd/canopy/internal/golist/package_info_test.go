package golist

import (
	"encoding/json"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mirrors the shape go list feeds the -f template (Module is nil for stdlib/GOPATH pkgs).
type tmplModule struct{ Path string }
type tmplPkg struct {
	Dir        string
	ImportPath string
	Module     *tmplModule
}

// renders packageInfoTemplate the same way `go list -f` would, then decodes it the same
// way PackageInfo's processor does, to prove the emitted line is valid JSON.
func TestPackageInfoTemplateProducesValidJSON(t *testing.T) {
	tmpl := template.Must(template.New("pkg").Parse(packageInfoTemplate))

	tests := []struct {
		name string
		in   tmplPkg
		want Package
	}{
		{
			name: "windows path with backslashes",
			in:   tmplPkg{Dir: `C:\Users\foo\pkg`, ImportPath: "example.com/foo/pkg", Module: &tmplModule{Path: "example.com/foo"}},
			want: Package{Dir: `C:\Users\foo\pkg`, ImportPath: "example.com/foo/pkg", ModulePath: "example.com/foo"},
		},
		{
			name: "path containing a quote",
			in:   tmplPkg{Dir: `/tmp/we"ird`, ImportPath: "example.com/bar", Module: &tmplModule{Path: "example.com/bar"}},
			want: Package{Dir: `/tmp/we"ird`, ImportPath: "example.com/bar", ModulePath: "example.com/bar"},
		},
		{
			name: "nil module (stdlib / GOPATH)",
			in:   tmplPkg{Dir: "/usr/local/go/src/fmt", ImportPath: "fmt", Module: nil},
			want: Package{Dir: "/usr/local/go/src/fmt", ImportPath: "fmt", ModulePath: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			require.NoError(t, tmpl.Execute(&buf, tt.in))

			var got Package
			require.NoError(t, json.Unmarshal([]byte(buf.String()), &got), "rendered template was not valid JSON: %s", buf.String())
			assert.Equal(t, tt.want, got)
		})
	}
}
