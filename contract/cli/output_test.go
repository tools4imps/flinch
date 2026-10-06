package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// A run's report records the toolchain it used, every obligation, the unviable count and the
// packages outside the Contract, and its progress goes to stderr.
//
// Contract: cli/L12
func TestARunsReportRecordsWhatItRan(t *testing.T) {
	r, rep := fullRun(t)
	goVersion, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Build.Go != strings.TrimSpace(string(goVersion)) || rep.Build.GOOS != runtime.GOOS || rep.Build.GOARCH != runtime.GOARCH {
		t.Errorf("build = %+v, want the go command's version, %s and %s", rep.Build, runtime.GOOS, runtime.GOARCH)
	}
	var obligations []string
	for _, o := range rep.Obligations {
		obligations = append(obligations, o.ID)
	}
	if !slices.Equal(obligations, []string{"calc/C1", "calc/C2", "weak/W1"}) {
		t.Errorf("obligations = %q, want every one in the Contract", obligations)
	}
	if rep.Summary.Unviable != 1 || !slices.Equal(rep.Outside, []string{"extra"}) {
		t.Errorf("%d unviable and outside %q, want Limit's mutant and extra", rep.Summary.Unviable, rep.Outside)
	}
	if r.stderr == "" {
		t.Error("a run should report its progress on stderr")
	}
}

// The report is text unless JSON is asked for, and --output sends it, or a dry run's list, to a
// file and leaves stdout empty.
//
// Contract: cli/L12
func TestTheReportGoesWhereItsAskedFor(t *testing.T) {
	mod(t)
	if r := flinch("contract"); r.code != 0 || r.stdout == "" || json.Valid([]byte(r.stdout)) {
		t.Errorf("the default report should be text:\n%s", r)
	}

	r := flinch("contract", "--format", "json", "--output", "contract.json")
	if r.code != 0 || r.stdout != "" {
		t.Fatalf("the report should go to the file:\n%s", r)
	}
	data, err := os.ReadFile("contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Errorf("contract.json isn't JSON:\n%s", data)
	}

	r = flinch("--dry-run", "--output", "plan.txt")
	if r.code != 0 || r.stdout != "" {
		t.Fatalf("the dry run should go to the file:\n%s", r)
	}
	data, err = os.ReadFile("plan.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), addArith) {
		t.Errorf("plan.txt should list %q:\n%s", addArith, data)
	}
}
