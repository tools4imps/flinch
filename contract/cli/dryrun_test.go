package cli_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A dry run lists the same mutants, with the same ids, hashes and places, that a run reports.
//
// Contract: cli/L3
func TestDryRunListsEveryMutantARunWouldBuild(t *testing.T) {
	_, rep := fullRun(t)
	var want []listing
	for _, m := range rep.Mutants {
		want = append(want, listing{fmt.Sprintf("%s:%d", m.File, m.Line), m.Hash, m.ID})
	}
	mod(t)
	got := listed(t, flinch("--dry-run"))
	byID := func(a, b listing) int { return strings.Compare(a.id, b.id) }
	slices.SortFunc(got, byID)
	slices.SortFunc(want, byID)
	if !slices.Equal(got, want) {
		t.Errorf("the dry run lists\n%q\nand the run reports\n%q", got, want)
	}
}

// A dry run builds and runs nothing, so a Contract test that fails on clean code, which would leave
// a run undecided, doesn't stop it.
//
// Contract: cli/L3
func TestDryRunBuildsNothing(t *testing.T) {
	mod(t)
	edit(t, "contract/calc/calc_test.go", "got != 5", "got != 6")
	r := flinch("--dry-run")
	if got := ids(listed(t, r)); !slices.Equal(got, sorted(append(calcMutants, weakMutants...)...)) {
		t.Errorf("the dry run lists\n%q", got)
	}
	if r.stderr != "" {
		t.Errorf("a dry run runs no tests, so it has no progress to report:\n%s", r)
	}
}
