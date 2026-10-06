package run_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/model"
)

// Contract: run/R1
func TestFailingAloneStopsTheRun(t *testing.T) {
	r := aloneScenario.get(t)
	msg := undecided(t, r.err)
	if !strings.Contains(msg, "alone/TestDepends") {
		t.Errorf("reason %q doesn't name alone/TestDepends", msg)
	}
	for _, name := range []string{"alone/TestSetup", "alone/TestAaSlow"} {
		if strings.Contains(msg, name) {
			t.Errorf("reason %q names %s, which passes", msg, name)
		}
	}
	firstLone := len(r.log)
	whole := map[string]bool{}
	for i, e := range r.log {
		if e.run == nil {
			whole[e.event] = true
			if i > firstLone {
				t.Errorf("%s ran in the whole suite after the lone runs began", e.event)
			}
		} else if e.lone() && i < firstLone {
			firstLone = i
		}
		if e.lone() && e.event == "TestSetup" {
			t.Error("TestSetup started alone after TestDepends failed alone")
		}
	}
	if len(whole) != 3 {
		t.Errorf("the whole suite ran %v, want all three tests", whole)
	}
	if firstLone == len(r.log) {
		t.Error("no test ran alone")
	}
}

// Contract: run/R1
func TestSuiteGoCantLoadStopsTheRun(t *testing.T) {
	r := aloneScenario.get(t)
	// The noload suite imports a package that doesn't exist, so none of its tests can run.
	if msg := undecided(t, r.err2); !strings.Contains(msg, "noload") {
		t.Errorf("reason %q doesn't name the noload suite", msg)
	}
}

// Contract: run/R1
func TestFailingInTheWholeSuiteExitsTwo(t *testing.T) {
	r := e2eScenario.get(t)
	if r.code != 2 {
		t.Errorf("exit %d, want 2\nstdout:\n%s\nstderr:\n%s", r.code, r.out, r.errOut)
	}
	if !strings.Contains(r.out, "calc/TestVictim") {
		t.Errorf("the report doesn't name calc/TestVictim:\n%s", r.out)
	}
	if strings.Contains(r.out, "calc/TestLeaky") {
		t.Errorf("the report names calc/TestLeaky, which passes:\n%s", r.out)
	}
}

// Contract: run/R1
func TestSuiteRunsUnderTheRunsBuildTags(t *testing.T) {
	r := mainScenario.get(t)
	// TestTagged is in a file only the fxtag build tag compiles, so it runs whole and alone only when
	// the run's tags reach every go command.
	var whole, lone bool
	for _, e := range r.log {
		if e.event == "TestTagged" {
			whole = whole || e.run == nil
			lone = lone || e.lone()
		}
	}
	if !whole || !lone {
		t.Errorf("TestTagged ran whole %v and alone %v, want both", whole, lone)
	}
}

// Contract: run/R2
func TestLoneRunsMapEachMutantToTheTestsThatReachIt(t *testing.T) {
	r := mainScenario.get(t)
	add := refs("a/TestAdd", "a/TestAddMore", "a/TestParFast", "b/TestOtherAdd",
		"a/TestChainA1", "a/TestChainA2", "a/TestChainA3", "a/TestChainA4", "a/TestChainA5")
	for _, c := range []struct {
		name, file, text string
		want             []model.TestRef
	}{
		{"inside a one-line function", calcGo, "a + b", add},
		{"on the last line of a block", calcGo, "x < 0", refs("a/TestAbs")},
		{"on a later line of a block, left of where it starts", calcGo, "return -x", refs("a/TestAbs")},
		{"at the first byte of a block", calcGo, "return x\n", refs("a/TestAbs")},
		{"in a loop body", calcGo, "s += i", refs("a/TestSum", "a/TestParSum")},
		{"in a case body no test runs", calcGo, "return 3", nil},
		{"in a function only crashing tests reach", calcGo, "a / b", refs("a/TestChainZ1", "a/TestChainZ2", "a/TestChainZ3", "a/TestChainZ4")},
		{"in another package", wordsGo, "strings.ToUpper", refs("b/TestShout", "b/ExampleShout", "b/FuzzShout")},
		{"in a function no test calls", calcGo, "os.WriteFile", nil},
		{"outside every function", calcGo, "package calc", nil},
	} {
		line, col := at(t, r.root, c.file, c.text)
		if got := r.base.Reach(c.file, line, col); !sameRefs(got, c.want) {
			t.Errorf("%s: Reach(%s:%d:%d) = %v, want %v", c.name, c.file, line, col, names(got), names(c.want))
		}
	}
	for _, c := range []struct {
		name, from, to string
		want           []model.TestRef
	}{
		{"one function's body", "func Abs(x int) int {", "func Div", refs("a/TestAbs")},
		{"a body with a loop", "func Sum(n int) int {", "func Slow", refs("a/TestSum", "a/TestParSum")},
		{"two functions", "func Div(a, b int) int {", "func Slow",
			refs("a/TestChainZ1", "a/TestChainZ2", "a/TestChainZ3", "a/TestChainZ4", "a/TestSum", "a/TestParSum")},
		{"a body no test runs", "func pause(marker string, d time.Duration) {", "", nil},
		{"comments between functions", "// Div is called", "func Div", nil},
	} {
		fl, fc, tl, tc := span(t, r.root, c.from, c.to)
		if got := r.base.ReachSpan(calcGo, fl, fc, tl, tc); !sameRefs(got, c.want) {
			t.Errorf("%s: ReachSpan(%d:%d, %d:%d) = %v, want %v", c.name, fl, fc, tl, tc, names(got), names(c.want))
		}
	}
}

