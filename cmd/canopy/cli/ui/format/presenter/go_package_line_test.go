package presenter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
)

func TestParseAndFormatPackageLine_Columns(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "no test files under -cover",
			line: "\tex.com/a\t\tcoverage: 0.0% of statements",
			want: "?       ex.com/a\t        \t0.0% covered\t[no test files]",
		},
		{
			name: "coverpkg suffix is dropped",
			line: "ok  \tex.com/a\t0.110s\tcoverage: 68.9% of statements in ./...",
			want: "ok      ex.com/a\t 0.11s  \t68.9% covered",
		},
		{
			name: "startup mark and notes",
			line: "ok  \tex.com/a\t6.200s ◕\tcoverage: 100.0% of statements in ./...\t(started after 5.66s)",
			want: "ok      ex.com/a\t 6.20s ◕\t100.0% covered\t(started after 5.66s)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAndFormatPackageLine(tt.line, style.NewGo(false), 0, ""))
		})
	}
}

func TestStripPackagePrefix(t *testing.T) {
	tests := []struct {
		name, pkg, prefix, want string
	}{
		{name: "sub package", pkg: "github.com/anchore/go-make/run", prefix: "github.com/anchore/go-make", want: "run"},
		{name: "module root", pkg: "github.com/anchore/go-make", prefix: "github.com/anchore/go-make", want: "."},
		{name: "prefix with trailing slash", pkg: "github.com/anchore/go-make/run", prefix: "github.com/anchore/go-make/", want: "run"},
		{name: "sibling sharing a prefix", pkg: "github.com/anchore/go-make-extra", prefix: "github.com/anchore/go-make", want: "github.com/anchore/go-make-extra"},
		{name: "unrelated", pkg: "example.com/other", prefix: "github.com/anchore/go-make", want: "example.com/other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripPackagePrefix(tt.pkg, tt.prefix))
		})
	}
}
