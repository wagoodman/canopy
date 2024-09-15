package strings

import (
	"strings"
)

func IsPalindrome(s string) bool {
	// time.Sleep(150 * time.Millisecond)

	s = strings.ReplaceAll(strings.ToLower(s), " ", "")
	runes := []rune(s)
	for i := 0; i < len(runes)/2; i++ {
		if runes[i] != runes[len(runes)-1-i] {
			return false
		}
	}
	return true
}
