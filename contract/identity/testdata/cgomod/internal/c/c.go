package c

// Z counts what c.go's init did.
var Z int

func init() { Z = Z + 1 }
