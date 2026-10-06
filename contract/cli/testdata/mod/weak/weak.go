// Package weak is code the fixture's Contract holds loosely: nothing checks what Positive returns,
// and nothing calls Unused.
package weak

// Positive says whether n is more than zero.
func Positive(n int) bool {
	return n > 0
}

// Double doubles n.
func Double(n int) int {
	return n * 2
}

// Unused is code no Contract test reaches.
func Unused(n int) int {
	return n + 1
}
