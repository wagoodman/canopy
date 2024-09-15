package gotest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractAnnotations(t *testing.T) {

	tests := []struct {
		name   string
		output string
		want   []Annotation
	}{
		{
			name:   "no annotations",
			output: "this is a normal output",
			want:   []Annotation{},
		},
		{
			name:   "no test files",
			output: "?   \tthis is a normal output\t[no test files]\n",
			want:   []Annotation{NoTestFiles},
		},
		{
			name:   "no tests to run",
			output: "?   \tthis is a normal output\t[no tests to run]\n",
			want:   []Annotation{NoTestsToRun},
		},
		{
			name:   "cached",
			output: "?   \tthis is a normal output\t(cached)\n",
			want:   []Annotation{Cached},
		},
		{
			name:   "build failed",
			output: "?   \tthis is a normal output\t[build failed]\n",
			want:   []Annotation{BuildFailed},
		},
		{
			name:   "setup failed",
			output: "?   \tthis is a normal output\t[setup failed]\n",
			want:   []Annotation{SetupFailed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, ExtractAnnotations(tt.output))
		})
	}
}
