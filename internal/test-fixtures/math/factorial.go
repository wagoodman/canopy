package math

import "time"

func Factorial(n int) int {
	time.Sleep(250 * time.Millisecond)

	if n < 0 {
		return 0
	}
	if n == 0 {
		return 1
	}
	result := 1
	for i := 1; i <= n; i++ {
		result *= i
	}
	return result
}