// span runs from the last byte of from to just past the closing brace before to, or before the end of
// the file when to is empty. For a function's opening line, that's the span an erase mutant replaces.
func span(t *testing.T, root, from, to string) (fl, fc, tl, tc int) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, calcGo))
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	i := strings.Index(s, from)
	if i < 0 {
		t.Fatalf("no %q", from)
	}
	start := i + len(from) - 1
	end := len(s)
	if to != "" {
		end = start + strings.Index(s[start:], to)
	}
	end = strings.LastIndex(s[:end], "}") + 1
	fl, fc = position(src, start)
	tl, tc = position(src, end)
	return fl, fc, tl, tc
}

// Contract: run/R3
func TestInitializersAreReachedByEveryTestLinkingTheirPackage(t *testing.T) {
	r := mainScenario.get(t)
	a, b := suiteRefs("a", aTests), suiteRefs("b", bTests)
	all := append(append([]model.TestRef(nil), a...), b...)
	if got := r.base.Linking(fx + "/calc"); !sameRefs(got, all) {
		t.Errorf("Linking(calc) = %v, want every test", names(got))
	}
	if got := r.base.Linking(fx + "/words"); !sameRefs(got, b) {
		t.Errorf("Linking(words) = %v, want suite b's tests", names(got))
	}
	if got := r.base.Linking(fx + "/nothing"); len(got) != 0 {
		t.Errorf("Linking(nothing) = %v, want none", names(got))
	}
	row := r.rows["initializer"]
	if !row.Complete || !sameRefs(row.Ran, all) || !slices.Equal(kills(row), []string{"a/TestTable assertion"}) {
		t.Errorf("initializer row = %+v, want every test run and a/TestTable the only killer", row)
	}
}

func suiteRefs(prim string, tests []string) []model.TestRef {
	var out []model.TestRef
	for _, n := range tests {
		out = append(out, model.TestRef{Primitive: prim, Name: n})
	}
	return out
}

// Contract: run/R4
func TestUnreachedMutantIsNeverBuilt(t *testing.T) {
	r := mainScenario.get(t)
	// The mutant doesn't compile, so building it would have left the row without a verdict.
	if got := r.rows["unreached"]; !reflect.DeepEqual(got, model.Row{Complete: true}) {
		t.Errorf("row = %+v, want a complete row with nothing run", got)
	}
	line, col := at(t, r.root, calcGo, "os.WriteFile")
	if got := r.base.Reach(calcGo, line, col); len(got) != 0 {
		t.Errorf("Reach of a line no test runs = %v", names(got))
	}
}

