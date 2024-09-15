package panic

import "testing"

func TestBeforePanic(t *testing.T) {
	t.Run("sub-test", func(t *testing.T) {
		t.Log("test before panic")
	})

}

func TestPanic(t *testing.T) {
	t.Log("panic test func")
	t.Run("sub-test", func(t *testing.T) {
		t.Log("panic test case")

		panic("err, woops?")
	})

	t.Log("panic test func (after)")
}
