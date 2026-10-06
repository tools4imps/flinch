// Package calc is code the fixture's Contract holds.
package calc

// Base is set by an initializer, which coverage can't see, so its mutants are reached by linking.
var Base = 40 + 2

// Limit's only mutant, 200 + 100, overflows a uint8, so it never type-checks.
var Limit uint8 = 200 - 100

var ready bool

func init() {
	ready = true
}

// Ready reports whether init ran.
func Ready() bool {
	return ready
}

// Add adds.
func Add(a, b int) int {
	sum := a + b
	return sum
}

// Sign names the sign of n. Its comparisons sit in case headers, which no coverage block holds.
func Sign(n int) string {
	switch {
	case n < 0:
		return "negative"
	case n == 0:
		return "zero"
	}
	return "positive"
}
