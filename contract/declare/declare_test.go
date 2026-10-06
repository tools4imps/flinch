package declare_test

import (
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/declare"
)

// The mutant ids in the run fixture that these tests declare.
const (
	closureID = "calc.Add.func1: a > b -> a >= b" // no Contract test calls Add's closure
	sumID     = "calc.Add: a + b -> a - b"        // TestAddSums kills it
)

// closureBlock declares the run fixture's one unheld mutant, so a run over it can pass.
var closureBlock = md(`## Add's debugging closure is open

Nothing promises what the closure says.

'''unpromised
` + closureID + `
'''
`)

// Contract: declare/D1
func TestAnEquivalentBlockListsIdsAndNeverAWildcard(t *testing.T) {
	text := md(`## The same value either way

Swapping the operands of + changes nothing.

'''equivalent
a.F: x + 1 -> 1 + x
a.G: x - y -> x + y
a.G: *
'''
`)
	c, ps := check(t, map[string]string{"p": text})
	expectAt(t, ps, "contract/p/mutants.md", lineOf(t, text, "a.G: *"))

	ix := declare.NewIndex(c)
	for id, prefix := range map[string]string{
		"a.F: x + 1 -> 1 + x":       "a.F: x + 1",
		"a.F:  x  +  1  ->  1 +  x": "a.F: x + 1",
		"a.F:\tx + 1\t->\t1 + x":    "a.F: x + 1",
		"a.G: x - y -> x + y":       "a.G: x - y",
	} {
		d := ix.Exact(id)
		if d == nil || d.Kind != "equivalent" || d.Line != lineOf(t, text, prefix) {
			t.Errorf("Exact(%q) = %+v, want the equivalent declaration on line %d", id, d, lineOf(t, text, prefix))
		}
	}
	if d := ix.Wildcard("a", "G"); d != nil {
		t.Errorf("Wildcard(a, G) = %+v, but a wildcard in an equivalent block declares nothing", *d)
	}
	if d := ix.Exact("a.F: x + 1 -> x - 1"); d != nil {
		t.Errorf("Exact found %+v for a mutant no line lists", *d)
	}
}

// Contract: declare/D2
func TestAnUnpromisedBlockListsIdsAndWildcards(t *testing.T) {
	text := md(`## Whatever F and init do is open

'''unpromised
a.F: *
a.T.M: 1 -> 0
a.init.0: *
'''
`)
	c, ps := check(t, map[string]string{"p": text})
	expectAt(t, ps, "contract/p/mutants.md")

	ix := declare.NewIndex(c)
	f, init := lineOf(t, text, "a.F: *"), lineOf(t, text, "a.init.0: *")
	for _, w := range []struct {
		dir, unit string
		line      int
	}{
		{"a", "F", f},
		{"a", "F.func1", f},
		{"a", "F.func1.func2", f},
		{"a", "init.0", init},
		{"a", "init.0.func1", init},
		{"a", "G", 0},
		{"a", "G.func1", 0},
		{"a", "Ffunc1", 0},
		{"a", "T.M", 0},
		{"b", "F", 0},
	} {
		d := ix.Wildcard(w.dir, w.unit)
		switch {
		case w.line == 0 && d != nil:
			t.Errorf("Wildcard(%q, %q) = %+v, want none", w.dir, w.unit, *d)
		case w.line != 0 && (d == nil || d.Line != w.line || d.Kind != "unpromised"):
			t.Errorf("Wildcard(%q, %q) = %+v, want the wildcard on line %d", w.dir, w.unit, d, w.line)
		}
	}
	if d := ix.Exact("a.T.M: 1 -> 0"); d == nil || d.Kind != "unpromised" || d.Line != lineOf(t, text, "a.T.M") {
		t.Errorf("Exact(a.T.M: 1 -> 0) = %+v, want the unpromised id", d)
	}
	if d := ix.Exact("a.F: *"); d != nil {
		t.Errorf("Exact(a.F: *) = %+v, but a wildcard isn't a mutant id", *d)
	}
	if got := len(ix.All()); got != 3 {
		t.Errorf("All() has %d declarations, want the block's 3 lines", got)
	}
}

