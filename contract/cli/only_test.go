package cli_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
)

// --only mutates the named primitives' code and nothing else. It repeats and takes a comma list,
// and the run it narrows is scoped: weak's erased function and unreached one don't fail a run
// that mutates only calc.
//
// Contract: cli/L5
func TestOnlyMutatesTheNamedPrimitives(t *testing.T) {
	dir := mod(t)
	if got := ids(listed(t, flinch("--dry-run", "--only", "weak"))); !slices.Equal(got, sorted(weakMutants...)) {
		t.Errorf("--only weak lists\n%q", got)
	}
	all := sorted(append(calcMutants, weakMutants...)...)
	if got := ids(listed(t, flinch("--dry-run", "--only", "calc,weak"))); !slices.Equal(got, all) {
		t.Errorf("--only calc,weak lists\n%q", got)
	}
	if got := ids(listed(t, flinch("--dry-run", "--only", "calc", "--only", "weak"))); !slices.Equal(got, all) {
		t.Errorf("--only calc --only weak lists\n%q", got)
	}

	plan := prepare(t, dir, engine.RunOptions{Only: []string{"weak"}})
	units := set("weak.Double", "weak.Positive", "weak.Unused")
	if plan.Full || !maps.Equal(plan.Whole, set("weak")) || !maps.Equal(plan.Units, units) {
		t.Errorf("an --only weak plan is full %v, whole %v, units %v; want only weak", plan.Full, plan.Whole, plan.Units)
	}

	r := flinch("run", "--only", "calc", "--format", "json", "--jobs", "2")
	if r.code != 0 {
		t.Fatalf("calc holds, so a run over calc alone should pass:\n%s", r)
	}
	rep := parse(t, r)
	var got []string
	for _, m := range rep.Mutants {
		got = append(got, m.ID)
	}
	slices.Sort(got)
	if !rep.Summary.Scoped || !slices.Equal(got, sorted(calcMutants...)) {
		t.Errorf("scoped %v, mutants %q; want a scoped run over calc's mutants", rep.Summary.Scoped, got)
	}
}
