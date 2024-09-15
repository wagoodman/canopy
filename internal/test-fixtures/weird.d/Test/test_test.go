package Test

import (
	"flag"
	"testing"
)

var skipOops = flag.Bool("skip-oops", false, "skip tests that have 'oops' in the name")

func TestDuplicateStructure(t *testing.T) {
	t.Run("test/something", func(t *testing.T) {
		t.Log("logging!")
	})

	t.Run("test", func(t *testing.T) {
		t.Run("something", func(t *testing.T) {

		})
	})
}
