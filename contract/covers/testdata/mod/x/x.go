// Package x holds every kind of unit a covers reference can name.
package x

// F is a function that a reference names by itself.
func F() int { return 1 }

// G is a function that no reference names by itself.
func G() int { return 2 }

// T is a type whose methods go with it.
type T struct{}

// M has a value receiver.
func (T) M() int { return 3 }

// P has a pointer receiver.
func (*T) P() int { return 4 }

// U is a type whose methods are named one at a time.
type U struct{}

// N has a value receiver.
func (U) N() int { return 5 }

// O has a pointer receiver.
func (*U) O() int { return 6 }

// Box is a generic type.
type Box[E any] struct{ e E }

// Get has a generic pointer receiver.
func (b *Box[E]) Get() E { return b.e }

// V is a package-level variable with an initializer.
var V = F()

// W has no initializer, so it holds nothing to mutate.
var W int

// The unit of this spec is B, its first name that isn't "_".
var _, B = 1, F()

// The unit of this spec is First, and naming Second names it too.
var First, Second = F(), G()

func init() { _ = G() }
