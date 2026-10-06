package cli_test

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// flinch contract prints the Contract's size, its errors sorted by path and line, and the packages
// outside it, as text or as JSON.
//
// Contract: cli/L6
func TestContractPrintsItsSizeErrorsAndOutside(t *testing.T) {
	mod(t)
	r := flinch("contract")
	if r.code != 0 {
		t.Fatalf("the Contract is sound:\n%s", r)
	}
	for _, want := range []string{"2 primitives, 3 obligations, 6 Contract tests", "no Contract errors", "Outside the Contract (1)", "\n  extra\n", "exit 0"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("flinch contract should print %q:\n%s", want, r)
		}
	}

	// The covers error in calc is found after the tag error in weak, and still prints first.
	edit(t, "contract/weak/README.md", "- **W1**", "- **W2** Nothing checks this.\n- **W1**")
	edit(t, "contract/calc/README.md", "```covers\ncalc\n", "```covers\ncalc\nnowhere\n")
	r = flinch("contract")
	if r.code != 1 {
		t.Fatalf("the Contract has two errors:\n%s", r)
	}
	calcAt, weakAt := strings.Index(r.stdout, "\n  contract/calc/README.md:10: "), strings.Index(r.stdout, "\n  contract/weak/README.md:5: ")
	if calcAt < 0 || weakAt < calcAt {
		t.Errorf("flinch contract should list calc's error and then weak's:\n%s", r)
	}
	for _, want := range []string{"2 primitives, 4 obligations, 6 Contract tests", "Contract errors (2)", "exit 1"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("flinch contract should print %q:\n%s", want, r)
		}
	}

	r = flinch("contract", "--format", "json")
	rep := parse(t, r)
	if r.code != 1 || rep.Exit != 1 || len(rep.ContractErrors) != 2 || !rep.Summary.Scoped || !slices.Equal(rep.Outside, []string{"extra"}) {
		t.Errorf("flinch contract --format json should report both errors, the package outside, and no full run:\n%s", r)
	}

	// With extra gone, nothing is outside the Contract, and the report says nothing about it.
	if err := os.RemoveAll("extra"); err != nil {
		t.Fatal(err)
	}
	if r := flinch("contract"); strings.Contains(r.stdout, "Outside") {
		t.Errorf("nothing is outside the Contract now:\n%s", r)
	}
}

// A Contract error stops a run before any mutant is generated. calc also gets a type error here,
// which would end the run with exit 2 if flinch went on to type-check it, and a Contract test that
// fails, which would end it with exit 2 if flinch ran the suite.
//
// Contract: cli/L6
func TestRunStopsAtContractErrors(t *testing.T) {
	mod(t)
	edit(t, "contract/calc/README.md", "- **C2**", "- **C3** Nothing checks this.\n- **C2**")
	write(t, "calc/broken.go", "package calc\n\nvar broken int = \"not an int\"\n")
	edit(t, "contract/calc/calc_test.go", "got != 5", "got != 6")

	r := flinch("run", "--format", "json", "--jobs", "2")
	if r.code != 1 {
		t.Fatalf("a Contract error should fail the run:\n%s", r)
	}
	rep := parse(t, r)
	if len(rep.ContractErrors) != 1 || rep.ContractErrors[0].Path != "contract/calc/README.md" || rep.ContractErrors[0].Line != 6 {
		t.Errorf("contract_errors = %+v, want calc/C3's line", rep.ContractErrors)
	}
	if len(rep.Mutants) != 0 || rep.Summary.Unviable != 0 || rep.Exit != 1 {
		t.Errorf("%d mutants, %d unviable, exit %d; want none generated and exit 1", len(rep.Mutants), rep.Summary.Unviable, rep.Exit)
	}

	r = flinch("--dry-run")
	if r.code != 1 || !strings.Contains(r.stdout, "contract/calc/README.md:6:") || strings.Contains(r.stdout, "none built") {
		t.Errorf("a dry run should report the Contract error and list no mutants:\n%s", r)
	}
}
