package run_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/runner"
)

// The fixture's suites, with their top-level tests in source order.
var (
	aTests = []string{
		"TestAdd", "TestAddMore", "TestAbs", "TestSum", "TestAfterSum1", "TestAfterSum2", "TestSlow", "TestSlowToo",
		"TestNap1", "TestNap2", "TestTable", "TestReady", "TestReadsTestdata", "TestSign", "TestSignBig", "TestSignNeg", "TestTwice", "TestBumpA", "TestBumpB", "TestPick", "TestHold1", "TestHold2", "TestQuiet1", "TestQuiet2",
		"TestChainA1", "TestChainZ1", "TestChainA2", "TestChainZ2", "TestChainA3", "TestChainZ3",
		"TestChainA4", "TestChainZ4", "TestChainA5", "TestParFast", "TestParSum", "TestTagged",
	}
	bTests = []string{"TestOtherAdd", "TestShout", "ExampleShout", "FuzzShout"}
)

// naps is the batch three mutants share. Its two naps give it a lone run time large enough to read in
// its budget, and its size makes the order its tests are listed in matter to any runner that cares.
var naps = refs("a/TestNap1", "a/TestNap2", "a/TestQuiet1", "a/TestQuiet2", "a/TestAfterSum1", "a/TestAfterSum2")

const (
	calcGo  = "calc/calc.go"
	wordsGo = "words/words.go"
)

// A mainRun is what the main scenario recorded: one Prove over suites a and b with coverage, a Run
// of every job below, a second Run that reuses a batch, an interrupted Run and a Run whose context had
// already ended.
type mainRun struct {
	root          string
	base          *runner.Baseline
	jobs          map[string]runner.Job
	rows          map[string]model.Row // by job name
	again         map[string]model.Row // the second Run, by job name
	interrupted   error                // what the interrupted Run returned
	canceled      error                // what the Run with an ended context returned
	log           []entry
	before, after map[string]string // the module's files before Prove and after the last Run
}

