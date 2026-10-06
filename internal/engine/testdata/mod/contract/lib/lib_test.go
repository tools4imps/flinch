package lib_test

import (
	"testing"

	"example.com/e/lib"
)

// Contract: lib/L1
func TestAdd(t *testing.T) {
	if lib.Add(1, 2) != 3 {
		t.Fatal("1 + 2 isn't 3")
	}
}

func TestUntagged(t *testing.T) {}