// Contract: run/R5
func TestEachBatchRunsCleanOnceBeforeAnyMutantUsesIt(t *testing.T) {
	r := mainScenario.get(t)
	batch := names(naps)
	for i := range batch {
		batch[i] = strings.TrimPrefix(batch[i], "a/")
	}
	for _, test := range batch {
		clean, mutated := -1, 0
		for i, e := range r.log {
			if e.event != test || !slices.Equal(e.run, batch) {
				continue
			}
			switch {
			case e.tag != "clean":
				mutated++
				if clean < 0 {
					t.Errorf("%s ran against mutant %s before its batch ran clean", test, e.tag)
				}
			case clean >= 0:
				t.Errorf("%s ran clean in its batch more than once", test)
			default:
				clean = i
			}
		}
		if clean < 0 {
			t.Errorf("%s never ran clean in its batch", test)
		}
		// Three mutants share the batch, the third in a later Run.
		if mutated != 3 {
			t.Errorf("%s ran against %d mutants in its batch, want 3", test, mutated)
		}
	}
	if row := r.again["naps 3"]; !row.Complete || !sameRefs(row.Ran, naps) || len(row.Kills) != 0 {
		t.Errorf("the later Run's row = %+v", row)
	}
}

// Contract: run/R5
func TestBatchFailingCleanStopsTheRun(t *testing.T) {
	r := batchScenario.get(t)
	for i, err := range []error{r.err, r.err2} {
		if msg := undecided(t, err); !strings.Contains(msg, "batchy/TestX") {
			t.Errorf("Run %d: reason %q doesn't name batchy/TestX", i+1, msg)
		}
	}
	ranClean := false
	for _, e := range r.log {
		if strings.HasPrefix(e.tag, "batchy") {
			t.Errorf("%s ran against mutant %s", e.event, e.tag)
		}
		if e.event == "TestX" && slices.Equal(e.run, []string{"TestX", "TestY"}) {
			ranClean = true
		}
	}
	if !ranClean {
		t.Error("the batch never ran clean")
	}
}

// Contract: run/R5
func TestBatchWhoseCleanBinaryCantStartStopsTheRun(t *testing.T) {
	r := batchScenario.get(t)
	// TestSpoil's clean run in its batch takes away the clean binary's permission to execute, so the
	// next batch can't run clean.
	if !r.spoiled.Complete || !sameRefs(r.spoiled.Ran, refs("batchy/TestSpoil", "batchy/TestZ")) || len(r.spoiled.Kills) != 0 {
		t.Errorf("first batch's row = %+v, want both tests run and passed", r.spoiled)
	}
	undecided(t, r.unclean)
}

// Contract: run/R6
func TestRunLeavesTheModuleAsItFoundIt(t *testing.T) {
	r := mainScenario.get(t)
	// The mutants were compiled: their changes took effect.
	for name, want := range map[string][]string{
		"assertions": {"a/TestAdd assertion", "a/TestAddMore assertion", "b/TestOtherAdd assertion"},
		"tail":       {"b/ExampleShout assertion", "b/FuzzShout assertion FuzzShout/seed#0", "b/TestShout assertion"},
		"head":       {},
	} {
		row := r.rows[name]
		if !row.Complete || !slices.Equal(kills(row), want) {
			t.Errorf("%s: row = %+v, want kills %v", name, row, want)
		}
	}
	if !hasEvent(r.log, "interrupt") {
		t.Error("the interrupted Run never started its mutant")
	}
	if r.interrupted == nil {
		t.Error("the interrupted Run returned no error")
	}
	if r.canceled == nil {
		t.Error("a Run whose context had ended returned no error")
	}
	for path, data := range r.before {
		if after, ok := r.after[path]; !ok {
			t.Errorf("%s is gone", path)
		} else if after != data {
			t.Errorf("%s changed", path)
		}
	}
	for path := range r.after {
		if _, ok := r.before[path]; !ok {
			t.Errorf("%s appeared", path)
		}
	}
}

// Contract: run/R6
func TestFileThatNoLongerFitsItsMutantStopsTheRun(t *testing.T) {
	r := batchScenario.get(t)
	undecided(t, r.shrunk)
}

// Contract: run/R7
func TestProcessLeftRunningDoesntHoldUpTheRun(t *testing.T) {
	r := mainScenario.get(t)
	// TestLinger leaves a process running that holds the output pipe open for 100 seconds.
	if row := r.again["linger"]; !row.Complete || len(row.Ran) != 1 || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want TestLinger run and passed", row)
	}
	if r.second > time.Minute {
		t.Errorf("the Run with TestLinger took %v, as long as the process it left behind", r.second)
	}
}

