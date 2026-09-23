package tested

import "example.com/covfixture/untested"

func A(x int) int {
	if x > 10 {
		return untested.B(x)
	}
	return 0
}
