// Package mod is a fixture for package source.
package mod

import "strings"

// V has an initializer, so it is a unit.
var V = strings.ToUpper("v")

// W has no initializer, so it is not a unit.
var W int

type T struct{}

type (
	U int
	S string
)

func A() func() int {
	return func() int { return 1 }
}

func (T) M() {}

func (t *T) N() {}

func init() {}
