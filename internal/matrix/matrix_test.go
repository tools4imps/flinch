package matrix

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
)

// mut makes a mutant in internal/skip, owned by skip, with its id built from the unit and change.
func mut(unit, top, orig, repl string, line int) model.Mutant {
	id := mutantid.ID{Dir: "internal/skip", Unit: unit, Original: orig, Replacement: repl, N: 1}
	return model.Mutant{
		ID: id.String(), Hash: id.Hash(), Primitive: "skip", Dir: "internal/skip",
		File: "internal/skip/skip.go", Line: line, Unit: unit, Top: top,
		Original: orig, Replace: repl, Operator: "boundary",
	}
}

func erase(unit string, line int) model.Mutant {
	m := mut(unit, unit, "{ ... }", "{ return }", line)
	m.Erase, m.Operator = true, "erase"
	return m
}

var (
	tSkip1   = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestOne"}, File: "contract/skip/skip_test.go", Line: 10, Obligations: []string{"skip/K1"}}
	tSkip2   = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestTwo"}, File: "contract/skip/skip_test.go", Line: 20, Obligations: []string{"skip/K2"}}
	tSkip3   = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestThree"}, File: "contract/skip/skip_test.go", Line: 30, Obligations: []string{"skip/K1", "skip/K3"}}
	tOther   = model.Test{TestRef: model.TestRef{Primitive: "other", Name: "TestOther"}, File: "contract/other/other_test.go", Line: 5, Obligations: []string{"other/O1"}}
	allTests = []model.Test{tSkip1, tSkip2, tSkip3, tOther}
	allObs   = []string{"skip/K1", "skip/K2", "skip/K3", "other/O1"}
)

func kill(t model.Test, kind model.KillKind) model.Kill {
	return model.Kill{Test: t.TestRef, Kind: kind}
}

func killedRow(kills ...model.Kill) model.Row {
	r := model.Row{Kills: kills, Complete: true}
	for _, k := range kills {
		r.Ran = append(r.Ran, k.Test)
	}
	return r
}

func livedRow(ran ...model.Test) model.Row {
	r := model.Row{Complete: true}
	for _, t := range ran {
		r.Ran = append(r.Ran, t.TestRef)
	}
	return r
}

func byHash(r *Result) map[string]Mutant {
	out := map[string]Mutant{}
	for _, m := range r.Mutants {
		out[m.Hash] = m
	}
	return out
}

func obligation(r *Result, id string) Obligation {
	for _, o := range r.Obligations {
		if o.ID == id {
			return o
		}
	}
	return Obligation{}
}

func test(r *Result, name string) Test {
	for _, t := range r.Tests {
		if t.Name == name {
			return t
		}
	}
	return Test{}
}

