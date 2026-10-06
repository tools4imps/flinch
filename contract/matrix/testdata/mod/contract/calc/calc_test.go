package calc_test

import (
	"testing"

	"example.com/fx/calc"
)

// Contract: calc/C1
func TestMax(t *testing.T) {
	if got := calc.Max(2, 5); got != 5 {
		t.Errorf("Max(2, 5) = %d, want 5", got)
	}
	if got := calc.Max(7, 3); got != 7 {
		t.Errorf("Max(7, 3) = %d, want 7", got)
	}
}
