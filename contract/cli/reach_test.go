package cli_test

import (
	"slices"
	"testing"
)

// Each mutant in the full run was run by the tests that reached it. An ordinary mutant's tests ran
// its line, an erase mutant's ran its function, a case header's ran the switch, and an initializer's
// or init's are every test linking calc. Nothing reaches Unused.
//
// Contract: cli/L7
func TestMutantsAreReachedByTheTestsThatRanThem(t *testing.T) {
	_, rep := fullRun(t)
	linking := []string{"calc/TestAdd", "calc/TestMode", "calc/TestReady", "calc/TestSign"}
	for id, want := range map[string][]string{
		baseArith:   linking,
		initErase:   linking,
		initBool:    linking,
		addArith:    {"calc/TestAdd"},
		readyErase:  {"calc/TestReady"},
		signErase:   {"calc/TestSign"},
		signBound:   {"calc/TestSign"},
		signEqual:   {"calc/TestSign"},
		doubleArith: {"weak/TestDouble"},
		unusedErase: nil,
		unusedArith: nil,
	} {
		m := rep.mutant(t, id)
		if !slices.Equal(m.RanBy, want) {
			t.Errorf("%s ran by %q, want %q", id, m.RanBy, want)
		}
		if len(want) > 0 && m.Status != "killed" {
			t.Errorf("%s is %s, want killed", id, m.Status)
		}
	}
}

// Erase mutants run first. Positive's lived, so its other mutant was skipped and never built, while
// Double, in the same package, had both its mutants run.
//
// Contract: cli/L8
func TestEraseMutantsRunFirst(t *testing.T) {
	_, rep := fullRun(t)
	for id, want := range map[string]string{
		positiveErase: "erased", positiveBound: "skipped", doubleErase: "killed", doubleArith: "killed",
	} {
		if m := rep.mutant(t, id); m.Status != want {
			t.Errorf("%s is %s, want %s", id, m.Status, want)
		}
	}
	if m := rep.mutant(t, positiveBound); len(m.KilledBy) != 0 {
		t.Errorf("a skipped mutant never runs, and %s was killed by %v", m.ID, m.KilledBy)
	}
}
