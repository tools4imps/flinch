package ops

// Constants, types and signatures sit outside every unit, so nothing in them is mutated.
const k = 1 + 2

type arr [2 * 3]int

func Sig(x [4 - 1]int) [1 + 1]int {
	return [2]int{x[0], x[2]}
}
