package ops

import "os"

// Quit's call is this file's only use of os, so removing it outright strands the import.
func Quit(code int) {
	os.Exit(code)
}

// Must ends in a panic, so removing it leaves the function without a terminating statement, in the
// plain form and the fallback form alike.
func Must(ok bool) int {
	if ok {
		return 1
	}
	panic("not ok")
}
