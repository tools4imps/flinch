package ops

import "strconv"

// Greet concatenates strings, which arithmetic and step leave alone.
func Greet(name string) string {
	s := "hello, " + name
	s += "!"
	return s
}

// Label is a defined string type, so its + concatenates too.
type Label string

func Tag(l Label) Label {
	return l + "#"
}

// Mixed adds numbers inside a concatenation, and only the numbers' + changes.
func Mixed(name string, n int) string {
	return name + strconv.Itoa(n+1)
}
