package weak_test

import (
	"testing"

	"example.com/fx/weak"
)

// Contract: weak/W1
func TestPositive(t *testing.T) {
	weak.Positive(3)
}

// Contract: weak/W1
func TestDouble(t *testing.T) {
	if got := weak.Double(4); got != 8 {
		t.Fatalf("Double(4) = %d", got)
	}
}
