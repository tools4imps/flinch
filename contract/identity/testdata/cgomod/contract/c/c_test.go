package c_test

import (
	"testing"

	"example.com/cgomod/internal/c"
)

// Contract: c/N1
func TestEveryInitRuns(t *testing.T) {
	if c.A != 1 || c.B != 1 || c.Z != 1 {
		t.Fatal("an init function didn't run")
	}
}
