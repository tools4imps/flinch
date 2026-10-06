// Package calc is the code the run Contract tests mutate. Each function is called by the fixture tests
// its comment names, so a Contract test can say exactly which tests reach it.
package calc

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Table comes from a package-level initializer.
var Table = map[string]int{"one": 1}

var ready bool

func init() {
	ready = true
}

// Ready reports whether init ran. TestReady calls it.
func Ready() bool { return ready }

// Tag names the code a test ran against. Mutants change it so the log can tell their runs apart.
func Tag() string { return "clean" }

// Add is called by TestAdd, TestAddMore, TestParFast, the TestChainA tests and b's TestOtherAdd.
func Add(a, b int) int { return a + b }

// Abs is called by TestAbs.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Div is called by the TestChainZ tests.
func Div(a, b int) int { return a / b }

// Sum is called by TestSum and TestParSum.
func Sum(n int) int {
	s := 0
	for i := 0; i < n; i++ {
		s += i
	}
	return s
}

// Slow is called by TestSlow.
func Slow() int {
	return 1
}

// Trim is called by TestReadsTestdata.
func Trim(s string) string {
	return strings.TrimSpace(s)
}

// Sign is called by TestSign, TestSignBig and TestSignNeg. Coverage puts no block over its case
// expressions.
func Sign(x int) int {
	switch {
	case x < 0:
		return -1
	case x > 100:
		return 3
	case x > 0:
		switch {
		case x > 9:
			return 2
		}
		return 1
	}
	return 0
}

// Twice is called by TestTwice. Coverage puts no block over the condition after its function literal.
func Twice(x int) int {
	if y := func() int { return 2 * x }(); y > 9 {
		return 9
	}
	return 2 * x
}

var bumps int

// Bump counts its calls in the process. TestBumpA and TestBumpB call it.
func Bump() int {
	bumps++
	return bumps
}

// Pick is called by TestPick.
func Pick(ch chan int) int {
	select {
	case v := <-ch:
		return v
	default:
		return -1
	}
}

// Hold is called by TestHold1 and TestHold2.
func Hold() int { return 7 }

// Note appends a line to fx.log beside the module root, which the Contract tests read. The line names
// the event, the tests the process was asked to run, its time budget, the tag and the process id. It
// reads the flags from os.Args, so it works in an init function too, before flags are parsed.
func Note(event string) {
	run, timeout := "", ""
	for _, a := range os.Args[1:] {
		if v, ok := strings.CutPrefix(a, "-test.run="); ok {
			run = v
		}
		if v, ok := strings.CutPrefix(a, "-test.timeout="); ok {
			timeout = v
		}
	}
	f, err := os.OpenFile("../../../fx.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "%s run=%s timeout=%s tag=%s pid=%d\n", event, run, timeout, Tag(), os.Getpid())
}

// freeze stops the process the first time it runs with a marker in a copy of the fixture, so Go's own
// timeout can't fire and only the runner's guard ends the process. No test calls it; a mutant does.
func freeze(marker string) {
	if _, err := os.Stat("../../../" + marker); err == nil {
		return
	}
	os.WriteFile("../../../"+marker, nil, 0o644)
	syscall.Kill(os.Getpid(), syscall.SIGSTOP)
}

// pause sleeps the first time it runs with a marker in a copy of the fixture and leaves the marker
// beside the module root, so later calls return at once. No test calls it; mutants do.
func pause(marker string, d time.Duration) {
	if _, err := os.Stat("../../../" + marker); err == nil {
		return
	}
	os.WriteFile("../../../"+marker, nil, 0o644)
	time.Sleep(d)
}
