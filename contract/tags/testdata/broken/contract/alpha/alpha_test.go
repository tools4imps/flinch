package alpha_test

import "testing"

// Contract: alpha/A1
func TestFine(t *testing.T) {}

func TestUntagged(t *testing.T) {}

// Contract: beta/B2
func TestCrossing(t *testing.T) {}

// Contract: alpha/A9
func TestUnknown(t *testing.T) {}

// Contract: alpha/A2

func TestSeparated(t *testing.T) {}

// Contract: alpha A2
func TestMisspelled(t *testing.T) {}

// Contract: alpha/A2
func ExampleSilent() {}

// Contract: alpha/A2
func ExampleLate() {
	// Output: late
	println("late")
	// a note after the output
}

func ExampleUntagged() {}

// Contract: alpha/A2
func ExampleBodiless()

// Contract: nosuch/N1
func TestNoSuchPrimitive(t *testing.T) {}
