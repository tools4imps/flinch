package gate_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
	"github.com/tools4imps/flinch/internal/gate"
	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
)

// Contract: gate/G1
func TestTheVerdictIgnoresTheJobCountAndGitConfig(t *testing.T) {
	dir := copyFixture(t, "testdata/mod")
	newRepo(t, dir)
	replace(t, filepath.Join(dir, "calc", "calc.go"), "\tif a > b {\n", "\tif a > b { // the larger one\n")
	t.Chdir(dir)

	cleanGit(t)
	code1, out1, errs1 := flinch("--since", "HEAD", "--jobs", "1", "--format", "json")
	hostileGit(t, dir)
	code2, out2, errs2 := flinch("--since", "HEAD", "--jobs", "4", "--format", "json")

	if code1 != gate.Fail {
		t.Fatalf("the first run exited %d, want 1 for calc.Max's living boundary mutant\n%s%s", code1, out1, errs1)
	}
	if code2 != code1 {
		t.Errorf("four jobs and a hostile git config exit %d, and one job with a clean config exits %d\n%s", code2, code1, errs2)
	}
	a, b := withoutTiming(t, out1), withoutTiming(t, out2)
	if a != b {
		t.Errorf("the reports differ.\none job, clean config:\n%s\nfour jobs, hostile config:\n%s", a, b)
	}
	var rep struct {
		Mutants []struct{ ID, Status string }
	}
	if err := json.Unmarshal([]byte(out1), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range rep.Mutants {
		got[m.ID] = m.Status
	}
	want := map[string]string{
		"calc.Max: { ... } -> { return *new(int) }": "killed",
		"calc.Max: a > b -> a >= b":                 "lived",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("--since HEAD gave mutants %v, want only calc.Max's, %v", got, want)
	}
}

// A scenario is one set of findings and the exit code they must give.
type scenario struct {
	name      string
	problems  []problem.Problem
	in        *matrix.Input // nil when the run stopped before mutating
	undecided string
	exit      int
	broken    int // how many declarations the matrix must call broken or stale
}

var (
	tOne   = ref("calc", "TestOne")
	tTwo   = ref("calc", "TestTwo")
	held   = mut("calc", "Add", "a + b", 3)
	other  = mut("calc", "Add", "a - b", 4)
	elsewh = mut("calc", "Sub", "a - b", 9)
	ghost  = mut("calc", "Add", "a * b", 5) // never generated: a declaration of it matches nothing
	inner  = mut("calc", "Add.func1", "x + 1", 6)
	ctrErr = problem.Problem{Path: "contract/calc/README.md", Line: 5, Message: "no Contract test names calc/C9"}
)

// sound is a full run in which one test holds the only mutant, so nothing fails.
func sound() *matrix.Input {
	return &matrix.Input{
		Obligations: []string{"calc/C1"},
		Tests:       []model.Test{ctest("calc", "TestOne", 10, "calc/C1")},
		Mutants:     []model.Mutant{held},
		Rows:        map[string]model.Row{held.Hash: killedBy(tOne)},
		Reached:     map[string][]model.TestRef{held.Hash: {tOne}},
		Full:        true,
	}
}

// with returns sound input changed by f.
func with(f func(in *matrix.Input)) *matrix.Input {
	in := sound()
	f(in)
	return in
}

// scoped makes a run scoped: it mutated the given primitives and "dir.Top" units whole.
func scoped(in *matrix.Input, prims []string, units ...string) {
	in.Full = false
	in.Whole = map[string]bool{}
	for _, p := range prims {
		in.Whole[p] = true
	}
	in.WholeUnits = map[string]bool{}
	for _, u := range units {
		in.WholeUnits[u] = true
	}
}

func addMutant(in *matrix.Input, m model.Mutant, row *model.Row, reached ...model.TestRef) {
	in.Mutants = append(in.Mutants, m)
	if row != nil {
		in.Rows[m.Hash] = *row
	}
	if len(reached) > 0 {
		in.Reached[m.Hash] = reached
	}
}

func rowPtr(r model.Row) *model.Row { return &r }

// Contract: gate/G2
func TestExitCodesSayPassFailOrUndecided(t *testing.T) {
	lived := rowPtr(passedBy(tOne))
	scenarios := []scenario{
		{name: "every mutant held", in: sound(), exit: gate.Pass},
		{name: "a Contract error alone", problems: []problem.Problem{ctrErr}, exit: gate.Fail},
		{name: "a lived mutant", in: with(func(in *matrix.Input) { addMutant(in, other, lived, tOne) }), exit: gate.Fail},
		{name: "an unreached mutant", in: with(func(in *matrix.Input) { addMutant(in, other, nil) }), exit: gate.Fail},
		{name: "an erased function", exit: gate.Fail, in: with(func(in *matrix.Input) {
			addMutant(in, eraseOf("calc", "Sub", 8), lived, tOne)
			addMutant(in, elsewh, nil, tOne)                    // reached, yet skipped
			addMutant(in, mut("calc", "Sub", "a * b", 10), nil) // unreached, yet skipped
		})},
		{name: "an erased function, declared", exit: gate.Pass, in: with(func(in *matrix.Input) {
			e := eraseOf("calc", "Sub", 8)
			addMutant(in, e, lived, tOne)
			addMutant(in, elsewh, nil, tOne)
			addMutant(in, mut("calc", "Sub", "a * b", 10), nil)
			in.Declarations = []matrix.Declaration{exact("unpromised", e)}
		})},
		{name: "a lived mutant declared equivalent", exit: gate.Pass, in: with(func(in *matrix.Input) {
			addMutant(in, other, lived, tOne)
			in.Declarations = []matrix.Declaration{exact("equivalent", other)}
		})},
		{name: "an unreached mutant under a wildcard", exit: gate.Pass, in: with(func(in *matrix.Input) {
			addMutant(in, elsewh, nil)
			in.Declarations = []matrix.Declaration{wildcard("calc", "Sub")}
		})},
		{name: "a wildcard leaves killed mutants killed", exit: gate.Pass, in: with(func(in *matrix.Input) {
			addMutant(in, other, lived, tOne)
			addMutant(in, inner, lived, tOne)
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a declared mutant a test kills", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			in.Declarations = []matrix.Declaration{exact("equivalent", held)}
		})},
		{name: "a stale declaration in a full run", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			in.Declarations = []matrix.Declaration{exact("unpromised", ghost)}
		})},
		{name: "a stale declaration of a unit a scoped run left alone", exit: gate.Pass, in: with(func(in *matrix.Input) {
			scoped(in, nil, "calc.Sub")
			in.Declarations = []matrix.Declaration{exact("unpromised", ghost)}
		})},
		{name: "a stale declaration of a unit a scoped run mutated whole", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			scoped(in, nil, "calc.Add")
			in.Declarations = []matrix.Declaration{exact("unpromised", ghost)}
		})},
		{name: "a stale declaration of a closure whose function was mutated whole", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			scoped(in, nil, "calc.Add")
			in.Declarations = []matrix.Declaration{exact("unpromised", inner)}
		})},
		{name: "a stale declaration in a primitive a scoped run mutated whole", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			scoped(in, []string{"calc"})
			in.Declarations = []matrix.Declaration{exact("unpromised", ghost)}
		})},
		{name: "a wildcard on a unit with nothing unheld", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a wildcard on a unit with a mutant without a verdict", exit: gate.Undecided, in: with(func(in *matrix.Input) {
			addMutant(in, other, rowPtr(model.Row{Verdict: "it wouldn't build"}), tOne)
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a stale wildcard in a scoped run that left its unit alone", exit: gate.Pass, in: with(func(in *matrix.Input) {
			scoped(in, nil, "calc.Sub")
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a stale wildcard in a scoped run that mutated its unit whole", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			scoped(in, nil, "calc.Add")
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a stale wildcard in a primitive a scoped run mutated whole", exit: gate.Fail, broken: 1, in: with(func(in *matrix.Input) {
			scoped(in, []string{"calc"})
			in.Declarations = []matrix.Declaration{wildcard("calc", "Add")}
		})},
		{name: "a hollow obligation in a full run", exit: gate.Fail, in: with(func(in *matrix.Input) {
			in.Obligations = append(in.Obligations, "calc/C2")
		})},
		{name: "a hollow obligation of a primitive a scoped run didn't mutate whole", exit: gate.Pass, in: with(func(in *matrix.Input) {
			scoped(in, nil)
			in.Obligations = append(in.Obligations, "calc/C2")
		})},
		{name: "a hollow obligation of a primitive a scoped run mutated whole", exit: gate.Fail, in: with(func(in *matrix.Input) {
			scoped(in, []string{"calc"})
			in.Obligations = append(in.Obligations, "calc/C2")
		})},
		{name: "a blind test in a full run", exit: gate.Fail, in: with(func(in *matrix.Input) {
			in.Tests = append(in.Tests, ctest("calc", "TestTwo", 20, "calc/C1"))
			in.Rows[held.Hash] = model.Row{Ran: []model.TestRef{tOne, tTwo}, Complete: true, Kills: []model.Kill{{Test: tOne, Kind: model.Assertion}}}
		})},
		{name: "a blind test of a primitive a scoped run didn't mutate whole", exit: gate.Pass, in: with(func(in *matrix.Input) {
			scoped(in, nil)
			in.Tests = append(in.Tests, ctest("calc", "TestTwo", 20, "calc/C1"))
			in.Rows[held.Hash] = model.Row{Ran: []model.TestRef{tOne, tTwo}, Complete: true, Kills: []model.Kill{{Test: tOne, Kind: model.Assertion}}}
		})},
		{name: "a blind test of a primitive a scoped run mutated whole", exit: gate.Fail, in: with(func(in *matrix.Input) {
			scoped(in, []string{"calc"})
			in.Tests = append(in.Tests, ctest("calc", "TestTwo", 20, "calc/C1"))
			in.Rows[held.Hash] = model.Row{Ran: []model.TestRef{tOne, tTwo}, Complete: true, Kills: []model.Kill{{Test: tOne, Kind: model.Assertion}}}
		})},
		{name: "a mutant without a verdict", exit: gate.Undecided, in: with(func(in *matrix.Input) {
			addMutant(in, other, rowPtr(model.Row{Verdict: "it wouldn't build"}), tOne)
		})},
		{name: "a mutant without a verdict beats a lived one", exit: gate.Undecided, in: with(func(in *matrix.Input) {
			addMutant(in, other, rowPtr(model.Row{Verdict: "it wouldn't build"}), tOne)
			addMutant(in, elsewh, lived, tOne)
		})},
		{name: "a reached mutant that never ran", exit: gate.Undecided, in: with(func(in *matrix.Input) {
			addMutant(in, other, nil, tOne)
		})},
		{name: "flinch couldn't decide", undecided: "calc/TestOne fails on clean code", exit: gate.Undecided},
		{name: "flinch couldn't decide, with Contract errors", problems: []problem.Problem{ctrErr},
			undecided: "calc/TestOne fails on clean code", exit: gate.Undecided},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			var r *matrix.Result
			full := true
			if s.in != nil {
				r = matrix.Compute(*s.in)
				full = s.in.Full
				if len(r.Broken) != s.broken {
					t.Errorf("%d broken declarations, want %d: %v", len(r.Broken), s.broken, r.Broken)
				}
			}
			v := gate.Decide(s.problems, r, s.undecided, full)
			if v.Exit != s.exit {
				t.Errorf("exit %d, want %d; reasons %q", v.Exit, s.exit, v.Reasons)
			}
		})
	}
}

