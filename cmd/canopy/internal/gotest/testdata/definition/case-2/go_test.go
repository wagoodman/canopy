package case_2

import (
	"flag"
	"reflect"
	"testing"
)

var skipOops = flag.Bool("skip-oops", false, "skip tests that have 'oops' in the name")

func TestGetCommonHobbies(t *testing.T) {
	// define a test table of input and expected output pairs
	testTable := []struct {
		input  []Person
		output []string
	}{
		{
			input: []Person{
				{"Alice", 23, []string{"reading", "hiking", "painting"}},
				{"Bob", 27, []string{"running", "hiking", "cooking", "reading"}},
				{"Charlie", 31, []string{"painting", "reading", "knitting"}},
				{"David", 19, []string{"knitting", "cooking", "reading"}},
			},
			output: []string{"reading"},
		},
		{
			input: []Person{
				{"Alice", 23, []string{"reading", "hiking", "painting"}},
				{"Bob", 27, []string{"running", "cooking"}},
				{"Charlie", 31, []string{"painting", "knitting"}},
				{"David", 19, []string{"knitting", "cooking", "reading"}},
			},
			output: []string{},
		},
		{
			input:  []Person{},
			output: []string{},
		},
	}

	// iterate over the test table and run the tests
	for _, test := range testTable {
		result := getCommonHobbies(test.input)
		if !reflect.DeepEqual(result, test.output) {
			t.Errorf("getCommonHobbies(%v) = %v, want %v", test.input, result, test.output)
		}
	}
}

type Person struct {
	Name    string
	Age     int
	Hobbies []string
}

// stub, doesn't really matter... just don't like red squiggles
func getCommonHobbies(_ []Person) []string {
	return nil
}
