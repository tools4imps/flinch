// Package calc is a tiny package for the runner's tests to mutate.
package calc

import (
	"os"
	"strings"
	"time"
)

// Table comes from a package-level initializer, where coverage puts no counters.
var Table = build()

func build() map[string]int { return map[string]int{"one": 1} }

var ready bool

func init() {
	ready = true
}

// Ready reports whether init ran.
func Ready() bool { return ready }

func Add(a, b int) int { return a + b }

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func Div(a, b int) int { return a / b }

func Sum(n int) int {
	s := 0
	for i := 0; i < n; i++ {
		s += i
	}
	return s
}

func Slow() int {
	return 1
}

func Trim(s string) string {
	return strings.TrimSpace(s)
}

// pause sleeps the first time it sees a marker path, and makes the file so later calls return at once.
// A mutant calls it from Slow to make a test slow on one run only.
func pause(marker string, d time.Duration) {
	if _, err := os.Stat(marker); marker == "" || err == nil {
		return
	}
	os.WriteFile(marker, nil, 0o644)
	time.Sleep(d)
}