// Contract: gate/G4
func TestContractErrorsFailEveryRunScopedOrNot(t *testing.T) {
	clean := matrix.Compute(*sound())
	for _, full := range []bool{true, false} {
		if v := gate.Decide([]problem.Problem{ctrErr}, clean, "", full); v.Exit != gate.Fail {
			t.Errorf("full = %v: a Contract error beside a clean matrix exits %d, want 1", full, v.Exit)
		}
		if v := gate.Decide([]problem.Problem{ctrErr}, nil, "", full); v.Exit != gate.Fail {
			t.Errorf("full = %v: a Contract error alone exits %d, want 1", full, v.Exit)
		}
	}

	// A whole run, scoped two ways to calc, still finds the error in another primitive.
	dir := copyFixture(t, "testdata/mod")
	write(t, filepath.Join(dir, "contract", "bad", "README.md"),
		"# bad\n\n- **B1** Nothing tests this.\n\n```covers\ncalc.Twice\n```\n")
	newRepo(t, dir)
	cleanGit(t)
	for _, o := range []engine.RunOptions{
		{Only: []string{"calc"}},
		{Since: "HEAD"},
	} {
		o.Options = engine.Options{Dir: dir, Contract: "contract"}
		o.Jobs, o.Coefficient = 2, 10
		rep, err := engine.Run(context.Background(), o)
		if err != nil {
			t.Fatal(err)
		}
		if rep.Exit != gate.Fail {
			t.Errorf("only %v, since %q: exit %d, want 1", o.Only, o.Since, rep.Exit)
		}
		if len(rep.Problems) != 1 || rep.Problems[0].Path != "contract/bad/README.md" {
			t.Errorf("only %v, since %q: Contract errors %v, want the one in contract/bad/README.md", o.Only, o.Since, rep.Problems)
		}
	}
}

