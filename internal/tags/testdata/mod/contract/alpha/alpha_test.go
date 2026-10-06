package alpha_test

import (
	"fmt"
	"os"
	"testing"
)

// TestOne carries two tags and repeats one.
//
// Contract: alpha/A1
// Contract: alpha/A2
// Contract: alpha/A1
func TestOne(t *testing.T) {}

// Contract: alpha/A1
// Contract: beta/B1
func TestReachesAcross(t *testing.T) {}

func TestUntagged(t *testing.T) {}

// Contract: alpha/A9
func TestUnknown(t *testing.T) {}

// Contract: alpha/a1
func TestMalformed(t *testing.T) {}

// Contract: alpha/A2
func ExampleNoOutput() {
	fmt.Println("hi")
}

// Contract: alpha/A2
func ExampleWithOutput() {
	fmt.Println("hi")
	// Output: hi
}

// Contract: alpha/A2
func ExampleUnordered() {
	fmt.Println("hi")
	// unordered output: hi
}

// ExampleUntaggedNoOutput is untagged, so only the missing tag is reported.
func ExampleUntaggedNoOutput() {}

// Contract: alpha/A1
func FuzzOne(f *testing.F) {}

// Contract: alpha/A1
func Test_underscore(t *testing.T) {}

// Contract: alpha/A1
func Testify(t *testing.T) {}

// Contract: alpha/A1
func TestMain(m *testing.M) { os.Exit(m.Run()) }

type suite struct{}

// Contract: alpha/A1
func (suite) TestMethod(t *testing.T) {}

// Contract: alpha/A1

func TestDetached(t *testing.T) {
	// Contract: alpha/A1
}
