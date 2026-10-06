package par_test

import (
	"testing"

	"example.com/fx/calc"
)

func TestParFast(t *testing.T) {
	t.Parallel()
	if got := calc.Add(1, 2); got != 3 {
		t.Errorf("Add(1, 2) = %d", got)
	}
}

func TestParSum(t *testing.T) {
	t.Parallel()
	if got := calc.Sum(3); got != 3 {
		t.Errorf("Sum(3) = %d", got)
	}
}
