package calc_test

import (
	"testing"

	"example.com/run/calc"
)

// Contract: calc/K1
func TestAddSums(t *testing.T) {
	if got := calc.Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
}

// Contract: calc/K2
func TestSubSubtracts(t *testing.T) {
	if got := calc.Sub(5, 3); got != 2 {
		t.Errorf("Sub(5, 3) = %d, want 2", got)
	}
}
