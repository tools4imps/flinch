package native

// Value sits in a plain file that sorts after the package's cgo files.
func Value() int {
	return A - one() - two()
}
