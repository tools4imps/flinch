package cli_test

import (
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
