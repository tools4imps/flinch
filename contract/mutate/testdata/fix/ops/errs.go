package ops

import (
	"errors"
	"fmt"
)

var errBad = errors.New("bad")

func Errors(n int) (int, error) {
	if n < 0 {
		return 0, errBad
	}
	if n == 0 {
		return 0, fmt.Errorf("zero %d",
			n)
	}
	return n, nil
}

// myErr is a concrete error type. An expression of that type isn't of type error.
type myErr struct{}

func (*myErr) Error() string { return "mine" }

func Concrete() *myErr { return &myErr{} }

// Boxing returns a *myErr as an error, but the expression's own type is *myErr.
func Boxing() error { return &myErr{} }

// Pass returns another call's results whole.
func Pass() (int, error) { return Errors(1) }

// Wrap's error is the only use of msg, so making it nil outright would strand the local.
func Wrap(n int) error {
	msg := fmt.Sprint(n)
	return errors.New(
		msg)
}

// Later returns an error from inside a function literal.
func Later() func() error {
	return func() error { return errBad }
}