func TestEveryStatus(t *testing.T) {
	killed := mut("Match", "Match", "a", "b", 3)
	lived := mut("Match", "Match", "c", "d", 4)
	unreached := mut("Match", "Match", "e", "f", 5)
	noVerdict := mut("Match", "Match", "g", "h", 6)
	orphan := mut("Match", "Match", "i", "j", 7)
	eraseF := erase("Walk", 10)
	inF := mut("Walk", "Walk", "k", "l", 11)
	inClosure := mut("Walk.func1", "Walk", "m", "n", 12)
	eraseG := erase("Keep", 20)
	inG := mut("Keep", "Keep", "o", "p", 21)

	in := Input{
		Obligations: allObs,
		Tests:       allTests,
		Mutants:     []model.Mutant{killed, lived, unreached, noVerdict, orphan, eraseF, inF, inClosure, eraseG, inG},
		Rows: map[string]model.Row{
			killed.Hash:    killedRow(kill(tSkip1, model.Assertion)),
			lived.Hash:     livedRow(tSkip1),
			noVerdict.Hash: {Verdict: "the mutant type-checked but won't build"},
			eraseF.Hash:    livedRow(tSkip2),
			eraseG.Hash:    killedRow(kill(tSkip2, model.Assertion)),
		},
		Reached: map[string][]model.TestRef{
			orphan.Hash: {tSkip1.TestRef},
			inF.Hash:    {tSkip2.TestRef},
		},
	}
	got := byHash(Compute(in))
	want := map[string]model.Status{
		killed.Hash:    model.Killed,
		lived.Hash:     model.Lived,
		unreached.Hash: model.Unreached,
		noVerdict.Hash: model.NoVerdict,
		orphan.Hash:    model.NoVerdict,
		eraseF.Hash:    model.Erased,
		inF.Hash:       model.Skipped,
		inClosure.Hash: model.Skipped,
		eraseG.Hash:    model.Killed,
		inG.Hash:       model.Unreached,
	}
	for h, s := range want {
		if got[h].Status != s {
			t.Errorf("%s: status %q, want %q", got[h].ID, got[h].Status, s)
		}
	}
	if got[noVerdict.Hash].Verdict == "" || got[orphan.Hash].Verdict == "" {
		t.Error("a mutant without a verdict should say why")
	}
	if rb := got[unreached.Hash].RanBy; len(rb) != 0 {
		t.Errorf("an unreached mutant has no tests running it, got %v", rb)
	}
	if ob := got[lived.Hash].Obligations; !reflect.DeepEqual(ob, []string{"skip/K1"}) {
		t.Errorf("lived mutant's obligations = %v, want [skip/K1]", ob)
	}
	if k := got[killed.Hash].Kills; len(k) != 1 || !reflect.DeepEqual(k[0].Obligations, []string{"skip/K1"}) {
		t.Errorf("a kill should carry its test's obligations, got %+v", k)
	}
}

func TestExactDeclarations(t *testing.T) {
	lived := mut("Match", "Match", "a", "b", 3)
	killed := mut("Match", "Match", "c", "d", 4)
	unreached := mut("Match", "Match", "e", "f", 5)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{lived, killed, unreached},
		Rows: map[string]model.Row{
			lived.Hash:  livedRow(tSkip1),
			killed.Hash: killedRow(kill(tSkip1, model.Assertion), kill(tSkip2, model.Assertion)),
		},
		Declarations: []Declaration{
			{Kind: "equivalent", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 9, ID: lived.ID, Reason: "Same"},
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 4, ID: killed.ID},
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 5, ID: unreached.ID},
		},
	}
	r := Compute(in)
	got := byHash(r)
	if got[lived.Hash].Status != model.Declared || got[lived.Hash].Declaration.Line != 9 {
		t.Errorf("a declared lived mutant should be declared by line 9, got %q %+v", got[lived.Hash].Status, got[lived.Hash].Declaration)
	}
	if got[unreached.Hash].Status != model.Declared {
		t.Errorf("a declared unreached mutant should be declared, got %q", got[unreached.Hash].Status)
	}
	if got[killed.Hash].Status != model.Killed || got[killed.Hash].Declaration == nil {
		t.Errorf("a killed mutant stays killed and keeps its declaration, got %q", got[killed.Hash].Status)
	}
	if len(r.Broken) != 1 {
		t.Fatalf("want one broken declaration, got %v", r.Broken)
	}
	b := r.Broken[0]
	if b.Path != "contract/skip/mutants.md" || b.Line != 4 ||
		!strings.Contains(b.Message, "skip/TestOne, skip/TestTwo kill this mutant") ||
		!strings.Contains(b.Message, "the Contract does promise this behavior") ||
		!strings.Contains(b.Message, "delete the line") {
		t.Errorf("broken declaration reads %q", b)
	}
}

