package ops

// Small's constant overflows a uint8 when its + becomes -.
func Small() uint8 {
	var u uint8 = 0 + 1
	return u
}

// Scale's * becomes a division by a constant zero.
func Scale(x int) int {
	return x * 0
}

// Must ends in a panic, so removing it leaves the function without a terminating statement, in the
// plain form and the fallback form alike.
func Must(ok bool) int {
	if ok {
		return 1
	}
	panic("not ok")
}
