package ops

func Compare(a, b int) bool {
	if a < b || a <= 0 {
		return a == b && a != 0
	}
	return !(a > b) && a >= 1
}

// Order compares strings, which boundary and equality change like any other operands.
func Order(a, b string) bool {
	return a == b || a < b
}

func Present(p *int) bool {
	return p != nil
}

// Twice makes the same comparison twice, so the second one's id ends in #2.
func Twice(x int) bool {
	a := x > 0
	b := x > 0
	return a && b
}
