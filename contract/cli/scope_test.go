package cli_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
	"github.com/tools4imps/flinch/internal/mutate"
)

// With no flags, a plan is full and mutates every primitive and every unit whole. The plan is of
// the module it's given, wherever the working directory is.
//
// Contract: cli/L10
func TestAFullPlanMutatesEverythingWhole(t *testing.T) {
	plan := prepare(t, modAt(t), engine.RunOptions{})
	if !plan.Full || !maps.Equal(plan.Whole, set("calc", "weak")) {
		t.Errorf("full %v, whole %v; want a full plan over calc and weak", plan.Full, plan.Whole)
	}
	units := set("calc.Add", "calc.Base", "calc.Mode", "calc.Ready", "calc.Sign", "calc.init.0", "weak.Double", "weak.Positive", "weak.Unused")
	if !maps.Equal(plan.Units, units) {
		t.Errorf("units = %v, want %v", plan.Units, units)
	}
	var got []string
	for _, m := range plan.Mutants {
		got = append(got, m.ID)
	}
	slices.Sort(got)
	if want := sorted(append(calcMutants, weakMutants...)...); plan.Unviable != 1 || !slices.Equal(got, want) {
		t.Errorf("%d unviable and mutants\n%q\nwant 1 unviable and\n%q", plan.Unviable, got, want)
	}
}

// Fewer operators make a plan scoped and leave nothing whole, while naming all ten is a full plan.
//
// Contract: cli/L10
func TestFewerOperatorsMutateNothingWhole(t *testing.T) {
	dir := modAt(t)
	plan := prepare(t, dir, engine.RunOptions{Operators: []string{"arithmetic"}})
	if plan.Full || len(plan.Units) != 0 || !maps.Equal(plan.Whole, map[string]bool{"calc": false, "weak": false}) {
		t.Errorf("an --operators plan is full %v, whole %v, units %v; want it scoped", plan.Full, plan.Whole, plan.Units)
	}
	plan = prepare(t, dir, engine.RunOptions{Operators: mutate.Defaults})
	if !plan.Full || len(plan.Mutants) != len(calcMutants)+len(weakMutants) {
		t.Errorf("naming every default operator is a full plan, and got full %v with %d mutants", plan.Full, len(plan.Mutants))
	}
}

// A --since plan is scoped. It mutates whole the primitives whose Contract changed, within any
// --only, and the units it keeps every mutant of.
//
// Contract: cli/L10
func TestASincePlanMutatesWholeWhatChanged(t *testing.T) {
	dir := mod(t)
	commit(t)
	sinceEdits(t)
	plan := prepare(t, dir, engine.RunOptions{Since: "HEAD"})
	units := set("calc.Add", "calc.Ready", "weak.Double", "weak.Positive", "weak.Unused")
	if plan.Full || !maps.Equal(plan.Whole, set("weak")) || !maps.Equal(plan.Units, units) {
		t.Errorf("a --since plan is full %v, whole %v, units %v; want weak whole and units %v", plan.Full, plan.Whole, plan.Units, units)
	}
	plan = prepare(t, dir, engine.RunOptions{Since: "HEAD", Only: []string{"calc"}})
	if len(plan.Whole) != 0 {
		t.Errorf("--since HEAD --only calc mutates %v whole, want nothing whole", plan.Whole)
	}
}

// A plan with nothing to mutate is still full or scoped by the same rule. Here both primitives cover
// only extra, which holds a constant and no unit, so no package needs type-checking.
//
// Contract: cli/L10
func TestAPlanWithNothingToMutateIsScopedTheSameWay(t *testing.T) {
	dir := mod(t)
	edit(t, "contract/calc/README.md", "```covers\ncalc\n", "```covers\nextra\n")
	edit(t, "contract/weak/README.md", "```covers\nweak\n", "```covers\nextra\n")
	if plan := prepare(t, dir, engine.RunOptions{}); !plan.Full || len(plan.Mutants) != 0 {
		t.Errorf("with no flags, the plan is full %v with %d mutants; want a full plan with none", plan.Full, len(plan.Mutants))
	}
	if plan := prepare(t, dir, engine.RunOptions{Only: []string{"calc"}}); plan.Full {
		t.Error("an --only plan with nothing to mutate should still be scoped")
	}
	if plan := prepare(t, dir, engine.RunOptions{Operators: []string{"erase"}}); plan.Full {
		t.Error("an --operators plan with nothing to mutate should still be scoped")
	}
}
