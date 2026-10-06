package fix

// Root sits in the module's root package, whose ids start with "..".
func Root(a int) bool {
	return a == 1
}
