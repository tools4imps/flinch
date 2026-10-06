package ops

// limit has an initializer, so it's a unit.
var limit = 10 * 2

// count has no initializer, so it isn't one.
var count int

// positive is a function literal in an initializer.
var positive = func(x int) bool { return x > 0 }

// A spec is named by its first name that isn't blank.
var _, second = 1, 2 + 3

// A spec whose names are all blank is called _.
var _ = 4 - 1

var (
	grouped = 6 * 7
	nested  = func() func() bool { return func() bool { return true } }
)

func init() {
	limit++
}

func init() {
	count--
}

type List[T any] struct{ items []T }

func (l *List[T]) Len() int { return len(l.items) }

type Pair2[K comparable, V any] struct {
	k K
	v V
}

func (p Pair2[K, V]) Key() K { return p.k }

type plain struct{ n int }

func (p *plain) Bump() { p.n++ }

func (plain) Same() bool { return true }

// Count numbers its function literals in source order, nested ones included.
func Count() int {
	a := func() int {
		b := func() int { return 1 }
		return b() + 1
	}
	c := func() int { return 2 }
	return a() * c()
}
