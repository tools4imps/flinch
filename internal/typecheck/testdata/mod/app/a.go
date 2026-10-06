package app

import "strings"

type box struct{ s string }

func (b box) Upper() string { return strings.ToUpper(b.s) }
