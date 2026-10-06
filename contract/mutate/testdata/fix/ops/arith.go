package ops

// Arith uses each arithmetic operator on numbers.
func Arith(a, b int) int {
	sum := a + b
	diff := a - b
	prod := a * b
	quot := a / b
	rem := a % b
	return sum + diff + prod + quot + rem
}

// Celsius is a defined numeric type, so its + is arithmetic.
type Celsius float64

func Warm(c, by Celsius) Celsius {
	return c + by
}

// Bits uses operators that no default operator changes.
func Bits(a, b uint) uint {
	return a<<1 | b>>1 ^ a&b&^b
}

// Signs uses unary minus and complement, which neither arithmetic nor negation changes.
func Signs(a int) int {
	return -a + ^a
}

// Compound assignments other than += and -= are left alone.
func Compound(a int) int {
	a *= 3
	a /= 2
	a %= 7
	return a
}

func init() {
	limit /= 2
}
