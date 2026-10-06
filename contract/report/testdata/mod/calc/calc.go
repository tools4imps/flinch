// Package calc is a small fixture for Contract tests that run flinch whole.
package calc

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Twice doubles n, and no Contract test calls it.
func Twice(n int) int {
	return n * 2
}
