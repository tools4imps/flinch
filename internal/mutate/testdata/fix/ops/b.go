package ops

func init() {
	if limit > 5 {
		limit = 5
	}
}

// Unowned belongs to no primitive, so it gets no mutants.
func Unowned(x int) int { return x + 1 }
