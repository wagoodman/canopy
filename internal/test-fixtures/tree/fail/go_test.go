package fail

import "testing"

func Test1(t *testing.T) {}

func Test2(t *testing.T) {
	t.Fail()
}

func Test3(t *testing.T) {}
