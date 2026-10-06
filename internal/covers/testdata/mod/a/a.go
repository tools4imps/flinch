package a

func F() { func() {}() }

func G() {}

type T struct{}

func (T) M() {}

func (*T) N() {}

type E struct{}

var V = 1

var X, Y = 1, 2

var Z int

const K = 1

func init() {}
