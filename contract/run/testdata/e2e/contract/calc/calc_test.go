package calc_test

import (
	"os"
	"testing"

	"example.com/e2e/calc"
)

var leaked bool

// Contract: calc/A1
func TestLeaky(t *testing.T) {
	leaked = true
	// It also leaves a file in the temp directory.
	if f, err := os.CreateTemp("", "e2e-leftover-"); err == nil {
		f.Close()
	}
	if calc.Add(1, 1) != 2 {
		t.Error("Add(1, 1) != 2")
	}
}

// TestVictim passes alone and fails when it runs after TestLeaky, as it does in the whole suite.
//
// Contract: calc/A1
func TestVictim(t *testing.T) {
	if leaked {
		t.Error("TestLeaky ran first")
	}
	if calc.Add(2, 2) != 4 {
		t.Error("Add(2, 2) != 4")
	}
}
