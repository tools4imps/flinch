package alpha_test

import (
	"fmt"
	"os"
	"testing"
)

// TestMain sets up the test binary. It is no Contract test, so it needs no tag.
func TestMain(m *testing.M) { os.Exit(m.Run()) }

// Contract: alpha/A1
func TestOne(t *testing.T) {}

// TestTwo checks two obligations.
//
// Contract: alpha/A1
// Contract: alpha/A2
func TestTwo(t *testing.T) {}

// Contract: alpha/A3
// Contract: alpha/A3
func Test(t *testing.T) {}

// Contract: alpha/A2
func Test_underscore(t *testing.T) {}

func Testify() {}

func helper() {}

type suite struct{}

func (suite) TestMethod(t *testing.T) {}

// Contract: alpha/A1
func ExampleOne() {
	fmt.Println("one")
	// Output: one
}

// Contract: alpha/A2
func ExampleUnordered() {
	fmt.Println("a")
	// Unordered output: a
}

// Contract: alpha/A3
func FuzzOne(f *testing.F) {}

// Contract: alpha/A3
func Example_lower() {
	// output:
}