// Contract: run/R7
func TestTestBinariesRunInTheirPackageDirectory(t *testing.T) {
	r := mainScenario.get(t)
	// TestReadsTestdata reads testdata/input.txt by a relative path, so it passes only in its own
	// directory.
	row := r.rows["survivor"]
	if !row.Complete || !sameRefs(row.Ran, refs("a/TestAbs", "a/TestReadsTestdata")) || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want both tests run and passed", row)
	}
	// Every fixture test writes fx.log through a path relative to its package directory.
	var whole, lone, budgeted bool
	for _, e := range r.log {
		if e.event == "TestReadsTestdata" {
			whole = whole || e.run == nil
			lone = lone || e.lone()
			budgeted = budgeted || e.budgeted()
		}
	}
	if !whole || !lone || !budgeted {
		t.Errorf("TestReadsTestdata's log: whole run %v, lone run %v, budgeted runs %v; want all", whole, lone, budgeted)
	}
}

// Contract: run/R8
func TestRowRecordsEveryFailureAndHowItFailed(t *testing.T) {
	r := mainScenario.get(t)
	for name, want := range map[string][]string{
		"assertions":     {"a/TestAdd assertion", "a/TestAddMore assertion", "b/TestOtherAdd assertion"},
		"subtests":       {"a/TestAbs assertion TestAbs/big,TestAbs/neg"},
		"mixed kinds":    {"a/TestAdd assertion", "a/TestAddMore assertion", "a/TestChainA1 panic"},
		"company":        {"a/TestBumpB assertion"},
		"exit zero":      {"a/TestReady panic"},
		"panics":         {"a/TestChainZ1 panic", "a/TestChainZ2 panic", "a/TestChainZ3 panic", "a/TestChainZ4 panic"},
		"serial timeout": {"a/TestSum timeout"},
		"tail":           {"b/ExampleShout assertion", "b/FuzzShout assertion FuzzShout/seed#0", "b/TestShout assertion"},
	} {
		row, job := r.rows[name], r.jobs[name]
		if !row.Complete || row.Verdict != "" || !sameRefs(row.Ran, job.Tests) {
			t.Errorf("%s: row = %+v, want every test run", name, row)
		}
		if got := kills(row); !slices.Equal(got, want) {
			t.Errorf("%s: kills = %v, want %v", name, got, want)
		}
	}
}

// Contract: run/R8
func TestExitZeroInATestIsAPanic(t *testing.T) {
	r := mainScenario.get(t)
	// os.Exit(0) under the test makes go test's own panic, which names TestReady, so the two tests
	// after it start again together. Had the process just ended, nothing would name a test and they
	// would run one per process.
	if got := kills(r.rows["exit zero"]); !slices.Equal(got, []string{"a/TestReady panic"}) {
		t.Errorf("kills = %v, want a/TestReady panic", got)
	}
	restarted(t, r.log, []string{"TestSignBig", "TestSignNeg"}, []string{"TestSignBig", "TestSignNeg"})
}

// Contract: run/R9
func TestCrashRestartsTheRestInOneFreshProcess(t *testing.T) {
	r := mainScenario.get(t)
	row := r.rows["panics"]
	if !row.Complete || !sameRefs(row.Ran, r.jobs["panics"].Tests) {
		t.Errorf("row = %+v, want every test run", row)
	}
	// Each TestChainZ test panics against the mutant, and the tests after it start again in one
	// process of their own.
	order := []string{"TestChainA1", "TestChainZ1", "TestChainA2", "TestChainZ2", "TestChainA3", "TestChainZ3", "TestChainA4", "TestChainZ4", "TestChainA5"}
	for crash := 1; crash < len(order); crash += 2 {
		restQ := slices.Clone(order[crash+1:])
		slices.Sort(restQ)
		restarted(t, r.log, restQ, order[crash+1:min(crash+3, len(order))])
	}
	restarted(t, r.log, []string{"TestAfterSum1", "TestAfterSum2"}, []string{"TestAfterSum1", "TestAfterSum2"})
	if got := kills(r.rows["serial timeout"]); !slices.Equal(got, []string{"a/TestSum timeout"}) {
		t.Errorf("serial timeout: kills = %v", got)
	}
}

