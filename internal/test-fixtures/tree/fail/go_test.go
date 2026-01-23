package fail

import "testing"

func Test1(t *testing.T) {}

func Test2(t *testing.T) {
	t.Error("message!")
}

func Test3(t *testing.T) {
	t.Fatal("message!")
}

func Test4(t *testing.T) {
}
