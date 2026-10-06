package ops

import "cmp"

// Number holds only numbers, so arithmetic on a type parameter it constrains is arithmetic.
type Number interface{ ~int | ~float64 }

// Integer holds only numbers through the interface it embeds.
type Integer interface{ Whole }

type Whole interface{ ~int | ~int64 }

func Sum[T Number](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Double[T Integer](x T) T {
	return x * 2
}

func Halve[T interface{ int }](x T) T {
	return x / 2
}

// Join's type parameter may be a string, so its + may concatenate.
func Join[T ~int | ~string](a, b T) T {
	return a + b
}

// Plus's type parameter may be a string too.
func Plus[T cmp.Ordered](a, b T) T {
	return a + b
}
