package run_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/runner"
)

// hog fills 512 MB and then waits forever. The amount is bounded on purpose: if the memory guard
// ever broke, Go's own timeout would end the process before it could take the machine down.
const hog = "x := make([]byte, 512<<20); for i := range x { x[i] = 1 }; for { time.Sleep(time.Second) }"

// proveSlow proves suite a's TestSlow against a fresh copy of the fixture.
func proveSlow(t *testing.T, o runner.Options) (string, *runner.Baseline) {
	t.Helper()
	root, err := copyFixture("fx")
	if err != nil {
		t.Fatal(err)
	}
	o.Root = root
	o.Jobs = 1
	o.Work = filepath.Join(filepath.Dir(root), "work")
	b, err := runner.Prove(context.Background(), o, []runner.Suite{suite("a", "TestSlow")}, []string{fx + "/calc"})
	if err != nil {
		t.Fatal(err)
	}
	return root, b
}

// Contract: run/R13
func TestATestProcessPastTheMemoryLimitIsStoppedAsACrash(t *testing.T) {
	root, b := proveSlow(t, runner.Options{MemoryLimit: 64 << 20})
	m, err := mutant(root, "calc/calc.go", "return 1", hog, "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}})
	if err != nil {
		t.Fatal(err)
	}
	if row := rows[m.Hash]; !row.Complete || !slices.Equal(kills(row), []string{"a/TestSlow panic"}) {
		t.Errorf("row = %+v, want a/TestSlow killed by a crash", row)
	}
}

// Contract: run/R14
func TestTestsKeepTheirTempFilesAndMutantBuildsInsideTheRun(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("FLINCH_GOCACHE", cache)
	root, b := proveSlow(t, runner.Options{})
	// The mutant writes down where its temp directory and build cache are, in a file in that temp
	// directory, and then fails TestSlow so the run has something to report.
	body := `os.WriteFile(os.TempDir()+"/where", []byte(os.Getenv("FLINCH_GOCACHE")), 0o644); return 2`
	m, err := mutant(root, "calc/calc.go", "return 1", body, "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}})
	if err != nil {
		t.Fatal(err)
	}
	if row := rows[m.Hash]; !slices.Equal(kills(row), []string{"a/TestSlow assertion"}) {
		t.Fatalf("row = %+v, want a/TestSlow killed", row)
	}
	where, err := os.ReadFile(filepath.Join(filepath.Dir(root), "work", "tmp", "where"))
	if err != nil {
		t.Fatalf("the test's temp directory isn't inside the run's work directory: %v", err)
	}
	if string(where) != cache {
		t.Errorf("the test saw FLINCH_GOCACHE=%q, want the run's own %q", where, cache)
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) == 0 {
		t.Errorf("the mutant wasn't built in the run's cache %s: %v", cache, err)
	}
}

// Contract: run/R14
func TestARunWithNoOuterCacheBuildsMutantsInItsWorkDirectory(t *testing.T) {
	t.Setenv("FLINCH_GOCACHE", "")
	root, b := proveSlow(t, runner.Options{})
	m, err := mutant(root, "calc/calc.go", "return 1", "return 2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(root), "work", "gocache"))
	if err != nil || len(entries) == 0 {
		t.Errorf("the mutant wasn't built in the work directory's cache: %v", err)
	}
}

// Contract: run/R15
func TestFilesATestLeavesInItsDirectoryAreGoneAndLaterMutantsStillBuild(t *testing.T) {
	root, b := proveSlow(t, runner.Options{})
	// The first mutant makes TestSlow write a .go file of another package and a text file into the
	// directory it runs in, which is the suite's own, as a mutant writing by relative path would.
	litter := `os.WriteFile("stray.go", []byte("package elsewhere\n"), 0o644); os.WriteFile("note.txt", nil, 0o644); return 2`
	first, err := mutant(root, "calc/calc.go", "return 1", litter, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := mutant(root, "calc/calc.go", "return 1", "return 3", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []model.Mutant{first, second} {
		rows, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}})
		if err != nil {
			t.Fatal(err)
		}
		if row := rows[m.Hash]; row.Verdict != "" || !slices.Equal(kills(row), []string{"a/TestSlow assertion"}) {
			t.Errorf("%s: row = %+v, want a/TestSlow killed and a verdict", m.Text, row)
		}
	}
	for _, name := range []string{"stray.go", "note.txt"} {
		if _, err := os.Stat(filepath.Join(root, "contract", "a", name)); err == nil {
			t.Errorf("%s is still in the suite's directory after the run", name)
		}
	}
}

// Contract: run/R15
func TestFilesATestLeavesStayWhileAnotherProcessOfItsSuiteRuns(t *testing.T) {
	root, err := copyFixture("fx")
	if err != nil {
		t.Fatal(err)
	}
	// TestNap1's lone run takes 200ms, so a coefficient of 100 gives each mutant about 20 seconds,
	// time enough to wait for the other one.
	o := runner.Options{Root: root, Jobs: 2, Coefficient: 100, Work: filepath.Join(filepath.Dir(root), "work")}
	b, err := runner.Prove(context.Background(), o, []runner.Suite{suite("a", "TestNap1")}, []string{fx + "/calc"})
	if err != nil {
		t.Fatal(err)
	}
	// Both mutants change Note, which TestNap1 calls first. The keeper leaves keep.txt in the suite's
	// directory, waits until the other process is done, and panics if keep.txt went in the meantime.
	// The other waits for the keeper to start before it finishes, so it ends while the keeper still runs.
	wait := func(marker string) string {
		return `for i := 0; i < 400; i++ { if _, err := os.Stat("../../../` + marker + `"); err == nil { break }; time.Sleep(25 * time.Millisecond) }; `
	}
	const orig = `run, timeout := "", ""`
	keeper, err := mutant(root, "calc/calc.go", orig, `os.WriteFile("keep.txt", nil, 0o644); os.WriteFile("../../../keeper", nil, 0o644); `+
		wait("other")+`time.Sleep(2 * time.Second); if _, err := os.Stat("keep.txt"); err != nil { panic("keep.txt went while its test ran") }; `+orig, "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := mutant(root, "calc/calc.go", orig, wait("keeper")+`os.WriteFile("../../../other", nil, 0o644); `+orig, "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := b.Run(context.Background(), []runner.Job{
		{Mutant: keeper, Tests: refs("a/TestNap1")},
		{Mutant: other, Tests: refs("a/TestNap1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "other")); err != nil {
		t.Fatal("the other mutant's test never ran")
	}
	if row := rows[keeper.Hash]; !row.Complete || len(row.Kills) != 0 {
		t.Errorf("keeper: row = %+v, want TestNap1 passed with keep.txt still there", row)
	}
	if _, err := os.Stat(filepath.Join(root, "contract", "a", "keep.txt")); err == nil {
		t.Error("keep.txt is still in the suite's directory after the run")
	}
}

// Contract: run/R14
func TestTheRunsOwnCachePastItsLimitIsEmptiedBetweenChunks(t *testing.T) {
	t.Setenv("FLINCH_GOCACHE", "")
	root, b := proveSlow(t, runner.Options{CacheLimit: 1})
	m, err := mutant(root, "calc/calc.go", "return 1", "return 2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "work", "gocache")); err == nil {
		t.Error("the run's cache outgrew a one-byte limit and is still there after the chunk")
	}
}

// Contract: run/R14
func TestTheRunsOwnCacheIsMeasuredByTheFilesItHolds(t *testing.T) {
	t.Setenv("FLINCH_GOCACHE", "")
	// A fresh cache holds tens of megabytes of compiled packages for one mutant, in 256 directories
	// that take a megabyte at most. A 16 MB limit sits between the two.
	root, b := proveSlow(t, runner.Options{CacheLimit: 16 << 20})
	m, err := mutant(root, "calc/calc.go", "return 1", "return 2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "work", "gocache")); err == nil {
		t.Error("the run's cache held more than 16 MB of compiled packages and is still there after the chunk")
	}
}

// Contract: run/R14
func TestASharedCacheIsNeverEmptiedByTheRunThatBorrowsIt(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("FLINCH_GOCACHE", cache)
	root, b := proveSlow(t, runner.Options{CacheLimit: 1})
	m, err := mutant(root, "calc/calc.go", "return 1", "return 2", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Run(context.Background(), []runner.Job{{Mutant: m, Tests: refs("a/TestSlow")}}); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(cache); err != nil || len(entries) == 0 {
		t.Errorf("a run emptied the outer run's cache it was only borrowing: %v", err)
	}
}
