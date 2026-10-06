package alone_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"example.com/fx/calc"
)

// lone reports whether the process was asked to run named tests, as a lone run is.
func lone() bool {
	for _, a := range os.Args {
		if strings.HasPrefix(a, "-test.run=") {
			return true
		}
	}
	return false
}

// TestAaSlow is still running alone when TestDepends fails alone.
func TestAaSlow(t *testing.T) {
	calc.Note(t.Name())
	if lone() {
		time.Sleep(20 * time.Second)
	}
}

var setUp bool

func TestSetup(t *testing.T) {
	calc.Note(t.Name())
	setUp = true
}

// TestDepends passes in the whole run, after TestSetup, and fails alone.
func TestDepends(t *testing.T) {
	calc.Note(t.Name())
	if lone() {
		// Any test that starts alone beside it has written its log line by the time it fails.
		time.Sleep(time.Second)
	}
	if !setUp {
		t.Error("needs TestSetup to run first")
	}
}
