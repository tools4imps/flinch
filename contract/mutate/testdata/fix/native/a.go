package native

// A sits in a plain file that sorts before the package's cgo files.
var A int

func Get() int {
	return A + 1
}
