package x

import "C"

// Cgo sits in a file that imports "C".
func Cgo() int { return 9 }
