package calc_test

import (
	"testing"

	"example.com/fx/calc"
)

// Contract: calc/C1
func TestAdd(t *testing.T) {
	if got := calc.Add(2, 3); got != 5 {
		t.Fatalf("Add(2, 3) = %d", got)
	}
}

// Contract: calc/C1
func TestReady(t *testing.T) {
	if !calc.Ready() || calc.Base != 42 {
		t.Fatalf("Ready() = %v, Base = %d", calc.Ready(), calc.Base)
	}
}

// Contract: calc/C1
func TestSign(t *testing.T) {
	for n, want := range map[int]string{-1: "negative", 0: "zero", 1: "positive"} {
		if got := calc.Sign(n); got != want {
			t.Errorf("Sign(%d) = %q, want %q", n, got, want)
		}
	}
}
