// Package calc is the code the run Contract tests mutate. Each function is called by the fixture tests
// its comment names, so a Contract test can say exactly which tests reach it.
package calc

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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

// Sign is called by TestSign, TestSignBig and TestSignNeg.
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

var bumps int

// Bump counts its calls in the process. TestBumpA and TestBumpB call it.
func Bump() int {
	bumps++
	return bumps
}

// Hold is called by TestHold1 and TestHold2.
func Hold() int { return 7 }

// Grow is called by TestGrow.
func Grow() int { return 64 }

// Linger is called by TestLinger.
func Linger() int { return 5 }

// Note appends a line to fx.log beside the module root, which the Contract tests read. The line names
// the event, the tests the process was asked to run, its time budget, the tag, the process id, the
// temp directory variables and the build cache flinch hands to nested runs. It reads the flags from
// os.Args, so it works in an init function too, before flags are parsed.
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
	fmt.Fprintf(f, "%s run=%s timeout=%s tag=%s pid=%d tmpdir=%s tmp=%s temp=%s cache=%s\n", event, run, timeout, Tag(), os.Getpid(),
		os.Getenv("TMPDIR"), os.Getenv("TMP"), os.Getenv("TEMP"), os.Getenv("FLINCH_GOCACHE"))
}

// freeze stops the process the first time it runs with a marker in a copy of the fixture, so Go's own
// timeout can't fire and only the runner's guard ends the process. No test calls it; a mutant does.
// When a stopped process's group is orphaned, Linux sends the group SIGHUP and then SIGCONT, so freeze
// ignores the one and stops again after the other.
func freeze(marker string) {
	if _, err := os.Stat("../../../" + marker); err == nil {
		return
	}
	os.WriteFile("../../../"+marker, nil, 0o644)
	signal.Ignore(syscall.SIGHUP)
	for {
		syscall.Kill(os.Getpid(), syscall.SIGSTOP)
	}
}

// hoard holds ever more memory, 1 MB every 10ms, touching each page so it counts. No test calls it;
// a mutant does.
func hoard() {
	var kept [][]byte
	for {
		b := make([]byte, 1<<20)
		for i := range b {
			b[i] = 1
		}
		kept = append(kept, b)
		time.Sleep(10 * time.Millisecond)
	}
}

// linger starts a process that outlives the test and keeps its output pipe open. No test calls it; a
// mutant does.
func linger() {
	cmd := exec.Command("sleep", "100")
	cmd.Stdout = os.Stdout
	cmd.Start()
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
