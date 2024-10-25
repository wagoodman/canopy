package panic2

import (
	"fmt"
	"testing"
)

func TestPanic1Test(t *testing.T) {
	t.Log("panic test func")

	t.Run("sub-test1", func(t *testing.T) {
		fmt.Println("non-panic test case 1")
	})

	t.Run("sub-test2", func(t *testing.T) {
		t.Log("non-panic test case 2")
	})

	t.Run("sub-test3", func(t *testing.T) {
		t.Log("non-panic test case 4")
	})

	panic("err, woops? (traceback)\nthere should be more than one line here\nand here...")

	t.Log("panic test func (after...should never show)")
}