// Contract: declare/D2
func TestAWildcardCoversOnlyTheUnheldMutantsOfItsUnit(t *testing.T) {
	code, rep := flinch(t, md(`## Add's insides are open

'''unpromised
calc.Add: *
'''
`))
	if code != 0 {
		t.Fatalf("flinch exited %d, want 0 with Add's unheld mutants declared\n%+v", code, rep)
	}
	problemsAt(t, "Contract errors", rep.ContractErrors)
	problemsAt(t, "broken declarations", rep.BrokenDeclarations)
	closures := 0
	for _, m := range rep.Mutants {
		switch {
		case m.Unit == "Add.func1":
			closures++
			if m.Status != "declared" || m.Declaration == nil || !m.Declaration.Wildcard {
				t.Errorf("%s is %s with declaration %+v, want declared by the wildcard", m.ID, m.Status, m.Declaration)
			}
		case m.Status != "killed" || m.Declaration != nil:
			t.Errorf("%s is %s with declaration %+v; a wildcard leaves a killed mutant killed", m.ID, m.Status, m.Declaration)
		}
	}
	if closures == 0 {
		t.Error("the run had no mutant in Add's closure")
	}
}

// Contract: declare/D3
func TestADeclarationTakesItsReasonFromTheHeadingAbove(t *testing.T) {
	text := md(`'''unpromised
a.G: x - y -> x + y
a.F: x > 0 -> x >= 0
'''

# Notes

## Logging is open

Nothing reads the log text.

'''unpromised
a.F.func1: x + 1 -> x - 1
'''
`)
	c, ps := check(t, map[string]string{"p": text})
	// The block without a heading is one error, however many lines it holds.
	expectAt(t, ps, "contract/p/mutants.md", 1)

	d := declare.NewIndex(c).Exact("a.F.func1: x + 1 -> x - 1")
	if d == nil {
		t.Fatal("the declaration under the heading wasn't indexed")
	}
	if !strings.Contains(d.Reason, "Logging is open") || !strings.Contains(d.Reason, "Nothing reads the log text.") || strings.Contains(d.Reason, "Notes") {
		t.Errorf("reason = %q, want the nearest heading and the prose under it", d.Reason)
	}
}

// Contract: declare/D4
func TestADeclarationBelongsWithThePrimitiveCoveringItsUnit(t *testing.T) {
	p := md(`## Open

'''unpromised
a.G: x - y -> x + y
b.B: 1 -> 2
c.C: *
a.F.func1: x + 1 -> x - 1
'''
`)
	q := md(`## Open

'''unpromised
b.B: 1 -> 0
'''
`)
	_, ps := check(t, map[string]string{"p": p, "q": q})
	expectAt(t, ps, "contract/p/mutants.md", lineOf(t, p, "b.B"), lineOf(t, p, "c.C"))
	if len(ps) > 0 && !strings.Contains(ps[0].Message, "contract/q/mutants.md") {
		t.Errorf("%v doesn't name contract/q/mutants.md, where b.B's declarations belong", ps[0])
	}
}

// Contract: declare/D5
func TestADeclaredMutantStillRunsAndAKillBreaksItsDeclaration(t *testing.T) {
	text := closureBlock + md(`
## The sum is open

'''unpromised
`+sumID+`
'''
`)
	code, rep := flinch(t, text)
	if code != 1 {
		t.Errorf("flinch exited %d, want 1 for a declaration a Contract test kills", code)
	}
	problemsAt(t, "Contract errors", rep.ContractErrors)
	problemsAt(t, "broken declarations", rep.BrokenDeclarations, lineOf(t, text, sumID))
	found := false
	for _, m := range rep.Mutants {
		if m.ID != sumID {
			continue
		}
		found = true
		if m.Status != "killed" || len(m.KilledBy) == 0 || m.KilledBy[0].Test != "calc/TestAddSums" {
			t.Errorf("%s is %s, killed by %+v; want it run and killed by TestAddSums", m.ID, m.Status, m.KilledBy)
		}
	}
	if !found {
		t.Errorf("the report has no mutant %s", sumID)
	}
}

