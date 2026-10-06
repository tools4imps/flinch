package calc_test

import (
	"testing"

	"example.com/e2e/calc"
)

var leaked bool

// Contract: calc/A1
func TestLeaky(t *testing.T) {
	leaked = true
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
