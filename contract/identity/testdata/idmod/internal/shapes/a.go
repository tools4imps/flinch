// Package shapes has one of each kind of unit, for naming mutants.
package shapes

const base = 2

var limit = base + 1

var check = func(n int) bool { return n > limit }

var registered int

func init() { registered = registered + 1 }

// A Box holds a number.
type Box struct{ n int }

// Grow makes the box bigger.
func (b *Box) Grow() { b.n++ }

// A List holds items of any type.
type List[T any] struct{ items []T }

// Empty reports whether the list holds nothing.
func (l List[T]) Empty() bool { return len(l.items) == 0 }

// Both makes the same change three times.
func Both(x int) int {
	if x > 0 {
		x--
	}
	if x > 0 {
		x--
	}
	if x > 0 {
		x--
	}
	return x
}

// Nest holds a function literal inside a function literal.
func Nest(n int) func() bool {
	return func() bool {
		inner := func() bool { return n > 1 }
		return inner() && n < 9
	}
}

// Spread writes one expression over two lines.
func Spread(a, b int) bool {
	return a > 0 &&
		b > 0
}

// Spaced compares with a literal that holds a run of spaces.
func Spaced(s string) bool { return s == "a   b" }
