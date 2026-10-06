package app

import (
	"net/http"

	"example.com/mod/lib"
)

// Twice reaches into another module package and into net/http, whose own imports go through the
// standard library's vendor directory.
func Twice(x int) int {
	n := lib.Double(x)
	_ = http.StatusOK
	return n
}
