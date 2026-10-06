// Package calc is a fixture for the declare primitive's Contract tests. Its Contract tests kill
// every mutant in Add and Sub except the ones inside Add's closure, which no test calls.
package calc

// Add returns a plus b.
func Add(a, b int) int {
	bigger := func() bool { return a > b }
	_ = bigger
	return a + b
}

// Sub returns a minus b.
func Sub(a, b int) int {
	return a - b
}
