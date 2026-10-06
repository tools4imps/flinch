// Package a is covered by the fixture's primitive p.
package a

// F has a closure inside it.
func F(x int) int {
	if x > 0 {
		return 1
	}
	g := func() int { return x + 1 }
	return g()
}

// G has nothing inside it.
func G(x, y int) int { return x - y }

// T has a method.
type T struct{}

// M is T's method.
func (T) M() int { return 1 }

func init() {}
