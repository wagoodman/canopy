package strings

func HasUniqueChars(s string) bool {
	// time.Sleep(150 * time.Millisecond)

	chars := make(map[rune]bool)
	for _, c := range s {
		if chars[c] {
			return false
		}
		chars[c] = true
	}
	return true
}
