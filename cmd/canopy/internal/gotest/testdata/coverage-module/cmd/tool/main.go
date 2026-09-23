// package main with no tests: it never gets linked into a test binary.
package main

import "fmt"

func main() {
	for i := 0; i < 3; i++ {
		fmt.Println(i)
	}
}