// Contract: gate/G5
func TestNoScoreEntersTheVerdict(t *testing.T) {
	// 999 killed and one lived: a score of 99.9% still fails.
	in := sound()
	for i := 0; i < 998; i++ {
		m := mut("calc", "Add", "n + "+itoa(i), 3)
		addMutant(in, m, rowPtr(killedBy(tOne)), tOne)
	}
	addMutant(in, other, rowPtr(passedBy(tOne)), tOne)
	if v := gate.Decide(nil, matrix.Compute(*in), "", true); v.Exit != gate.Fail {
		t.Errorf("999 killed and 1 lived exit %d, want 1", v.Exit)
	}

	// One killed and fifty declared: a score of 100% over a single mutant passes.
	in = sound()
	for i := 0; i < 50; i++ {
		m := mut("calc", "Sub", "n - "+itoa(i), 9)
		addMutant(in, m, rowPtr(passedBy(tOne)), tOne)
		in.Declarations = append(in.Declarations, exact("unpromised", m))
	}
	if v := gate.Decide(nil, matrix.Compute(*in), "", true); v.Exit != gate.Pass {
		t.Errorf("1 killed and 50 declared exit %d, want 0", v.Exit)
	}

	// No mutants at all: there is no score, and nothing fails.
	empty := matrix.Compute(matrix.Input{Full: true})
	if v := gate.Decide(nil, empty, "", true); v.Exit != gate.Pass {
		t.Errorf("a run with no mutants exits %d, want 0", v.Exit)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