func TestStaleExactDeclarationOnlyWhenItsUnitWasMutatedWhole(t *testing.T) {
	present := mut("Walk", "Walk", "a", "b", 3)
	gone := mutantid.ID{Dir: "internal/skip", Unit: "Walk.func1", Original: "x", Replacement: "y", N: 1}
	d := Declaration{Kind: "equivalent", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 7, ID: gone.String()}
	base := Input{
		Obligations: allObs, Tests: allTests,
		Mutants:      []model.Mutant{present},
		Rows:         map[string]model.Row{present.Hash: killedRow(kill(tSkip1, model.Assertion))},
		Declarations: []Declaration{d},
	}
	scoped := base
	if r := Compute(scoped); len(r.Broken) != 0 {
		t.Errorf("a scoped run that didn't mutate the unit whole called it stale: %v", r.Broken)
	}
	other := base
	other.WholeUnits = map[string]bool{"internal/skip.Keep": true}
	if r := Compute(other); len(r.Broken) != 0 {
		t.Errorf("mutating another unit whole made this declaration stale: %v", r.Broken)
	}
	for name, in := range map[string]Input{
		"full":      withScope(base, true, nil, nil),
		"primitive": withScope(base, false, map[string]bool{"skip": true}, nil),
		"unit":      withScope(base, false, nil, map[string]bool{"internal/skip.Walk": true}),
	} {
		r := Compute(in)
		if len(r.Broken) != 1 || r.Broken[0].Line != 7 || !strings.Contains(r.Broken[0].Message, "stale") {
			t.Errorf("%s: want one stale declaration at line 7, got %v", name, r.Broken)
		}
	}
}

func withScope(in Input, full bool, whole, units map[string]bool) Input {
	in.Full, in.Whole, in.WholeUnits = full, whole, units
	return in
}

func TestWildcards(t *testing.T) {
	lived := mut("Walk", "Walk", "a", "b", 3)
	closure := mut("Walk.func1", "Walk", "c", "d", 4)
	killed := mut("Walk", "Walk", "e", "f", 5)
	exact := mut("Walk", "Walk", "g", "h", 6)
	neighbour := mut("Walker", "Walker", "i", "j", 7)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{lived, closure, killed, exact, neighbour},
		Rows: map[string]model.Row{
			lived.Hash:  livedRow(tSkip1),
			killed.Hash: killedRow(kill(tSkip1, model.Assertion)),
			exact.Hash:  livedRow(tSkip1),
		},
		Declarations: []Declaration{
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 3, Wildcard: true, Dir: "internal/skip", Unit: "Walk"},
			{Kind: "equivalent", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 12, ID: exact.ID},
		},
		Full: true,
	}
	r := Compute(in)
	got := byHash(r)
	for _, h := range []string{lived.Hash, closure.Hash} {
		if got[h].Status != model.Declared || !got[h].Declaration.Wildcard {
			t.Errorf("%s should be declared by the wildcard, got %q", got[h].ID, got[h].Status)
		}
	}
	if got[killed.Hash].Status != model.Killed || got[killed.Hash].Declaration != nil {
		t.Errorf("a wildcard leaves killed mutants alone, got %q %+v", got[killed.Hash].Status, got[killed.Hash].Declaration)
	}
	if d := got[exact.Hash].Declaration; d == nil || d.Wildcard || d.Line != 12 {
		t.Errorf("an exact declaration should win over a wildcard, got %+v", d)
	}
	if got[neighbour.Hash].Status != model.Unreached {
		t.Errorf("a wildcard on Walk reached into Walker: %q", got[neighbour.Hash].Status)
	}
	if len(r.Broken) != 0 {
		t.Errorf("no declaration here is broken or stale: %v", r.Broken)
	}
}

func TestStaleWildcard(t *testing.T) {
	killed := mut("Walk", "Walk", "a", "b", 3)
	undecided := mut("Keep", "Keep", "c", "d", 4)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{killed, undecided},
		Rows: map[string]model.Row{
			killed.Hash:    killedRow(kill(tSkip1, model.Assertion)),
			undecided.Hash: {Verdict: "crashed three times"},
		},
		Declarations: []Declaration{
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 3, Wildcard: true, Dir: "internal/skip", Unit: "Walk"},
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 8, Wildcard: true, Dir: "internal/skip", Unit: "Keep"},
		},
	}
	if r := Compute(in); len(r.Broken) != 0 {
		t.Errorf("a wildcard on a unit not mutated whole is never stale: %v", r.Broken)
	}
	in.WholeUnits = map[string]bool{"internal/skip.Walk": true, "internal/skip.Keep": true}
	r := Compute(in)
	if len(r.Broken) != 1 || r.Broken[0].Line != 3 ||
		!strings.Contains(r.Broken[0].Message, "internal/skip.Walk has no unheld mutants") {
		t.Errorf("want the Walk wildcard stale and the undecided Keep one left alone, got %v", r.Broken)
	}
}

