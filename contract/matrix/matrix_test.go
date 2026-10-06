package matrix_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
	"github.com/tools4imps/flinch/internal/gate"
	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
)

// Contract: matrix/X1
func TestAnObligationHoldsWhatItsTestsKilled(t *testing.T) {
	one, two, three := ref("calc", "TestOne"), ref("calc", "TestTwo"), ref("calc", "TestThree")
	var (
		alone1 = mut("calc", "Add", "a + b", 3)
		shared = mut("calc", "Add", "a - b", 4)
		both   = mut("calc", "Sub", "a - b", 9)
		alone2 = mut("calc", "Sub", "a * b", 10)
		alone3 = mut("calc", "Mul", "a * b", 15)
		lived  = mut("calc", "Mul", "a / b", 16)
		broke  = mut("calc", "Div", "a / b", 20)
	)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "calc/C2", "calc/C3", "calc/C4"},
		// Out of order on purpose: the result lists each obligation's tests sorted.
		Tests: []model.Test{
			ctest("calc", "TestThree", "contract/calc/calc_test.go", 30, "calc/C3"),
			ctest("calc", "TestTwo", "contract/calc/calc_test.go", 20, "calc/C2", "calc/C1"),
			ctest("calc", "TestOne", "contract/calc/calc_test.go", 10, "calc/C1"),
		},
		Mutants: []model.Mutant{alone1, shared, both, alone2, alone3, lived, broke},
		Rows: map[string]model.Row{
			alone1.Hash: killedBy(model.Assertion, one),
			shared.Hash: killedBy(model.Assertion, two),
			// The row lists TestThree's kill first; the result sorts kills by test.
			both.Hash: {Ran: []model.TestRef{three, one}, Complete: true, Kills: []model.Kill{
				{Test: three, Kind: model.Assertion}, {Test: one, Kind: model.Panic, Subtests: []string{"TestOne/z", "TestOne/a"}},
			}},
			alone2.Hash: killedBy(model.Assertion, one),
			alone3.Hash: killedBy(model.Timeout, one),
			// Three more tests ran it too, so the order the result lists them in can't come by luck.
			lived.Hash: passedBy(ref("calc", "TestSix"), two, ref("calc", "TestFour"), one, three, ref("calc", "TestFive")),
			// A row without a verdict holds nothing, whatever it recorded.
			broke.Hash: {Ran: []model.TestRef{one}, Kills: []model.Kill{{Test: one, Kind: model.Assertion}}, Verdict: "it wouldn't build"},
		},
		Full: true,
	}
	r := matrix.Compute(in)

	for _, c := range []struct {
		id          string
		holds, sole []string
	}{
		{"calc/C1", hashes(alone1, shared, both, alone2, alone3), hashes(alone1, alone2, alone3)},
		{"calc/C2", hashes(shared), nil},
		{"calc/C3", hashes(both), nil},
		{"calc/C4", nil, nil},
	} {
		o := obligationIn(t, r, c.id)
		sameStrings(t, c.id+" holds", o.Holds, sorted(c.holds))
		sameStrings(t, c.id+" sole holds", o.SoleHolds, sorted(c.sole))
	}
	if got, want := obligationIn(t, r, "calc/C1").Tests, []model.TestRef{one, two}; !reflect.DeepEqual(got, want) {
		t.Errorf("calc/C1 is named by %v, want %v", got, want)
	}

	for _, c := range []struct {
		m    model.Mutant
		want model.Status
	}{
		{alone1, model.Killed}, {shared, model.Killed}, {both, model.Killed}, {alone3, model.Killed},
		{lived, model.Lived}, {broke, model.NoVerdict},
	} {
		if got := mutantIn(t, r, c.m).Status; got != c.want {
			t.Errorf("%s is %s, want %s", c.m.ID, got, c.want)
		}
	}

	var ranBy []string
	for _, tr := range mutantIn(t, r, lived).RanBy {
		ranBy = append(ranBy, tr.Name)
	}
	sameStrings(t, "the tests that ran the lived mutant", ranBy,
		[]string{"TestFive", "TestFour", "TestOne", "TestSix", "TestThree", "TestTwo"})

	kills := mutantIn(t, r, both).Kills
	if len(kills) != 2 {
		t.Fatalf("%s has %d kills, want 2: %+v", both.ID, len(kills), kills)
	}
	if kills[0].Test != one || kills[1].Test != three {
		t.Errorf("kills are by %s then %s, want calc/TestOne then calc/TestThree", kills[0].Test, kills[1].Test)
	}
	if kills[0].Kind != model.Panic || kills[1].Kind != model.Assertion {
		t.Errorf("kill kinds are %s and %s, want panic and assertion", kills[0].Kind, kills[1].Kind)
	}
	sameStrings(t, "TestOne's failing subtests", kills[0].Subtests, []string{"TestOne/a", "TestOne/z"})
	sameStrings(t, "TestOne's kill obligations", kills[0].Obligations, []string{"calc/C1"})
	sameStrings(t, "TestTwo's kill obligations", mutantIn(t, r, shared).Kills[0].Obligations, []string{"calc/C1", "calc/C2"})
}