// Contract: declare/D6
func TestADeclarationThatMatchesNothingIsStale(t *testing.T) {
	text := closureBlock + md(`
## Sub's old rounding

'''unpromised
calc.Sub: a * b -> a / b
calc.Sub: *
'''
`)
	code, rep := flinch(t, text)
	if code != 1 {
		t.Errorf("a full run exited %d, want 1 for stale declarations", code)
	}
	problemsAt(t, "Contract errors", rep.ContractErrors)
	problemsAt(t, "broken declarations", rep.BrokenDeclarations,
		lineOf(t, text, "calc.Sub: a * b"), lineOf(t, text, "calc.Sub: *"))

	// A run that leaves out most operators mutates no unit whole, so it can't call either stale.
	code, rep = flinch(t, text, "--operators", "boundary")
	if code != 0 {
		t.Errorf("a run of one operator exited %d, want 0\n%+v", code, rep)
	}
	problemsAt(t, "broken declarations in a scoped run", rep.BrokenDeclarations)
}

// Contract: declare/D7
func TestADeclarationOnAUnitThatsGoneIsAnErrorInEveryRun(t *testing.T) {
	text := md(`## Gone

'''unpromised
a.Gone: *
a.T.Gone: 1 -> 0
a.F.func9: x -> y
..Root: *
gone.F: a -> b
a.G: x - y -> x + y
a.F.func1: x + 1 -> x - 1
a.init.0: *
'''
`)
	_, ps := check(t, map[string]string{"p": text})
	var lines []int
	for _, prefix := range []string{"a.Gone", "a.T.Gone", "a.F.func9", "..Root", "gone.F"} {
		lines = append(lines, lineOf(t, text, prefix))
	}
	expectAt(t, ps, "contract/p/mutants.md", lines...)

	gone := closureBlock + md(`
## Mul's overflow

'''unpromised
calc.Mul: a * b -> a / b
'''
`)
	for _, args := range [][]string{{"contract"}, {"run"}, {"run", "--only", "calc"}, {"run", "--operators", "boundary"}} {
		code, rep := flinch(t, gone, args...)
		if code != 1 {
			t.Errorf("flinch %q exited %d, want 1", args, code)
		}
		problemsAt(t, "Contract errors from flinch "+strings.Join(args, " "), rep.ContractErrors, lineOf(t, gone, "calc.Mul"))
	}
}

// Contract: declare/D8
func TestEachLineDeclaresOneIdOrWildcardOnce(t *testing.T) {
	text := md(`## Open

'''unpromised
a.G: x - y -> x + y
not an id
a.F: x > 0 ->
a.F: x > 0 -> x >= 0 #1
a.F: *
a.G:  x  -  y  ->  x + y
'''

## The same value

'''equivalent
a.T.M: 1 -> 0
'''

## Open again

'''unpromised
a.F: *
a.T.M: 1 -> 0
a.F.func1: x + 1 -> x - 1
'''
`)
	_, ps := check(t, map[string]string{"p": text})
	expectAt(t, ps, "contract/p/mutants.md",
		nth(t, text, "not an id", 1),
		nth(t, text, "a.F: x > 0 ->", 1),
		nth(t, text, "a.F: x > 0 -> x >= 0 #1", 1),
		nth(t, text, "a.G:  x  -  y  ->  x + y", 1), // the same id as the line above, spaced out
		nth(t, text, "a.F: *", 2),
		nth(t, text, "a.T.M: 1 -> 0", 2), // repeated in another block, of another kind
	)
}
