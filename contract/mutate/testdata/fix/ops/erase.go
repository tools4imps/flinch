package ops

import "errors"

// These functions already return their zero values, so erasing them would change nothing.

func Zero() int { return 0 }

func Blank() (string, error) { return "", nil }

func Off() bool { return (false) }

func Rate() float64 { return 0.0 }

func Hex() int { return 0x0 }

func Imag() complex128 { return 0i }

func NUL() rune { return '\x00' }

func Bare() (n int) { return }

func Done() { return }

func Nothing() {}

func NilErr() error { return nil }

func NilPtr() *int { return nil }

// These look close but aren't zero, so they keep their erase mutants.

func Boxed() any { return 0 }

func BoxedFalse() any { return false }

func BoxedEmpty() any { return "" }

func One() int { return 1 }

func Letter() rune { return 'a' }

func Text() string { return "x" }

func Yes() bool { return true }

func Pointer() *int { return new(int) }

func Two() (int, error) { return 0, errors.New("x") }

func Named() (n int, err error) {
	n = 1
	return
}

func Touch(p *int) {
	*p = 1
}

func Generic[T any](x T) T { return x }

func Bound[T ~int]() T { return 0 }

// Pair's result type spans lines, and its erase mutant still adds none.
func Pair() struct {
	a int    // the count
	b string `tag:"two
lines"`
	c []int `json:"c"`
} {
	return struct {
		a int
		b string `tag:"two
lines"`
		c []int `json:"c"`
	}{1, "x", nil}
}
