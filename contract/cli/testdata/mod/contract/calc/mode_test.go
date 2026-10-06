//go:build !fx

package calc_test

import (
	"testing"

	"example.com/fx/calc"
)

// Contract: calc/C2
func TestMode(t *testing.T) {
	if got := calc.Mode(); got != "plain" {
		t.Fatalf("Mode() = %q", got)
	}
}
