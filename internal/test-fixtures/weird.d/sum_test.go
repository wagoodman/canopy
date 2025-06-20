package weird_d

import (
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"
)

var skipOops = flag.Bool("skip-oops", false, "skip tests that have 'oops' in the name")

func TestAddNested(t *testing.T) {
	testCases := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"Test positive numbers", 2, 3, 5},
		{"Test negative numbers", -5, 7, 2},
		{"Test zero", 0, 0, 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Log("sleep is for the wicked!")
			time.Sleep(1 * time.Second)
			// note: there is a nested t.Run here... for no reason at all other than to be evil
			// also note that there is simply one case, so this is a bit silly (but different than the loop test below)
			t.Logf("this is an exciting log message for %q", tt.name)

			t.Run("nested-something", func(t *testing.T) {
				sum := Add(tt.a, tt.b)
				if sum != tt.expected {
					t.Fatalf("Expected %d but got %d", tt.expected, sum)
				}
			})
		})
	}
	if skipOops != nil && !*skipOops {
		t.Error("fail anyway!")
	}
}

func TestDuplicateStructure(t *testing.T) {
	t.Run("test/something", func(t *testing.T) {

	})

	t.Run("test", func(t *testing.T) {
		t.Run("something", func(t *testing.T) {

		})
	})
}

func TestAddNestedLoop(t *testing.T) {
	over := []int{
		1,
		2,
		3,
	}
	testCases := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"Test positive numbers", 2, 3, 5},
		{"Test negative numbers", -5, 7, 2},
		{"Test zero", 0, 0, 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// note: there is a nested t.Run here... for no reason at all other than to be evil
			for _, o := range over {
				t.Run(fmt.Sprintf("offset=%d", o), func(t *testing.T) {
					sum := Add(tt.a+o, tt.b)
					if sum != tt.expected+o {
						t.Fatalf("Expected %d but got %d", tt.expected, sum)
					}
				})
			}
		})
	}

	if skipOops != nil && !*skipOops {
		// we want this failure to show even though there is no output from any subtest or test
		t.Fail()
	}
}

func TestAddFailingSubtest(t *testing.T) {
	over := []int{
		1,
		2,
		3,
	}
	testCases := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"Test normal numbers", 2, 3, 5},
		{"Test weird numbers (oops)", -5, 7, 37},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if skipOops != nil && *skipOops && strings.Contains(tt.name, "oops") {
				t.Skip()
			}
			// note: there is a nested t.Run here... for no reason at all other than to be evil
			for _, o := range over {
				t.Run(fmt.Sprintf("offset=%d", o), func(t *testing.T) {
					sum := Add(tt.a+o, tt.b)
					if sum != tt.expected+o {
						t.Fatalf("Expected %d but got %d", tt.expected, sum)
					}
				})
			}
		})
	}
}

func TestParallel(t *testing.T) {

	testCases := []struct {
		name string
		amt  time.Duration
	}{
		{"sleep a little", 1},
		{"sleep just a little", 1},
		{"sleep some more", 2},
		{"sleep some more now", 2},
		{"sleep even more now!", 3},
		{"sleep a lot more", 4},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			time.Sleep(tt.amt * time.Second)
		})
	}
}
