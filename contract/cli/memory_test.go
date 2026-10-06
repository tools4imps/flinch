package cli_test

import (
	"os"
	"testing"
)

// growFile is a function whose step mutant, i++ to i--, appends without end.
const growFile = `package calc

// Grow returns n copies of s.
func Grow(s string, n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, s)
	}
	return out
}
`

const growTest = `package calc_test

import (
	"testing"

	"example.com/fx/calc"
)

// Contract: calc/C1
func TestGrow(t *testing.T) {
	if got := len(calc.Grow("a", 3)); got != 3 {
		t.Fatalf("len(Grow(a, 3)) = %d", got)
	}
}
`

// Contract: cli/L13
func TestAMemoryLimitStopsARunawayTestProcess(t *testing.T) {
	mod(t)
	write(t, "calc/grow.go", growFile)
	write(t, "contract/calc/grow_test.go", growTest)
	r := flinch("run", "--only", "calc", "--operators", "step", "--memory-limit", "64", "--format", "json", "--jobs", "1")
	m := parse(t, r).mutant(t, "calc.Grow: i++ -> i--")
	if m.Status != "killed" || len(m.KilledBy) != 1 || m.KilledBy[0].Kind != "panic" {
		t.Fatalf("the runaway mutant is %s, killed by %+v; want killed by TestGrow as a crash\n%s", m.Status, m.KilledBy, r)
	}
}

// Contract: cli/L13
func TestAMemoryLimitOfZeroStopsNothing(t *testing.T) {
	mod(t)
	r := flinch("run", "--only", "calc", "--operators", "arithmetic", "--memory-limit", "0", "--format", "json", "--jobs", "1")
	m := parse(t, r).mutant(t, "calc.Add: a + b -> a - b")
	if m.Status != "killed" || len(m.KilledBy) != 1 || m.KilledBy[0].Kind != "assertion" {
		t.Fatalf("Add's mutant is %s, killed by %+v; want killed by an assertion\n%s", m.Status, m.KilledBy, r)
	}
}

// Contract: cli/L14
func TestARunInsideAnotherRunBuildsInTheOuterRunsCache(t *testing.T) {
	mod(t)
	outer, own := t.TempDir(), t.TempDir()
	t.Setenv("FLINCH_GOCACHE", outer)
	t.Setenv("GOCACHE", own)
	r := flinch("run", "--only", "calc", "--operators", "arithmetic", "--format", "json", "--jobs", "1")
	if r.code == 2 {
		t.Fatalf("the run couldn't decide:\n%s", r)
	}
	if entries, _ := os.ReadDir(outer); len(entries) == 0 {
		t.Error("nothing was built in the outer run's cache")
	}
	if entries, _ := os.ReadDir(own); len(entries) != 0 {
		t.Errorf("the run built in its own GOCACHE too: %d entries", len(entries))
	}
}
