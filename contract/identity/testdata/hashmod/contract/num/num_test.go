package num_test

import (
	"testing"

	"example.com/hashmod/internal/num"
)

// Contract: num/N1
func TestPositive(t *testing.T) {
	if !num.Positive(5) || num.Positive(-5) {
		t.Error("Positive has the sign wrong")
	}
}
