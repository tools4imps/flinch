package a

func F(x int) int {
	if x > 0 {
		return 1
	}
	g := func() int { return x + 1 }
	return g()
}

func G(x, y int) int { return x - y }

type T struct{}

func (T) M() int { return 1 }

func init() {}