// restarted checks that the tests in ran each wrote a budgeted log entry from one process that was
// asked to run exactly rest.
func restarted(t *testing.T, log []entry, rest, ran []string) {
	t.Helper()
	pids := map[int]bool{}
	for _, test := range ran {
		found := false
		for _, e := range log {
			if e.event == test && e.budgeted() && slices.Equal(e.run, rest) {
				found = true
				pids[e.pid] = true
			}
		}
		if !found {
			t.Errorf("%s never ran in a process asked to run %v", test, rest)
		}
	}
	if len(pids) > 1 {
		t.Errorf("%v ran in %d processes, want one", ran, len(pids))
	}
}

// Contract: run/R9
func TestTimeoutIsPinnedOnTheTestsStillRunning(t *testing.T) {
	r := mainScenario.get(t)
	// TestParFast passes and reports after TestParSum starts, so the stream's last test is TestParFast
	// when the budget runs out.
	row := r.rows["parallel timeout"]
	if !row.Complete || !sameRefs(row.Ran, refs("a/TestParFast", "a/TestParSum")) {
		t.Errorf("row = %+v, want both tests run", row)
	}
	if got := kills(row); !slices.Equal(got, []string{"a/TestParSum timeout"}) {
		t.Errorf("kills = %v, want a/TestParSum timeout", got)
	}
}

// Contract: run/R10
func TestDeathBeforeTheFirstTestKillsTheWholeBatch(t *testing.T) {
	r := mainScenario.get(t)
	row, job := r.rows["init panics"], r.jobs["init panics"]
	var want []string
	for _, ref := range job.Tests {
		want = append(want, ref.String()+" panic")
	}
	slices.Sort(want)
	if !row.Complete || !sameRefs(row.Ran, job.Tests) || !slices.Equal(kills(row), want) {
		t.Errorf("row = %+v, want every test killed by a panic", row)
	}
	// The mutant's init function writes a line before it panics, once in each suite's process.
	var runs [][]string
	for _, e := range r.log {
		if e.event == "init" {
			runs = append(runs, e.run)
		}
	}
	if len(runs) != 2 || len(runs[0]) < 2 || len(runs[1]) < 2 {
		t.Errorf("processes that died in init were asked to run %v, want one process for each suite's batch", runs)
	}
}

// Contract: run/R10
func TestCrashNamingNoTestRerunsOneTestPerProcess(t *testing.T) {
	r := mainScenario.get(t)
	row := r.rows["exit"]
	if !row.Complete || !sameRefs(row.Ran, r.jobs["exit"].Tests) || !slices.Equal(kills(row), []string{"a/TestReadsTestdata panic"}) {
		t.Errorf("row = %+v, want a/TestReadsTestdata killed by its crash and the rest run", row)
	}
	for _, test := range []string{"TestReadsTestdata", "TestQuiet1", "TestQuiet2"} {
		found := false
		for _, e := range r.log {
			found = found || e.event == test && e.budgeted() && slices.Equal(e.run, []string{test})
		}
		if !found {
			t.Errorf("%s never ran in a process of its own after the crash", test)
		}
	}
}

// Contract: run/R10
func TestHangWithNoTestToBlameRerunsOneTestPerProcess(t *testing.T) {
	r := mainScenario.get(t)
	// Against this mutant TestHold1 stops its process the first time it runs, so Go's timeout never
	// fires and the timeout panic never names a test.
	row := r.rows["guard"]
	if !row.Complete || !sameRefs(row.Ran, refs("a/TestHold1", "a/TestHold2")) || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want both tests run and passed", row)
	}
	for _, test := range []string{"TestHold1", "TestHold2"} {
		found := false
		for _, e := range r.log {
			found = found || e.event == test && e.budgeted() && slices.Equal(e.run, []string{test})
		}
		if !found {
			t.Errorf("%s never ran in a process of its own after the hang", test)
		}
	}
}

