package cli_test

import (
	"slices"
	"testing"
)

// --operators keeps the mutants of the operators it names, as a list or repeated.
//
// Contract: cli/L9
func TestOperatorsKeepOnlyTheirMutants(t *testing.T) {
	mod(t)
	got := ids(listed(t, flinch("--dry-run", "--operators", "arithmetic")))
	if want := sorted(baseArith, addArith, doubleArith, unusedArith); !slices.Equal(got, want) {
		t.Errorf("--operators arithmetic lists\n%q\nwant\n%q", got, want)
	}
	got = ids(listed(t, flinch("--dry-run", "--operators", "boundary", "--operators", "bool,equality")))
	if want := sorted(initBool, positiveBound, signBound, signEqual); !slices.Equal(got, want) {
		t.Errorf("--operators boundary --operators bool,equality lists\n%q\nwant\n%q", got, want)
	}
}

// --since keeps the mutants on lines changed since the merge base, the erase mutant of any function
// with a changed line, and every mutant of a primitive whose Contract directory changed. --only
// still narrows what it keeps.
//
// Contract: cli/L9
func TestSinceKeepsChangedLinesAndChangedContracts(t *testing.T) {
	mod(t)
	commit(t)
	sinceEdits(t)
	got := ids(listed(t, flinch("--dry-run", "--since", "HEAD")))
	want := sorted(append([]string{addErase, addSwapped, readyErase}, weakMutants...)...)
	if !slices.Equal(got, want) {
		t.Errorf("--since HEAD lists\n%q\nwant\n%q", got, want)
	}
	got = ids(listed(t, flinch("--dry-run", "--since", "HEAD", "--only", "calc")))
	if want := sorted(addErase, addSwapped, readyErase); !slices.Equal(got, want) {
		t.Errorf("--since HEAD --only calc lists\n%q\nwant\n%q", got, want)
	}
}

// addSwapped is Add's arithmetic mutant once sinceEdits has swapped its operands.
const addSwapped = "calc.Add: b + a -> b - a"

// sinceEdits changes the committed fixture without moving a line. Add's second line changes, which
// keeps its mutant there and its erase mutant on the line above. Only Ready's closing brace
// changes, which keeps its erase mutant alone. weak's Contract changes, which keeps all of weak.
func sinceEdits(t *testing.T) {
	t.Helper()
	edit(t, "calc/calc.go", "sum := a + b", "sum := b + a")
	edit(t, "calc/calc.go", "\treturn ready\n}\n", "\treturn ready\n} // Ready ends here.\n")
	edit(t, "contract/weak/README.md", "loosely.", "loosely, and this line is new.")
}