// Contract: matrix/X1
func TestARunCreditsEachKillToTheKillersObligations(t *testing.T) {
	dir := copyFixture(t, "testdata/mod")
	rep, err := engine.Run(context.Background(), engine.RunOptions{
		Options: engine.Options{Dir: dir, Contract: "contract"}, Jobs: 2, Coefficient: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Matrix == nil {
		t.Fatalf("the run stopped before mutating: %v", rep.Problems)
	}
	byID := map[string]matrix.Mutant{}
	for _, m := range rep.Matrix.Mutants {
		byID[m.ID] = m
	}
	erase, ok := byID["calc.Max: { ... } -> { return *new(int) }"]
	if !ok {
		t.Fatalf("the run has no erase mutant for calc.Max; its mutants are %v", keysOf(byID))
	}
	if erase.Status != model.Killed || len(erase.Kills) != 1 || erase.Kills[0].Test.String() != "calc/TestMax" {
		t.Fatalf("calc.Max's erase mutant is %s with kills %+v, want killed by calc/TestMax", erase.Status, erase.Kills)
	}
	sameStrings(t, "the killer's obligations", erase.Kills[0].Obligations, []string{"calc/C1"})
	if boundary := byID["calc.Max: a > b -> a >= b"]; boundary.Status != model.Lived {
		t.Errorf("calc.Max's boundary mutant is %q, want lived", boundary.Status)
	}
	o := obligationIn(t, rep.Matrix, "calc/C1")
	sameStrings(t, "calc/C1 holds", o.Holds, []string{erase.Hash})
	sameStrings(t, "calc/C1 sole holds", o.SoleHolds, []string{erase.Hash})
}

// Contract: matrix/X2
func TestInAFullRunAnObligationHoldingNothingIsHollow(t *testing.T) {
	one, two := ref("calc", "TestOne"), ref("calc", "TestTwo")
	held, passed := mut("calc", "Add", "a + b", 3), mut("calc", "Add", "a - b", 4)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "calc/C2", "calc/C3"},
		Tests: []model.Test{
			ctest("calc", "TestOne", "contract/calc/calc_test.go", 10, "calc/C1"),
			ctest("calc", "TestTwo", "contract/calc/calc_test.go", 20, "calc/C2"),
		},
		Mutants: []model.Mutant{held, passed},
		Rows: map[string]model.Row{
			held.Hash:   killedBy(model.Assertion, one),
			passed.Hash: passedBy(one, two),
		},
		Full: true,
	}
	r := matrix.Compute(in)
	for _, c := range []struct {
		id     string
		hollow bool
	}{
		{"calc/C1", false}, // holds a mutant
		{"calc/C2", true},  // its test ran and killed nothing
		{"calc/C3", true},  // no test names it
	} {
		o := obligationIn(t, r, c.id)
		if !o.Judged {
			t.Errorf("%s isn't judged in a full run", c.id)
		}
		if o.Hollow != c.hollow {
			t.Errorf("%s hollow = %v, want %v", c.id, o.Hollow, c.hollow)
		}
	}
}

