package ops

func Flags() (bool, bool) {
	return true, false
}

// Not negates twice, and each ! goes on its own.
func Not(ok, done bool) bool {
	return !ok && !(done)
}

// odd has a field called true, which isn't the predeclared true.
type odd struct{ true bool }

func Odd(o odd) bool { return o.true }

func Make() odd { return odd{true: false} }

// Shadow declares a local called true.
func Shadow() int {
	true := 1
	return true
}
