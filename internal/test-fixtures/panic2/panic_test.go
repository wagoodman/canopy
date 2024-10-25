package panic2

import (
	"testing"
)

func TestBeforePanic(t *testing.T) {
	t.Run("sub-test", func(t *testing.T) {
		t.Log("test before panic")
	})

}

func TestPanic1RecoverPass(t *testing.T) {
	t.Log("panic-recover-pass test func")

	t.Run("sub-test", func(t *testing.T) {

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered. Error: \n %+v", r) // note no failure
			}
		}()

		t.Log("panic-recover test case")

		panic("err, woops? (pass)")
	})

	t.Log("panic-recover test func (after)")
}

func TestPanic2Recover(t *testing.T) {
	t.Log("panic-recover test func")

	t.Run("sub-test", func(t *testing.T) {

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Recovered. Error: \n %+v", r) // fail!
			}
		}()

		t.Log("panic-recover test case")

		panic("err, woops? (fail)")
	})

	t.Log("panic-recover test func (after)")
}

func TestPanic3Subtest(t *testing.T) {
	t.Log("panic test func")

	t.Run("sub-test1", func(t *testing.T) {
		t.Log("non-panic test case 1")
	})

	t.Run("sub-test", func(t *testing.T) {
		t.Log("panic test case")

		panic("err, woops? (traceback)")
	})

	t.Log("panic test func (after...should never show)")
}