// Contract: matrix/X3
func TestInAFullRunATestThatKillsNothingInACompleteRowIsBlind(t *testing.T) {
	var (
		sees       = ref("calc", "TestSees")
		blind      = ref("calc", "TestBlind")
		incomplete = ref("calc", "TestIncomplete")
		declared   = ref("calc", "TestDeclaredOnly")
		wild       = ref("calc", "TestWildcardOnly")
		idle       = ref("calc", "TestIdle")
	)
	var (
		killed     = mut("calc", "Add", "a + b", 3)
		lived1     = mut("calc", "Add", "a - b", 4)
		lived2     = mut("calc", "Sub", "a - b", 8)
		unfinished = mut("calc", "Sub", "a * b", 9)
		excused    = mut("calc", "Mul", "a * b", 12)
		inClosure  = mut("calc", "Div.func1", "a / b", 20)
	)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "calc/C2"},
		// Out of order on purpose: the result sorts tests by file, then line, then name.
		Tests: []model.Test{
			ctest("calc", "TestWildcardOnly", "contract/calc/b_test.go", 5, "calc/C2"),
			ctest("calc", "TestIdle", "contract/calc/a_test.go", 90, "calc/C2"),
			ctest("calc", "TestDeclaredOnly", "contract/calc/a_test.go", 40, "calc/C2"),
			ctest("calc", "TestIncomplete", "contract/calc/a_test.go", 30, "calc/C2"),
			ctest("calc", "TestBlind", "contract/calc/a_test.go", 20, "calc/C2"),
			ctest("calc", "TestSees", "contract/calc/a_test.go", 10, "calc/C1"),
		},
		Mutants: []model.Mutant{killed, lived1, lived2, unfinished, excused, inClosure},
		Rows: map[string]model.Row{
			killed.Hash: {Ran: []model.TestRef{sees, blind}, Complete: true, Kills: []model.Kill{{Test: sees, Kind: model.Assertion}}},
			lived1.Hash: passedBy(sees, blind),
			lived2.Hash: passedBy(blind),
			// TestIncomplete passed in a row that a crash left incomplete.
			unfinished.Hash: {Ran: []model.TestRef{incomplete, sees}, Kills: []model.Kill{{Test: sees, Kind: model.Panic}}},
			excused.Hash:    passedBy(declared),
			inClosure.Hash:  passedBy(wild),
		},
		Declarations: []matrix.Declaration{
			exact("equivalent", "calc", excused, 3),
			wildcard("calc", "calc", "Div", 9),
		},
		Full: true,
	}
	r := matrix.Compute(in)
	if len(r.Broken) > 0 {
		t.Fatalf("the declarations here are sound, yet the matrix calls them broken: %v", r.Broken)
	}
	for _, m := range []model.Mutant{excused, inClosure} {
		got := mutantIn(t, r, m)
		if got.Status != model.Declared || got.Declaration == nil {
			t.Errorf("%s is %s with declaration %v, want declared", m.ID, got.Status, got.Declaration)
		}
	}

	for _, c := range []struct {
		ref   model.TestRef
		ran   int
		blind bool
	}{
		{sees, 2, false},
		{blind, 3, true},
		{incomplete, 0, false},
		{declared, 0, false},
		{wild, 0, false},
		{idle, 0, false},
	} {
		got := testIn(t, r, c.ref)
		if !got.Judged {
			t.Errorf("%s isn't judged in a full run", c.ref)
		}
		if got.Ran != c.ran || got.Blind != c.blind {
			t.Errorf("%s ran in %d complete rows of undeclared mutants and blind = %v, want %d and %v",
				c.ref, got.Ran, got.Blind, c.ran, c.blind)
		}
	}
	sameStrings(t, "TestSees's kills", testIn(t, r, sees).Kills, sorted(hashes(killed, unfinished)))

	var order []string
	for _, x := range r.Tests {
		order = append(order, x.Name)
	}
	sameStrings(t, "test order", order, []string{"TestSees", "TestBlind", "TestIncomplete", "TestDeclaredOnly", "TestIdle", "TestWildcardOnly"})
}