// Contract: run/R10
func TestNoVerdictAfterThreeRestarts(t *testing.T) {
	r := mainScenario.get(t)
	// Against this mutant, every process prints the line that ends a test run and exits before any
	// test starts, so no restart settles anything.
	row := r.rows["no test starts"]
	if row.Verdict == "" || row.Complete || len(row.Ran) != 0 || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want no verdict", row)
	}
	var runs [][]string
	for _, e := range r.log {
		if e.event == "stall" {
			runs = append(runs, e.run)
		}
	}
	add, ready := []string{"TestAdd"}, []string{"TestReady"}
	if len(runs) != 7 || !slices.Equal(runs[0], []string{"TestAdd", "TestReady"}) ||
		count(runs[1:], add) != 3 || count(runs[1:], ready) != 3 {
		t.Errorf("processes asked to run %v, want the batch once and then each test alone three times", runs)
	}
	// Against this mutant TestSlowToo times out, and the rerun that would confirm the timeout settles
	// nothing in three restarts either.
	if row := r.rows["rerun stalls"]; row.Verdict == "" || row.Complete {
		t.Errorf("rerun stalls: row = %+v, want no verdict", row)
	}
}

func count(runs [][]string, run []string) int {
	n := 0
	for _, r := range runs {
		if slices.Equal(r, run) {
			n++
		}
	}
	return n
}

// Contract: run/R11
func TestBudgetIsLoneTimesTimesTenPlusTwoSeconds(t *testing.T) {
	r := mainScenario.get(t)
	// TestNap1 and TestNap2 each sleep 200ms, so their batch's lone run times add up to at least
	// 400ms, and the budget is at least 10 times that plus 2 seconds. The upper bound leaves room for
	// a busy machine; a budget that drops the coefficient, the 2 seconds or a lone run time comes out
	// at 4 seconds or less.
	batch := names(naps)
	for i := range batch {
		batch[i] = strings.TrimPrefix(batch[i], "a/")
	}
	// The batch's clean run gets ten times the budget (R5); every mutant's run gets the budget itself.
	var budgets []time.Duration
	for _, e := range r.log {
		if !slices.Contains(batch, e.event) || !e.budgeted() || !slices.Equal(e.run, batch) {
			continue
		}
		d, err := time.ParseDuration(e.timeout)
		if err != nil {
			t.Fatal(err)
		}
		budgets = append(budgets, d)
	}
	if len(budgets) == 0 {
		t.Fatal("the batch never ran with a budget")
	}
	budget := slices.Min(budgets)
	if budget < 6*time.Second || budget > 20*time.Second {
		t.Errorf("the batch ran with budget %s, want 10 x (its batch's lone run times) + 2s", budget)
	}
	for _, d := range budgets {
		if d != budget && (d-10*budget).Abs() > 20*time.Millisecond {
			t.Errorf("the batch ran with budget %s, want %s, or ten times it for its clean run", d, budget)
		}
	}
}

// Contract: run/R11
func TestTimeoutCountsOnlyWhenARerunTimesOutToo(t *testing.T) {
	r := mainScenario.get(t)
	// Against the mutant, TestSlow sleeps 6s the first time and returns at once after that.
	row := r.rows["timeout once"]
	if !row.Complete || !sameRefs(row.Ran, refs("a/TestSlow")) || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want TestSlow run and not a killer", row)
	}
	var budgets []time.Duration
	for _, e := range r.log {
		if e.event == "TestSlow" && e.budgeted() && slices.Equal(e.run, []string{"TestSlow"}) && len(budgets) < 2 {
			d, err := time.ParseDuration(e.timeout)
			if err != nil {
				t.Fatal(err)
			}
			budgets = append(budgets, d)
		}
	}
	// The batch's clean run, at ten times a mutant's budget, then the mutant's run and its rerun.
	if len(budgets) != 3 || (budgets[0]-10*budgets[1]).Abs() > 20*time.Millisecond || (budgets[2]-2*budgets[1]).Abs() > 2*time.Millisecond {
		t.Errorf("TestSlow's budgets = %v, want the clean run's ten times the mutant's and the rerun's twice it", budgets)
	}
	// TestSum loops forever against its mutant, and its rerun, alone, times out too.
	if got := kills(r.rows["serial timeout"]); !slices.Equal(got, []string{"a/TestSum timeout"}) {
		t.Errorf("serial timeout: kills = %v", got)
	}
	rerun := false
	for _, e := range r.log {
		if e.event == "TestSum" && e.budgeted() && slices.Equal(e.run, []string{"TestSum"}) {
			d, _ := time.ParseDuration(e.timeout)
			rerun = rerun || d >= 4*time.Second
		}
	}
	if !rerun {
		t.Error("TestSum never reran alone with twice a budget of at least 2s")
	}
}

