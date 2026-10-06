// Package num gives the Contract test in this fixture something to kill and something to miss.
package num

// Positive reports whether n is above zero.
func Positive(n int) bool { return n > 0 }

// Negative reports whether n is below zero. No Contract test calls it.
func Negative(n int) bool { return n < 0 }