func TestHoldsAndSoleHolds(t *testing.T) {
	a := mut("Match", "Match", "a", "b", 3)
	b := mut("Match", "Match", "c", "d", 4)
	c := mut("Match", "Match", "e", "f", 5)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{a, b, c},
		Rows: map[string]model.Row{
			a.Hash: killedRow(kill(tSkip1, model.Assertion)),
			b.Hash: killedRow(kill(tSkip1, model.Assertion), kill(tSkip2, model.Assertion)),
			c.Hash: killedRow(kill(tSkip3, model.Assertion)),
		},
		Full: true,
	}
	r := Compute(in)
	k1, k2, k3 := obligation(r, "skip/K1"), obligation(r, "skip/K2"), obligation(r, "skip/K3")
	if len(k1.Holds) != 3 || !reflect.DeepEqual(k1.SoleHolds, []string{a.Hash}) {
		t.Errorf("K1 holds %v alone %v; want three holds and only %s alone", k1.Holds, k1.SoleHolds, a.Hash)
	}
	if len(k2.Holds) != 1 || len(k2.SoleHolds) != 0 {
		t.Errorf("K2 holds %v alone %v; want one hold, none alone", k2.Holds, k2.SoleHolds)
	}
	if len(k3.Holds) != 1 || len(k3.SoleHolds) != 0 {
		t.Errorf("K3 shares its only hold with K1 through one test; got holds %v alone %v", k3.Holds, k3.SoleHolds)
	}
	if !reflect.DeepEqual(k1.Tests, []model.TestRef{tSkip1.TestRef, tSkip3.TestRef}) {
		t.Errorf("K1's tests = %v", k1.Tests)
	}
}

func TestHollowAndBlindAreJudgedOnlyInScope(t *testing.T) {
	a := mut("Match", "Match", "a", "b", 3)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{a},
		Rows: map[string]model.Row{
			a.Hash: {Ran: []model.TestRef{tSkip1.TestRef, tSkip2.TestRef}, Kills: []model.Kill{kill(tSkip1, model.Assertion)}, Complete: true},
		},
	}
	r := Compute(in)
	if o := obligation(r, "skip/K2"); o.Hollow || o.Judged {
		t.Errorf("a scoped run without skip whole judged K2: %+v", o)
	}
	if tt := test(r, "TestTwo"); tt.Blind || tt.Judged {
		t.Errorf("a scoped run without skip whole judged TestTwo: %+v", tt)
	}

	in.Whole = map[string]bool{"skip": true}
	r = Compute(in)
	if !obligation(r, "skip/K2").Hollow || !obligation(r, "skip/K3").Hollow || obligation(r, "skip/K1").Hollow {
		t.Error("with skip whole, K2 and K3 hold nothing and are hollow, and K1 isn't")
	}
	if o := obligation(r, "other/O1"); o.Hollow {
		t.Error("other wasn't mutated whole, so O1 isn't judged")
	}
	if !test(r, "TestTwo").Blind || test(r, "TestOne").Blind || test(r, "TestThree").Blind {
		t.Error("TestTwo ran a complete row and killed nothing; TestOne killed; TestThree never ran")
	}

	in.Whole, in.Full = nil, true
	r = Compute(in)
	if !obligation(r, "other/O1").Hollow || !test(r, "TestTwo").Blind {
		t.Error("a full run judges every obligation and test")
	}
}

func TestBlindNeedsACompleteRowOfAnUndeclaredMutant(t *testing.T) {
	partial := mut("Match", "Match", "a", "b", 3)
	declared := mut("Match", "Match", "c", "d", 4)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{partial, declared},
		Rows: map[string]model.Row{
			partial.Hash:  {Ran: []model.TestRef{tSkip2.TestRef}, Complete: false},
			declared.Hash: livedRow(tSkip2),
		},
		Declarations: []Declaration{{Kind: "equivalent", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 3, ID: declared.ID}},
		Full:         true,
	}
	r := Compute(in)
	if tt := test(r, "TestTwo"); tt.Blind || tt.Ran != 0 {
		t.Errorf("a test seen only in an incomplete row and a declared mutant's row isn't blind: %+v", tt)
	}
}

