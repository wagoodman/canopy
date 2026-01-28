package fail

import (
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test1(t *testing.T) {}

func Test2(t *testing.T) {
	t.Error("message!")
}

func Test3(t *testing.T) {
	t.Fatal("message!")
}

func Test4(t *testing.T) {
	// fail ~40% of the time (flaky test for testing flaky detection)
	if rand.Float64() < 0.4 {
		t.Fatal("random failure")
	}
}

func Test5(t *testing.T) {
	subject := 2
	assert.Equal(t, 33, subject, "should be equal")
}

func Test6(t *testing.T) {
	subject := 3
	require.Equal(t, 10, subject, "should be equal")
}

func Test7(t *testing.T) {
	subject := "thing!\nis?\nhere!"
	if d := cmp.Diff("thing?\nis?\nHERE!\n\n:)", subject); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}

func Test8(t *testing.T) {
	subject := []string{"thing!", "is?", "not", "here!"}
	if d := cmp.Diff([]string{"thing?", "is?", "not", "HERE!", " ", "or-there", ":)"}, subject); d != "" {
		t.Fatalf("mismatch (-want +got):\n%s", d)
	}
}
