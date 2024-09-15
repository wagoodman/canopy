package math

import "time"

func Fibonacci(n int) []int {
	time.Sleep(250 * time.Millisecond)

	fib := make([]int, n)
	a, b := 0, 1
	for i := 0; i < n; i++ {
		fib[i] = a
		a, b = b, a+b
	}
	return fib
}
