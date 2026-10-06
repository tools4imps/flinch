// Package old is in a module that says go 1.21, so a mutant may only use what Go 1.21 has.
package old

func Total(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}
