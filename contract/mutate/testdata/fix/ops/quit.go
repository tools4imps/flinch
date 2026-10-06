package ops

import "os"

// Quit's call is this file's only use of os, so removing it outright would strand the import.
func Quit(code int) {
	os.Exit(code)
}