func TestCrashOnly(t *testing.T) {
	a := mut("Match", "Match", "a", "b", 3)
	b := mut("Match", "Match", "c", "d", 4)
	c := mut("Match", "Match", "e", "f", 5)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{a, b, c},
		Rows: map[string]model.Row{
			a.Hash: killedRow(kill(tSkip1, model.Panic)),
			b.Hash: killedRow(kill(tSkip1, model.Timeout)),
			c.Hash: killedRow(kill(tSkip2, model.Panic), kill(tSkip3, model.Assertion)),
		},
	}
	r := Compute(in)
	if !obligation(r, "skip/K2").CrashOnly {
		t.Error("K2 holds c only through a panic")
	}
	if obligation(r, "skip/K1").CrashOnly {
		t.Error("K1 holds c through TestThree's assertion, so not every hold came from a crash")
	}
	if obligation(r, "other/O1").CrashOnly {
		t.Error("an obligation that holds nothing isn't held only by crashes")
	}
}

func TestOutsideOnly(t *testing.T) {
	outside := mut("Match", "Match", "a", "b", 3)
	both := mut("Match", "Match", "c", "d", 4)
	in := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{outside, both},
		Rows: map[string]model.Row{
			outside.Hash: killedRow(kill(tOther, model.Assertion)),
			both.Hash:    killedRow(kill(tOther, model.Assertion), kill(tSkip1, model.Assertion)),
		},
	}
	got := byHash(Compute(in))
	if !got[outside.Hash].OutsideOnly || got[both.Hash].OutsideOnly {
		t.Errorf("outside-only: got %v and %v, want true and false", got[outside.Hash].OutsideOnly, got[both.Hash].OutsideOnly)
	}
	if o := obligation(Compute(in), "other/O1"); len(o.Holds) != 2 {
		t.Errorf("an obligation holds what its tests kill in any primitive's code, got %v", o.Holds)
	}
}

func TestDeterministicWhateverTheInputOrder(t *testing.T) {
	a := mut("Match", "Match", "a", "b", 3)
	b := mut("Match", "Match", "c", "d", 4)
	one := Input{
		Obligations: allObs, Tests: allTests,
		Mutants: []model.Mutant{a, b},
		Rows: map[string]model.Row{
			a.Hash: killedRow(kill(tSkip3, model.Assertion), kill(tSkip1, model.Panic)),
			b.Hash: livedRow(tSkip2, tSkip1),
		},
		Reached: map[string][]model.TestRef{b.Hash: {tSkip3.TestRef, tSkip2.TestRef}},
		Full:    true,
	}
	two := one
	two.Tests = []model.Test{tOther, tSkip3, tSkip2, tSkip1}
	two.Rows = map[string]model.Row{
		a.Hash: killedRow(kill(tSkip1, model.Panic), kill(tSkip3, model.Assertion)),
		b.Hash: livedRow(tSkip1, tSkip2),
	}
	two.Reached = map[string][]model.TestRef{b.Hash: {tSkip2.TestRef, tSkip3.TestRef}}
	r1, r2 := Compute(one), Compute(two)
	if !reflect.DeepEqual(r1.Mutants, r2.Mutants) || !reflect.DeepEqual(r1.Obligations, r2.Obligations) {
		t.Error("the same run in a different order gave different mutants or obligations")
	}
	for i := range r1.Tests {
		if r1.Tests[i].TestRef != r2.Tests[i].TestRef || !reflect.DeepEqual(r1.Tests[i].Kills, r2.Tests[i].Kills) {
			t.Errorf("tests differ at %d: %v and %v", i, r1.Tests[i].TestRef, r2.Tests[i].TestRef)
		}
	}
}
