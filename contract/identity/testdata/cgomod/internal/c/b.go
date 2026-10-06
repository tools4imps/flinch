package c

import "C"

// B counts what b.go's init did.
var B int

func init() { B = B + 1 }