// Contract: run/R12
func TestMutantWhoseTestsCantStartStopsTheRun(t *testing.T) {
	r := batchScenario.get(t)
	// Built for another operating system, the mutant's binary never starts.
	undecided(t, r.foreign)
	// The mutant's test takes away its binary's permission to execute and then hangs, so the rerun
	// that would confirm the timeout can't start.
	undecided(t, r.rerun)
}

// Contract: run/R12
func TestMutantThatWontBuildHasNoVerdict(t *testing.T) {
	r := mainScenario.get(t)
	row := r.rows["broken"]
	if row.Verdict == "" || row.Complete || len(row.Ran) != 0 || len(row.Kills) != 0 {
		t.Errorf("row = %+v, want no verdict and nothing run", row)
	}
	// The compiler's first complaint, one line, naming calc.go by its path in the module.
	t.Logf("reason: %s", row.Verdict)
	if !strings.Contains(row.Verdict, calcGo+":") || strings.ContainsAny(row.Verdict, "\n#") || strings.Contains(row.Verdict, r.work) {
		t.Errorf("reason %q isn't the compiler's first complaint about %s", row.Verdict, calcGo)
	}
}

// Contract: run/R13
func TestProcessOverTheMemoryLimitCrashes(t *testing.T) {
	r := mainScenario.get(t)
	// TestGrow holds ever more memory against the mutant. The guard stops its process, which names
	// no test, so the batch runs one test per process and the lone TestGrow is the killer.
	row := r.rows["memory"]
	if !row.Complete || !sameRefs(row.Ran, r.jobs["memory"].Tests) || !slices.Equal(kills(row), []string{"a/TestGrow panic"}) {
		t.Errorf("row = %+v, want a/TestGrow killed by a crash and the rest run", row)
	}
	for _, test := range []string{"TestGrow", "TestGrowAfter1", "TestGrowAfter2"} {
		found := false
		for _, e := range r.log {
			found = found || e.event == test && e.budgeted() && slices.Equal(e.run, []string{test})
		}
		if !found {
			t.Errorf("%s never ran in a process of its own", test)
		}
	}
}

// Contract: run/R14
func TestTestProcessesKeepTempFilesInTheWorkDirectory(t *testing.T) {
	r := mainScenario.get(t)
	tmp := filepath.Join(r.work, "tmp")
	for _, e := range r.log {
		if e.tmpdir != tmp || e.tmp != tmp || e.temp != tmp {
			t.Fatalf("%s ran with TMPDIR %q, TMP %q and TEMP %q, want %s", e.event, e.tmpdir, e.tmp, e.temp, tmp)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "fx-leftover-*")); len(left) == 0 {
		t.Errorf("TestLeftover's file isn't in %s", tmp)
	}
}

// Contract: run/R14
func TestRunLeavesNothingInTheTempDirectory(t *testing.T) {
	r := e2eScenario.get(t)
	// TestLeaky left a file in its temp directory.
	if len(r.left) != 0 {
		t.Errorf("the run left %v in its temp directory", r.left)
	}
}

// Contract: run/R14
func TestMutantBuildsUseTheRunsOwnCache(t *testing.T) {
	r := cacheScenario.get(t)
	own := filepath.Join(r.own, "gocache")
	if entries, err := os.ReadDir(own); err != nil || len(entries) == 0 {
		t.Errorf("the mutant build left nothing in %s: %v", own, err)
	}
	if _, err := os.Stat(filepath.Join(r.handed, "gocache")); err == nil {
		t.Error("the run handed a cache built one of its own anyway")
	}
	for name, log := range map[string][]entry{"first": r.ownLog, "second": r.handedLog} {
		if len(log) == 0 {
			t.Errorf("the %s run's tests wrote nothing", name)
		}
		for _, e := range log {
			if e.cache != own {
				t.Errorf("the %s run handed %s the cache %q, want %s", name, e.event, e.cache, own)
			}
		}
	}
}
