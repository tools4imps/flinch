package shapes

func init() { registered += 2 }

func init() { registered += 3 }

// Add puts x and y in the box and returns what it holds.
func (b *Box) Add(x, y int) int {
	b.n += x + y
	return b.n
}

// Twice grows the box, then adds to it with a call written over two lines.
func Twice(b *Box) {
	b.Grow()
	b.Add(1,
		2)
}
