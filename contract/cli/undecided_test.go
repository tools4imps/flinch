package cli_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
)

// Outside a module, or when a file doesn't parse or a directory can't be read, there's nothing
// flinch can check, so it exits 2 and says why on stderr.
//
// Contract: cli/L11
func TestCantDecideWhenItCantReadTheModule(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, args := range [][]string{nil, {"contract"}} {
		r := flinch(args...)
		if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "go.mod") {
			t.Errorf("flinch %s outside a module should exit 2 and say so:\n%s", strings.Join(args, " "), r)
		}
	}

	mod(t)
	write(t, "calc/broken.go", "package calc\n\nfunc {\n")
	if r := flinch("contract"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "broken.go") {
		t.Errorf("a file that doesn't parse should leave the check undecided:\n%s", r)
	}
	if err := os.Remove("calc/broken.go"); err != nil {
		t.Fatal(err)
	}

	// Each locked directory is empty, so the temp directory's cleanup can remove it even if the
	// test stops before unlocking it.
	lock := func(name string) func() {
		if err := os.Mkdir(name, 0o000); err != nil {
			t.Fatal(err)
		}
		return func() {
			if err := os.Chmod(name, 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	unlock := lock("calc/locked")
	if r := flinch("contract"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "locked") {
		t.Errorf("an unreadable package directory should leave the check undecided:\n%s", r)
	}
	unlock()
	// A Contract whose name starts with an underscore is one the package walk skips, so only the
	// Contract loader meets its locked directory.
	if err := os.Rename("contract", "_contract"); err != nil {
		t.Fatal(err)
	}
	unlock = lock("_contract/weak/locked")
	if r := flinch("contract", "--contract", "_contract"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "locked") {
		t.Errorf("an unreadable Contract directory should leave the check undecided:\n%s", r)
	}
	unlock()
}

// A type error in covered code, a missing go command and a temp directory flinch can't write to
// each leave a run undecided.
//
// Contract: cli/L11
func TestCantDecideWithoutABuild(t *testing.T) {
	dir := mod(t)
	write(t, "calc/broken.go", "package calc\n\nvar broken int = \"not an int\"\n")
	if r := flinch("--dry-run"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "broken.go") {
		t.Errorf("a type error in covered code should leave the run undecided:\n%s", r)
	}
	if err := os.Remove("calc/broken.go"); err != nil {
		t.Fatal(err)
	}

	// The go command keeps a temp directory of its own, so only flinch's work directory fails.
	goTmp := t.TempDir()
	t.Setenv("GOTMPDIR", goTmp)
	t.Setenv("TMPDIR", dir+"/missing")
	if r := flinch("run"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "missing") {
		t.Errorf("with no temp directory for its work, a run should be undecided:\n%s", r)
	}

	t.Setenv("PATH", goTmp)
	if r := flinch("run"); r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "go command") {
		t.Errorf("with no go command on PATH, a run should be undecided:\n%s", r)
	}
}

// A Contract test that fails on clean code means flinch can't decide. The run exits 2, and the
// report it prints names the test.
//
// Contract: cli/L11
func TestCantDecideWhenASuiteFailsOnCleanCode(t *testing.T) {
	mod(t)
	edit(t, "contract/calc/calc_test.go", "got != 5", "got != 6")
	r := flinch("run", "--format", "json", "--jobs", "2")
	if r.code != 2 {
		t.Fatalf("a failing Contract test should leave the run undecided:\n%s", r)
	}
	rep := parse(t, r)
	if rep.Exit != 2 || !strings.Contains(rep.Undecided, "TestAdd") || len(rep.Mutants) != 0 {
		t.Errorf("the report should name TestAdd and hold no mutants:\n%s", r)
	}
}

// cancelOn is a progress writer that cancels a run the first time a progress line holds marker,
// which interrupts it at a known stage the way Ctrl-C would.
type cancelOn struct {
	mu     sync.Mutex
	marker string
	cancel context.CancelFunc
}

func (c *cancelOn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.Contains(string(p), c.marker) {
		c.cancel()
	}
	return len(p), nil
}

// A run interrupted while it proves the suite green, while it runs the erase mutants, or while it
// runs the rest returns an error and no report, which the command line prints on stderr with exit 2.
//
// Contract: cli/L11
func TestAnInterruptedRunHasNoVerdict(t *testing.T) {
	dir := modAt(t)
	// The fixture has 7 reached erase mutants and 6 other mutants that run, and the first progress
	// line of each batch counts its first finished mutant.
	for _, marker := range []string{"Contract suites whole", "1/7 mutants", "1/6 mutants"} {
		ctx, cancel := context.WithCancel(context.Background())
		rep, err := engine.Run(ctx, engine.RunOptions{
			Options: engine.Options{Dir: dir}, Jobs: 2, Coefficient: 10,
			Progress: &cancelOn{marker: marker, cancel: cancel},
		})
		cancel()
		if err == nil || rep != nil {
			t.Errorf("a run interrupted at %q returned a report and error %v, want only an error", marker, err)
		}
	}
}
