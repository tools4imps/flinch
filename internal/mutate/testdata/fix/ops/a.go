package ops

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
)

// limit has an initializer, which coverage never counts.
var limit = 10 * 2

// count has no initializer, so it isn't a unit.
var count int

// positive is a closure in an initializer, which coverage does count.
var positive = func(x int) bool { return x > 0 }

func init() {
	limit++
}

func Arith(a, b int) int {
	sum := a + b
	diff := a - b
	prod := a * b
	quot := a / b
	rem := a % b
	return sum + diff + prod + quot + rem
}

func Concat(a, b string) string {
	s := a + b
	s += "!"
	return s
}

func Compare(a, b int) bool {
	if a < b || a <= 0 {
		return a == b && a != 0
	}
	return !(a > b) && a >= 1
}

func Flags() (bool, bool) {
	return true, false
}

func Step(n int) (total int) {
	for i := 0; i < n; i++ {
		total += i
	}
	n--
	total -= n
	return
}

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

type myErr struct{}

func (*myErr) Error() string { return "mine" }

func Concrete() *myErr { return &myErr{} }

func Calls(w io.Writer, l *log.Logger, s *slog.Logger) {
	log.Printf("a")
	l.Println("b")
	slog.Info("c")
	s.Info("d")
	fmt.Fprintln(w, "e")
	fmt.Fprintf(w,
		"%d", 1)
}

func Strand(w io.Writer) {
	x := 1
	fmt.Fprint(w, x)
}

// Wrap's error is the only use of msg, so making it nil outright strands the local.
func Wrap(n int) error {
	msg := fmt.Sprint(n)
	return errors.New(
		msg)
}

func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

type Number interface{ ~int | ~float64 }

func Sum[T Number](xs []T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Join[T ~int | ~string](a, b T) T {
	return a + b
}

type List[T any] struct{ items []T }

func (l *List[T]) Len() int { return len(l.items) }

func Counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

func Twice(x int) bool {
	a := x > 0
	b := x > 0
	return a && b
}

type odd struct{ true bool }

func Odd(o odd) bool { return o.true }

func Nothing() {}

func Pair() struct {
	a int
	b string
} {
	return struct {
		a int
		b string
	}{1, "x"}
}