// Contract: matrix/X4
func TestAScopedRunJudgesOnlyPrimitivesItMutatedWhole(t *testing.T) {
	c1, d1 := ref("calc", "TestCalc"), ref("dial", "TestDial")
	cm, dm := mut("calc", "Add", "a + b", 3), mut("dial", "Turn", "n + 1", 7)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "dial/D1"},
		Tests: []model.Test{
			ctest("calc", "TestCalc", "contract/calc/calc_test.go", 10, "calc/C1"),
			ctest("dial", "TestDial", "contract/dial/dial_test.go", 10, "dial/D1"),
		},
		Mutants: []model.Mutant{cm, dm},
		Rows:    map[string]model.Row{cm.Hash: passedBy(c1), dm.Hash: passedBy(d1)},
		Full:    false,
		Whole:   map[string]bool{"calc": true},
	}
	r := matrix.Compute(in)

	if o := obligationIn(t, r, "calc/C1"); !o.Judged || !o.Hollow {
		t.Errorf("calc/C1 judged = %v, hollow = %v; calc was mutated whole, so want both", o.Judged, o.Hollow)
	}
	if o := obligationIn(t, r, "dial/D1"); o.Judged || o.Hollow {
		t.Errorf("dial/D1 judged = %v, hollow = %v; dial wasn't mutated whole, so want neither", o.Judged, o.Hollow)
	}
	if x := testIn(t, r, c1); !x.Judged || !x.Blind {
		t.Errorf("calc/TestCalc judged = %v, blind = %v; want both", x.Judged, x.Blind)
	}
	if x := testIn(t, r, d1); x.Judged || x.Blind {
		t.Errorf("dial/TestDial judged = %v, blind = %v; want neither", x.Judged, x.Blind)
	}
}

// Contract: matrix/X5
func TestAnObligationHeldOnlyByCrashesIsReportedAndPasses(t *testing.T) {
	var (
		panics   = ref("calc", "TestPanics")
		times    = ref("calc", "TestTimesOut")
		asserts  = ref("calc", "TestAsserts")
		mixed    = ref("calc", "TestMixed")
		passes   = ref("calc", "TestPasses")
		viaPanic = mut("calc", "Add", "a + b", 3)
		viaTime  = mut("calc", "Add", "a - b", 4)
		viaBoth  = mut("calc", "Sub", "a - b", 8)
		viaAssrt = mut("calc", "Sub", "a * b", 9)
		onlyTime = mut("calc", "Mul", "a * b", 12)
	)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "calc/C2", "calc/C3", "calc/C4", "calc/C5"},
		Tests: []model.Test{
			ctest("calc", "TestPanics", "contract/calc/calc_test.go", 10, "calc/C1"),
			ctest("calc", "TestTimesOut", "contract/calc/calc_test.go", 20, "calc/C1"),
			ctest("calc", "TestMixed", "contract/calc/calc_test.go", 30, "calc/C2"),
			ctest("calc", "TestPasses", "contract/calc/calc_test.go", 40, "calc/C3"),
			ctest("calc", "TestAsserts", "contract/calc/calc_test.go", 50, "calc/C4"),
			ctest("calc", "TestTimesOut2", "contract/calc/calc_test.go", 60, "calc/C5"),
		},
		Mutants: []model.Mutant{viaPanic, viaTime, viaBoth, viaAssrt, onlyTime},
		Rows: map[string]model.Row{
			viaPanic.Hash: killedBy(model.Panic, panics),
			viaTime.Hash:  killedBy(model.Timeout, times),
			viaBoth.Hash: {Ran: []model.TestRef{mixed, passes}, Complete: true, Kills: []model.Kill{
				{Test: mixed, Kind: model.Panic},
			}},
			viaAssrt.Hash: {Ran: []model.TestRef{mixed, asserts, passes}, Complete: true, Kills: []model.Kill{
				{Test: mixed, Kind: model.Assertion}, {Test: asserts, Kind: model.Assertion},
			}},
			onlyTime.Hash: killedBy(model.Timeout, ref("calc", "TestTimesOut2")),
		},
		Full: true,
	}
	r := matrix.Compute(in)
	for _, c := range []struct {
		id        string
		crashOnly bool
	}{
		{"calc/C1", true},  // a panic and a timeout, no assertion
		{"calc/C2", false}, // a panic, and an assertion elsewhere
		{"calc/C3", false}, // holds nothing at all
		{"calc/C4", false}, // assertions only
		{"calc/C5", true},  // a timeout only
	} {
		if got := obligationIn(t, r, c.id).CrashOnly; got != c.crashOnly {
			t.Errorf("%s held only by crashes = %v, want %v", c.id, got, c.crashOnly)
		}
	}
	// calc/C3 is hollow and TestPasses is blind, which fail the build on their own. Without them,
	// the crash-only holds alone must pass.
	in.Obligations = []string{"calc/C1", "calc/C2", "calc/C4", "calc/C5"}
	in.Rows[viaBoth.Hash] = model.Row{Ran: []model.TestRef{mixed}, Complete: true, Kills: []model.Kill{{Test: mixed, Kind: model.Panic}}}
	in.Rows[viaAssrt.Hash] = model.Row{Ran: []model.TestRef{mixed, asserts}, Complete: true, Kills: []model.Kill{
		{Test: mixed, Kind: model.Assertion}, {Test: asserts, Kind: model.Assertion},
	}}
	in.Tests = append(in.Tests[:3:3], in.Tests[4:]...)
	r = matrix.Compute(in)
	if !obligationIn(t, r, "calc/C1").CrashOnly {
		t.Fatal("calc/C1 should still be held only by crashes")
	}
	if v := gate.Decide(nil, r, "", true); v.Exit != gate.Pass {
		t.Errorf("an obligation held only by crashes failed the build: exit %d, %v", v.Exit, v.Reasons)
	}
}

