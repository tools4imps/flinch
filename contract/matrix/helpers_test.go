package matrix_test

import (
	"reflect"
	"testing"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
)

// The helpers below build matrix input by hand. A primitive's code sits in a package directory of
// the same name, and its Contract tests sit in contract/<primitive>.

func ref(prim, name string) model.TestRef { return model.TestRef{Primitive: prim, Name: name} }

// ctest makes a Contract test of prim at the given file and line, naming the given obligations.
func ctest(prim, name, file string, line int, obligations ...string) model.Test {
	return model.Test{
		TestRef: ref(prim, name), Kind: "Test", Dir: "contract/" + prim,
		ImportPath: "example.com/m/contract/" + prim, File: file, Line: line, Obligations: obligations,
	}
}

// mut makes a mutant of prim's code in unit, with a real id and hash. The change text tells
// mutants in the same unit apart.
func mut(prim, unit, change string, line int) model.Mutant {
	top := unit
	for i := range unit {
		if unit[i] == '.' {
			top = unit[:i]
			break
		}
	}
	id := mutantid.ID{Dir: prim, Unit: unit, Original: change, Replacement: change + "'", N: 1}
	return model.Mutant{
		ID: id.String(), Hash: id.Hash(), Primitive: prim, Dir: prim, ImportPath: "example.com/m/" + prim,
		File: prim + "/" + prim + ".go", Line: line, Col: 2, Unit: unit, Top: top,
		Original: change, Replace: change + "'", Operator: "arithmetic",
	}
}

// eraseOf makes the erase mutant of a top-level function.
func eraseOf(prim, fn string, line int) model.Mutant {
	m := mut(prim, fn, "{ ... }", line)
	m.Erase, m.Operator = true, "erase"
	return m
}

// killedBy is a complete row in which each test failed the given way.
func killedBy(kind model.KillKind, tests ...model.TestRef) model.Row {
	row := model.Row{Ran: tests, Complete: true}
	for _, t := range tests {
		row.Kills = append(row.Kills, model.Kill{Test: t, Kind: kind})
	}
	return row
}

// passedBy is a complete row in which every test passed.
func passedBy(tests ...model.TestRef) model.Row { return model.Row{Ran: tests, Complete: true} }

func exact(kind, prim string, m model.Mutant, line int) matrix.Declaration {
	return matrix.Declaration{
		Kind: kind, Primitive: prim, Path: "contract/" + prim + "/mutants.md", Line: line,
		Reason: "## Why\n\nBecause.", ID: m.ID,
	}
}

func wildcard(prim, dir, unit string, line int) matrix.Declaration {
	return matrix.Declaration{
		Kind: "unpromised", Primitive: prim, Path: "contract/" + prim + "/mutants.md", Line: line,
		Reason: "## Open\n\nOpen on purpose.", Wildcard: true, Dir: dir, Unit: unit,
	}
}

func mutantIn(t *testing.T, r *matrix.Result, m model.Mutant) matrix.Mutant {
	t.Helper()
	for _, x := range r.Mutants {
		if x.Hash == m.Hash {
			return x
		}
	}
	t.Fatalf("the result has no mutant %s", m.ID)
	return matrix.Mutant{}
}

func obligationIn(t *testing.T, r *matrix.Result, id string) matrix.Obligation {
	t.Helper()
	for _, o := range r.Obligations {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("the result has no obligation %s", id)
	return matrix.Obligation{}
}

func testIn(t *testing.T, r *matrix.Result, tr model.TestRef) matrix.Test {
	t.Helper()
	for _, x := range r.Tests {
		if x.TestRef == tr {
			return x
		}
	}
	t.Fatalf("the result has no test %s", tr)
	return matrix.Test{}
}

func hashes(ms ...model.Mutant) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Hash)
	}
	return out
}

// sorted returns s in order, so a test can state an expected list without sorting hashes by hand.
func sorted(s []string) []string {
	out := append([]string(nil), s...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sameStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}