var mainScenario = newScenario(func(ctx context.Context) (*mainRun, error) {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	root, err := copyFixture("fx")
	if err != nil {
		return nil, err
	}
	go guardLog(ctx, root, 2000, stop)
	if err := gitInit(ctx, root); err != nil {
		return nil, err
	}
	r := &mainRun{root: root, jobs: map[string]runner.Job{}, rows: map[string]model.Row{}, again: map[string]model.Row{}}
	if r.before, err = snapshot(root); err != nil {
		return nil, err
	}
	o := runner.Options{Root: root, Jobs: 4, Work: filepath.Join(filepath.Dir(root), "work"), Progress: io.Discard, Tags: []string{"fxtag"}}
	r.base, err = runner.Prove(ctx, o, []runner.Suite{suite("a", aTests...), suite("b", bTests...)}, []string{fx + "/calc", fx + "/words"})
	if err != nil {
		return nil, err
	}

	linked := r.base.Linking(fx + "/calc")
	specs := []struct {
		name, orig, repl, tag string
		tests                 []model.TestRef
	}{
		// TestHold1 stops its process the first time, which only the runner's guard can end.
		{"guard", "return 7", `freeze("fx.freeze"); return 7`, "", refs("a/TestHold1", "a/TestHold2")},
		{"serial timeout", "i++", "i--", "", refs("a/TestSum", "a/TestAfterSum1", "a/TestAfterSum2")},
		{"parallel timeout", "i++", "i--", " #par", refs("a/TestParFast", "a/TestParSum")},
		{"timeout once", "return 1", `pause("fx.pause", 6*time.Second); return 1`, "", refs("a/TestSlow")},
		{"no test starts", "ready = true", `Note("stall"); println("\x16PASS"); os.Exit(0)`, "", refs("a/TestAdd", "a/TestReady")},
		{"assertions", "a + b", "a - b", "", refs("a/TestAdd", "a/TestAddMore", "b/TestOtherAdd")},
		// Add(2, 3) and Add(-1, 1) come out wrong, and Add(1, 1) divides by zero.
		{"mixed kinds", "a + b", "a + b/(a*b-1)", "", refs("a/TestAdd", "a/TestAddMore", "a/TestChainA1")},
		// TestSlowToo times out the first time, and every process after that exits without settling it.
		{"rerun stalls", "return 1", `pause("fx.stall", 6*time.Second); println("\x16PASS"); os.Exit(1); return 1`, "", refs("a/TestSlowToo")},
		{"subtests", "return -x", "return x", "", refs("a/TestAbs")},
		// TestBumpB fails only after TestBumpA has run in the same process.
		{"company", "bumps++", "bumps += 2", "", refs("a/TestBumpA", "a/TestBumpB")},
		{"panics", "a / b", "a / (b - b)", "", refs(
			"a/TestChainA1", "a/TestChainZ1", "a/TestChainA2", "a/TestChainZ2", "a/TestChainA3",
			"a/TestChainZ3", "a/TestChainA4", "a/TestChainZ4", "a/TestChainA5")},
		{"init panics", "ready = true", `Note("init"); panic("init")`, "", linked},
		{"initializer", `"one": 1`, `"one": 2`, "", linked},
		{"exit", "return strings.TrimSpace(s)", "os.Exit(3); return strings.TrimSpace(s)", "",
			refs("a/TestReadsTestdata", "a/TestQuiet1", "a/TestQuiet2")},
		{"naps 1", `return "clean"`, `return "nap1"`, "", naps},
		{"naps 2", `return "clean"`, `return "nap2"`, "", reversed(naps)},
		{"broken", "a + b", `a + "x"`, "", refs("a/TestAdd")},
		{"unreached", "s += i", `s += "x"`, "", nil},
		{"survivor", "x < 0", "x <= 0", "", refs("a/TestAbs", "a/TestReadsTestdata")},
		{"naps 3", `return "clean"`, `return "nap3"`, "", naps},
		{"interrupt", "return 1", `Note("interrupt"); time.Sleep(time.Minute); return 1`, "", refs("a/TestSlow")},
		{"late", `return "clean"`, `return "late"`, "", naps},
	}
	for _, s := range specs {
		m, err := mutant(root, calcGo, s.orig, s.repl, s.tag)
		if err != nil {
			return nil, err
		}
		r.jobs[s.name] = runner.Job{Mutant: m, Tests: s.tests}
	}
	// The head mutant inserts at the start of a file, and its text is longer than the file. The tail
	// mutant ends at the file's last byte.
	words, err := os.ReadFile(filepath.Join(root, wordsGo))
	if err != nil {
		return nil, err
	}
	head := "// This comment is longer than the rest of the file, so the mutated file is more than twice as long as the original.\n"
	r.jobs["head"] = runner.Job{Mutant: splice(wordsGo, words, 0, 0, head, ""), Tests: refs("b/TestShout")}
	tail := strings.Index(string(words), "{ return strings.ToUpper(s) }")
	r.jobs["tail"] = runner.Job{
		Mutant: splice(wordsGo, words, tail, len(words), "{ return strings.Repeat(s, 0) }\n", ""),
		Tests:  refs("b/TestShout", "b/ExampleShout", "b/FuzzShout"),
	}
	var first []runner.Job
	for _, s := range specs {
		switch s.name {
		case "naps 3", "interrupt", "late":
		default:
			first = append(first, r.jobs[s.name])
		}
	}
	first = append(first, r.jobs["head"], r.jobs["tail"])
	second := []runner.Job{r.jobs["naps 3"]}
	interrupt, late := r.jobs["interrupt"], r.jobs["late"]

	if err := runInto(ctx, r.base, first, r.jobs, r.rows); err != nil {
		return nil, err
	}
	if err := runInto(ctx, r.base, second, r.jobs, r.again); err != nil {
		return nil, err
	}

	// The interrupted Run stops once the mutant's test has started.
	ictx, interrupted := context.WithCancel(ctx)
	go func() {
		defer interrupted()
		for ictx.Err() == nil {
			if log, _ := readLog(root); hasEvent(log, "interrupt") {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	_, r.interrupted = r.base.Run(ictx, []runner.Job{interrupt})
	interrupted()
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, r.canceled = r.base.Run(cctx, []runner.Job{late})

	if r.log, err = readLog(root); err != nil {
		return nil, err
	}
	if r.after, err = snapshot(root); err != nil {
		return nil, err
	}
	return r, ctx.Err()
})

func reversed(rs []model.TestRef) []model.TestRef {
	out := slices.Clone(rs)
	slices.Reverse(out)
	return out
}

// runInto runs jobs and files each row under its job's name.
func runInto(ctx context.Context, b *runner.Baseline, jobs []runner.Job, byName map[string]runner.Job, out map[string]model.Row) error {
	rows, err := b.Run(ctx, jobs)
	if err != nil {
		return err
	}
	for name, j := range byName {
		if row, ok := rows[j.Mutant.Hash]; ok {
			out[name] = row
		}
	}
	return nil
}

func hasEvent(log []entry, event string) bool {
	for _, e := range log {
		if e.event == event {
			return true
		}
	}
	return false
}

// guardLog stops a scenario whose fixture log grows past limit lines, the way it does when a broken
// runner restarts test processes without end.
func guardLog(ctx context.Context, root string, limit int, stop func()) {
	for ctx.Err() == nil {
		time.Sleep(100 * time.Millisecond)
		if log, _ := readLog(root); len(log) > limit {
			stop()
			return
		}
	}
}

// A proveRun is a scenario that proves one suite that fails against the clean code somewhere.
type proveRun struct {
	err  error // what Prove, or the first Run, returned
	err2 error // what a second Prove, or a second Run with the same batch, returned
	log  []entry

	// The batch scenario's Runs with test binaries that can't start.
	foreign error     // the mutant's binary is built for another operating system
	rerun   error     // the mutant's test takes away its binary's execute permission and times out
	spoiled model.Row // a batch's clean run took away the clean binary's execute permission
	unclean error     // a later batch needs the clean binary
}

// aloneScenario proves a suite in which TestDepends passes in the whole run and fails alone, with two
// jobs. TestAaSlow, the first test alone, is still running when TestDepends fails, and TestSetup is the
// last test alone.
var aloneScenario = newScenario(func(ctx context.Context) (*proveRun, error) {
	root, err := copyFixture("fx")
	if err != nil {
		return nil, err
	}
	o := runner.Options{Root: root, Jobs: 2, Work: filepath.Join(filepath.Dir(root), "work")}
	_, perr := runner.Prove(ctx, o, []runner.Suite{suite("alone", "TestAaSlow", "TestDepends", "TestSetup")}, nil)
	log, err := readLog(root)
	if err != nil {
		return nil, err
	}
	// The noload suite imports a package that doesn't exist, so go list can't load it.
	o.Work = filepath.Join(filepath.Dir(root), "work2")
	_, perr2 := runner.Prove(ctx, o, []runner.Suite{suite("noload", "TestNothing")}, nil)
	return &proveRun{err: perr, err2: perr2, log: log}, nil
})

// batchScenario proves a suite in which TestX fails when it runs in one process with TestY alone, and
// then runs two mutants whose batch is just those two, one Run each.
var batchScenario = newScenario(func(ctx context.Context) (*proveRun, error) {
	root, err := copyFixture("fx")
	if err != nil {
		return nil, err
	}
	o := runner.Options{Root: root, Jobs: 2, Work: filepath.Join(filepath.Dir(root), "work")}
	b, err := runner.Prove(ctx, o, []runner.Suite{suite("batchy", "TestX", "TestY", "TestZ", "TestSpoil", "TestSlowSpoil")}, nil)
	if err != nil {
		return nil, err
	}
	r := &proveRun{}
	for i, tag := range []string{"batchy1", "batchy2"} {
		m, err := mutant(root, calcGo, `return "clean"`, `return "`+tag+`"`, "")
		if err != nil {
			return nil, err
		}
		_, err = b.Run(ctx, []runner.Job{{Mutant: m, Tests: refs("batchy/TestX", "batchy/TestY")}})
		if i == 0 {
			r.err = err
		} else {
			r.err2 = err
		}
	}
	run := func(tag, orig, repl string, tests ...string) (model.Row, error) {
		m, err := mutant(root, calcGo, orig, repl, tag)
		if err != nil {
			return model.Row{}, err
		}
		rows, err := b.Run(ctx, []runner.Job{{Mutant: m, Tests: refs(tests...)}})
		return rows[m.Hash], err
	}
	tag := func(name string) (string, string) { return `return "clean"`, `return "` + name + `"` }

	// Built for another operating system, the mutant's binary can't start. GOOS is the process's
	// own environment, so it changes only between Runs.
	// The fixture uses syscall functions Windows lacks, so the other system is macOS or Linux.
	foreign := "linux"
	if runtime.GOOS == "linux" {
		foreign = "darwin"
	}
	goos, had := os.LookupEnv("GOOS")
	os.Setenv("GOOS", foreign)
	orig, repl := tag("foreign")
	_, r.foreign = run("", orig, repl, "batchy/TestZ")
	if had {
		os.Setenv("GOOS", goos)
	} else {
		os.Unsetenv("GOOS")
	}

	_, r.rerun = run("", "return 1", "os.Chmod(os.Args[0], 0o644); time.Sleep(time.Minute); return 1", "batchy/TestSlowSpoil")

	orig, repl = tag("spoil1")
	r.spoiled, err = run("", orig, repl, "batchy/TestSpoil")
	if err != nil {
		return nil, err
	}
	orig, repl = tag("spoil2")
	_, r.unclean = run("", orig, repl, "batchy/TestSpoil", "batchy/TestZ")

	r.log, err = readLog(root)
	return r, err
})
