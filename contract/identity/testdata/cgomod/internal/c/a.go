// Package c mixes plain files and a cgo file, each with an init function.
package c

// A counts what a.go's init did.
var A int

func init() { A = A + 1 }
