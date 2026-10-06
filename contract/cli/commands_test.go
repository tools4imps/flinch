package cli_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/mutate"
	"github.com/tools4imps/flinch/internal/version"
)

// A bare flinch runs the whole check over every primitive: each mutant of covered code ends with a
// status, and weak's erased function and unreached one fail the run.
//
// Contract: cli/L1
func TestRunChecksTheWholeContract(t *testing.T) {
	r, rep := fullRun(t)
	if r.code != 1 || rep.Exit != 1 || rep.Summary.Scoped {
		t.Fatalf("a full run should fail on weak:\n%s", r)
	}
	want := map[string]string{
		baseArith: "killed", initErase: "killed", initBool: "killed", readyErase: "killed",
		addErase: "killed", addArith: "killed", modeErase: "killed",
		signErase: "killed", signBound: "killed", signEqual: "killed",
		positiveErase: "erased", positiveBound: "skipped",
		doubleErase: "killed", doubleArith: "killed",
		unusedErase: "unreached", unusedArith: "unreached",
	}
	if got := rep.statuses(); !maps.Equal(got, want) {
		t.Errorf("statuses =\n%v\nwant\n%v", got, want)
	}
}

// When every mutant is held or declared, flinch run passes. Declarations reach the verdict, whether
// they name one mutant or a whole unit.
//
// Contract: cli/L1
func TestRunPassesWhenTheContractHolds(t *testing.T) {
	mod(t)
	write(t, "contract/weak/mutants.md", "# weak\n\n"+
		"## Positive's answer is left open\n\n```unpromised\n"+positiveErase+"\n```\n\n"+
		"## Nothing calls Unused\n\n```unpromised\nweak.Unused: *\n```\n")
	if r := flinch("run", "--jobs", "2"); r.code != 0 {
		t.Fatalf("every mutant is held or declared, so the run should pass:\n%s", r)
	}
}

// flinch contract checks the Contract's files and stops there: it passes a sound Contract with no
// go command on PATH and a type error in covered code, which any build would trip on.
//
// Contract: cli/L1
func TestContractStopsAfterTheContractChecks(t *testing.T) {
	mod(t)
	write(t, "calc/broken.go", "package calc\n\nvar broken int = \"not an int\"\n")
	t.Setenv("PATH", t.TempDir())
	if r := flinch("contract"); r.code != 0 || !strings.Contains(r.stdout, "no Contract errors") {
		t.Fatalf("the Contract is sound:\n%s", r)
	}
}

// flinch operators lists the default operators in order, flinch version and --version print the
// version, and flinch help and -h print the usage text. Each exits 0.
//
// Contract: cli/L1
func TestCommandsPrintWhatTheirNamesSay(t *testing.T) {
	r := flinch("operators")
	if r.code != 0 {
		t.Fatalf("flinch operators failed:\n%s", r)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n") {
		name, about, _ := strings.Cut(line, " ")
		if strings.TrimSpace(about) == "" {
			t.Errorf("operator %s has no description", name)
		}
		names = append(names, name)
	}
	if !slices.Equal(names, mutate.Defaults) {
		t.Errorf("flinch operators lists %q, want %q", names, mutate.Defaults)
	}

	for _, args := range [][]string{{"version"}, {"--version"}} {
		if r := flinch(args...); r.code != 0 || r.stdout != "flinch "+version.Version+"\n" {
			t.Errorf("flinch %s should print the version:\n%s", args[0], r)
		}
	}
	for _, args := range [][]string{{"help"}, {"-h"}} {
		if r := flinch(args...); r.code != 0 || !strings.Contains(r.stdout, "flinch contract") || r.stderr != "" {
			t.Errorf("flinch %s should print the usage text:\n%s", args[0], r)
		}
	}
}
