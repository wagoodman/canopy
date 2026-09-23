package untested

// B is only reached through the tested package, so it only gets credit under -coverpkg.
func B(x int) int {
	if x > 0 {
		return 1
	}
	return 2
}

func Unused() int { return 3 }
