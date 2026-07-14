package localize

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestSymbolsFromSource(t *testing.T) {
	src := `package flaky

import "fmt"

// a package-level var/const/type is NOT a symbol: it is not a call-graph node.
var threshold = 0.5

type Analyzer struct{ name string }

func calculateFlakyScore(passes, fails int) float64 {
	return float64(passes)
}

func (a *Analyzer) Analyze() string {
	return a.name
}

func helper() {
	fmt.Println("x")
}
`
	got, err := symbolsFromSource("/abs/flaky/analyzer.go", []byte(src))
	require.NoError(t, err)

	// only funcs/methods; var/const/type are excluded. methods carry their receiver.
	want := []Symbol{
		{File: "/abs/flaky/analyzer.go", Name: "calculateFlakyScore", Line: 10, EndLine: 12},
		{File: "/abs/flaky/analyzer.go", Name: "*Analyzer.Analyze", Line: 14, EndLine: 16},
		{File: "/abs/flaky/analyzer.go", Name: "helper", Line: 18, EndLine: 20},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("symbolsFromSource() mismatch (-want +got):\n%s", diff)
	}
}

func TestSymbolsFromSource_ParseError(t *testing.T) {
	_, err := symbolsFromSource("bad.go", []byte("package x\nfunc ("))
	require.Error(t, err)
}