// Contract: matrix/X6
func TestAMutantOnlyOtherPrimitivesKillIsHeldFromOutsideAndPasses(t *testing.T) {
	own, other, third := ref("calc", "TestCalc"), ref("dial", "TestDial"), ref("dial", "TestDial2")
	var (
		outside = mut("calc", "Add", "a + b", 3)
		both    = mut("calc", "Add", "a - b", 4)
		inside  = mut("calc", "Sub", "a - b", 8)
		dials   = mut("dial", "Turn", "n + 1", 5)
	)
	in := matrix.Input{
		Obligations: []string{"calc/C1", "dial/D1"},
		Tests: []model.Test{
			ctest("calc", "TestCalc", "contract/calc/calc_test.go", 10, "calc/C1"),
			ctest("dial", "TestDial", "contract/dial/dial_test.go", 10, "dial/D1"),
			ctest("dial", "TestDial2", "contract/dial/dial_test.go", 20, "dial/D1"),
		},
		Mutants: []model.Mutant{outside, both, inside, dials},
		Rows: map[string]model.Row{
			outside.Hash: {Ran: []model.TestRef{own, other, third}, Complete: true, Kills: []model.Kill{
				{Test: other, Kind: model.Assertion}, {Test: third, Kind: model.Assertion},
			}},
			both.Hash:   killedBy(model.Assertion, other, own),
			inside.Hash: killedBy(model.Assertion, own),
			dials.Hash:  killedBy(model.Assertion, other, third),
		},
		Full: true,
	}
	r := matrix.Compute(in)
	for _, c := range []struct {
		m    model.Mutant
		want bool
	}{
		{outside, true}, {both, false}, {inside, false}, {dials, false},
	} {
		if got := mutantIn(t, r, c.m).OutsideOnly; got != c.want {
			t.Errorf("%s held only from outside = %v, want %v", c.m.ID, got, c.want)
		}
	}
	if v := gate.Decide(nil, r, "", true); v.Exit != gate.Pass {
		t.Errorf("a mutant held only from outside failed the build: exit %d, %v", v.Exit, v.Reasons)
	}
}

// copyFixture copies a fixture module into a temp directory and returns its path, so a run never
// writes next to the Contract.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func keysOf(m map[string]matrix.Mutant) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return sorted(out)
}
