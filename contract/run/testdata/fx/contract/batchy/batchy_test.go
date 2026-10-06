package batchy_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"example.com/fx/calc"
)

var dirty bool

// TestX passes alone and in the whole run, where TestZ cleans up after TestY, and fails in a batch
// with TestY alone.
func TestY(t *testing.T) {
	calc.Note(t.Name())
	dirty = true
}

func TestZ(t *testing.T) {
	calc.Note(t.Name())
	dirty = false
}

// TestSpoil passes. When its process was asked to run more than one test, as a batch's runs are, it
// takes away its own binary's permission to execute, so no process can start from that binary again.
func TestSpoil(t *testing.T) {
	calc.Note(t.Name())
	if slices.ContainsFunc(os.Args, func(a string) bool { return strings.HasPrefix(a, "-test.run=") && strings.Contains(a, "|") }) {
		os.Chmod(os.Args[0], 0o644)
	}
}

func TestSlowSpoil(t *testing.T) {
	calc.Note(t.Name())
	if got := calc.Slow(); got != 1 {
		t.Errorf("Slow() = %d", got)
	}
}

func TestX(t *testing.T) {
	calc.Note(t.Name())
	if dirty {
		t.Error("ran after TestY with no TestZ between")
	}
	if calc.Add(1, 1) != 2 {
		t.Error("Add(1, 1) != 2")
	}
}
