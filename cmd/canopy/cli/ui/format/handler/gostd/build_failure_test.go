package gostd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// buildFailureEvents mirrors a real `go test -json ./...` run over a module where:
//   - broken   does not compile on its own
//   - user     compiles fine but imports dep, which does not compile
//   - dep      does not compile
//   - good     passes
//
// go attributes each compiler diagnostic to the build unit it compiled, never to the package whose run
// failed, and reports completion in whatever order the packages finish.
func buildFailureEvents() []gotest.Event {
	brokenUnit := gotest.NewReference("example.com/m/broken", "")
	depUnit := gotest.NewReference("example.com/m/dep", "")

	return []gotest.Event{
		{Action: gotest.BuildOutputAction, Reference: depUnit, Output: "# example.com/m/dep\n"},
		{Action: gotest.BuildOutputAction, Reference: depUnit, Output: "dep/dep.go:3:2: undefined: nope\n"},
		{Action: gotest.BuildFailAction, Reference: depUnit},
		{Action: gotest.BuildOutputAction, Reference: brokenUnit, Output: "# example.com/m/broken [example.com/m/broken.test]\n"},
		{Action: gotest.BuildOutputAction, Reference: brokenUnit, Output: "broken/b_test.go:5:2: undefined: alsoNope\n"},
		{Action: gotest.BuildFailAction, Reference: brokenUnit},

		// packages conclude out of alphabetical order
		{Action: gotest.FailAction, Reference: gotest.NewReference("example.com/m/dep", ""), Output: "FAIL\texample.com/m/dep [build failed]\n", FailedBuild: "example.com/m/dep"},
		{Action: gotest.FailAction, Reference: gotest.NewReference("example.com/m/user", ""), Output: "FAIL\texample.com/m/user [build failed]\n", FailedBuild: "example.com/m/dep"},
		{Action: gotest.PassAction, Reference: gotest.NewReference("example.com/m/good", "TestGood"), Output: "--- PASS: TestGood (0.00s)\n"},
		{Action: gotest.PassAction, Reference: gotest.NewReference("example.com/m/good", ""), Output: "ok  \texample.com/m/good\t0.01s\n"},
		{Action: gotest.FailAction, Reference: gotest.NewReference("example.com/m/broken", ""), Output: "FAIL\texample.com/m/broken [build failed]\n", FailedBuild: "example.com/m/broken"},
	}
}

func TestHandlers_BuildFailureDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name string
		new  func(*bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error }
	}{
		{"quiet", func(b *bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error } {
			return NewQuietHandler(b, PackageConfig{}).(*quietHandler)
		}},
		{"verbose", func(b *bytes.Buffer) interface{ OnGoTestEvent(gotest.Event) error } {
			return NewVerboseHandler(b, PackageConfig{}).(*verboseHandler)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := tt.new(&buf)
			for _, e := range buildFailureEvents() {
				require.NoError(t, h.OnGoTestEvent(e))
			}
			out := buf.String()

			// a package broken by its own sources shows its own diagnostic...
			assert.Contains(t, out, "broken/b_test.go:5:2: undefined: alsoNope")
			// ...and one broken by a dependency shows the dependency's, rather than a bare "[build failed]"
			assert.Equal(t, 2, strings.Count(out, "dep/dep.go:3:2: undefined: nope"),
				"expected the dep diagnostic under both dep and user:\n%s", out)

			// the build units must not surface as packages of their own
			assert.NotContains(t, out, "[example.com/m/broken.test]\tFAIL")

			// packages render alphabetically regardless of completion order
			assert.Equal(t,
				[]string{"example.com/m/broken", "example.com/m/dep", "example.com/m/good", "example.com/m/user"},
				concludedPackageOrder(out),
				"package output order:\n%s", out)
		})
	}
}

// concludedPackageOrder extracts the package names from the go conclusion lines, in the order rendered.
func concludedPackageOrder(out string) []string {
	var order []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != "FAIL" && fields[0] != "ok") {
			continue
		}
		order = append(order, fields[1])
	}
	return order
}
