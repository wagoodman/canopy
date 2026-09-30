// package a is part of the sharding test fixture.
package a

// Name identifies this package.
const Name = "mid/a"

// Sign gives the join a coverprofile with statements to merge; the negative branch is never
// tested, so coverage stays below 100%.
func Sign(n int) string {
	if n < 0 {
		return "negative"
	}
	return "non-negative"
}
