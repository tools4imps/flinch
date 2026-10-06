package mutate_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutate"
)

// Contract: mutate/M6
func TestTheSameSourceGivesTheSameMutantsInTheSameOrder(t *testing.T) {
	gen := func(root string, ops []string) []model.Mutant {
		t.Helper()
		l, pkgs := load(t, root, fixturePaths...)
		var all []model.Mutant
		for _, ip := range fixturePaths {
			ms, _ := mutate.Generate(l, pkgs[ip], mutableAll(pkgs), ownAll, ops)
			all = append(all, ms...)
		}
		// A second pass with the same loader sees what the first did.
		for _, ip := range fixturePaths {
			again, _ := mutate.Generate(l, pkgs[ip], mutableAll(pkgs), ownAll, ops)
			var first []model.Mutant
			for _, m := range all {
				if m.ImportPath == ip {
					first = append(first, m)
				}
			}
			if !slices.Equal(again, first) {
				t.Errorf("%s: a second Generate with the same loader gave different mutants", ip)
			}
		}
		return all
	}
	first := gen(fixture, nil)
	if len(first) < 100 {
		t.Fatalf("only %d mutants in the fixture", len(first))
	}
	// The module checked out somewhere else, and the operators named in another order.
	elsewhere := copyFixture(t)
	reversed := slices.Clone(defaults)
	slices.Reverse(reversed)
	second := gen(elsewhere, reversed)
	if !slices.Equal(first, second) {
		sameList(t, "the copy", ids(second), ids(first))
		t.Error("the same module at another path, with the operators in another order, gave different mutants")
	}
}

// Contract: mutate/M6
func TestMutantsComeInFileAndPositionOrder(t *testing.T) {
	for _, ip := range fixturePaths {
		ms, _ := generate(t, ip, nil)
		for i := 1; i < len(ms); i++ {
			a, b := ms[i-1], ms[i]
			if a.File > b.File || a.File == b.File && (a.Start > b.Start || a.Start == b.Start && !a.Erase) {
				t.Errorf("%s at %s[%d] comes before %s at %s[%d]", a.ID, a.File, a.Start, b.ID, b.File, b.Start)
			}
		}
	}

	// A body written on one line puts its first call where the erase mutant goes.
	dir := copyFixture(t)
	tight := "package ops\n\nfunc Tight(n *int) {Now(n)}\n"
	if err := os.WriteFile(filepath.Join(dir, "ops", "tight.go"), []byte(tight), 0o644); err != nil {
		t.Fatal(err)
	}
	l, pkgs := load(t, dir, opsPath)
	ms, _ := mutate.Generate(l, pkgs[opsPath], map[string]bool{"ops/tight.go": true}, ownAll, nil)
	want := []string{"ops.Tight: { ... } -> { return }", "ops.Tight: Now(n) -> (removed)"}
	if got := ids(ms); !slices.Equal(got, want) {
		t.Fatalf("tight.go got %q, want %q", got, want)
	}
	if ms[0].Start != ms[1].Start {
		t.Errorf("the erase mutant starts at %d and the removed call at %d, so the test checks no tie", ms[0].Start, ms[1].Start)
	}
}
