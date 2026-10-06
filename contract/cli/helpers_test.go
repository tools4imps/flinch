package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/engine"
)

// testdata is absolute, so a test can still find its fixtures after it changes directory.
var testdata, _ = filepath.Abs("testdata")

// mod copies the fixture module in testdata/mod to a fresh directory and makes that the working
// directory, because flinch checks the module it starts in. A test that calls it can't run in
// parallel with another.
//
// The fixture has two primitives. calc holds all its code: a package-level variable, an init
// function, Add, Sign, whose comparisons sit in case headers, and Mode, which has one file per
// build tag. weak checks nothing Positive returns
// and never calls Unused. A package named extra sits outside the Contract.
func mod(t *testing.T) string {
	t.Helper()
	dir := modAt(t)
	t.Chdir(dir)
	return dir
}

// modAt copies the fixture module to a fresh directory and leaves the working directory alone.
func modAt(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join(testdata, "mod"))); err != nil {
		t.Fatal(err)
	}
	return dir
}

// write puts a file in the working directory, replacing any file already there.
func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// edit replaces the one place old appears in a file in the working directory.
func edit(t *testing.T, name, old, new string) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), old) != 1 {
		t.Fatalf("%s should hold %q exactly once", name, old)
	}
	write(t, name, strings.Replace(string(data), old, new, 1))
}

// git runs git in the working directory with no user or system config, so nothing set up on this
// machine, such as commit signing, gets in the way.
func git(t *testing.T, args ...string) {
	t.Helper()
	pinned := []string{"-c", "user.name=flinch", "-c", "user.email=flinch@example.com", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(pinned, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// commit makes the working directory a git repository with everything in it committed.
func commit(t *testing.T) {
	t.Helper()
	git(t, "init", "-q")
	git(t, "add", "-A")
	git(t, "commit", "-q", "-m", "base")
}

// result is what one flinch command printed and how it exited.
type result struct {
	code           int
	stdout, stderr string
}

func (r result) String() string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
}

// flinch runs the command line in-process, in the working directory.
func flinch(args ...string) result {
	var stdout, stderr bytes.Buffer
	code := cli.Main(args, strings.NewReader(""), &stdout, &stderr)
	return result{code, stdout.String(), stderr.String()}
}

// full is the fixture's full run. Several tests read it, so a test process makes it once.
var full struct {
	once sync.Once
	r    result
}

// fullRun runs a bare flinch with a JSON report over a fresh copy of the fixture, the first time a
// test in this process asks, and returns what it printed and the report.
func fullRun(t *testing.T) (result, report) {
	t.Helper()
	full.once.Do(func() {
		mod(t)
		full.r = flinch("--format", "json", "--jobs", "2")
	})
	return full.r, parse(t, full.r)
}

// mutant finds a mutant in a report by id.
func (rep report) mutant(t *testing.T, id string) reportMutant {
	t.Helper()
	for _, m := range rep.Mutants {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("the report has no mutant %q", id)
	return reportMutant{}
}

// report is the part of the JSON report these tests read.
type report struct {
	Summary struct {
		Scoped   bool   `json:"scoped"`
		Since    string `json:"since"`
		Unviable int    `json:"unviable"`
	} `json:"summary"`
	Build struct {
		Go     string   `json:"go"`
		GOOS   string   `json:"goos"`
		GOARCH string   `json:"goarch"`
		Tags   []string `json:"tags"`
	} `json:"build"`
	ContractErrors []struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Message string `json:"message"`
	} `json:"contract_errors"`
	Mutants     []reportMutant `json:"mutants"`
	Obligations []struct {
		ID string `json:"id"`
	} `json:"obligations"`
	Outside   []string `json:"outside"`
	Undecided string   `json:"undecided"`
	Exit      int      `json:"exit"`
}

// reportMutant is one mutant in a JSON report.
type reportMutant struct {
	ID        string   `json:"id"`
	Hash      string   `json:"hash"`
	Primitive string   `json:"primitive"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
	Status    string   `json:"status"`
	RanBy     []string `json:"ran_by"`
	KilledBy  []struct {
		Test string `json:"test"`
		Kind string `json:"kind"`
	} `json:"killed_by"`
}

// parse reads the JSON report a command printed.
func parse(t *testing.T, r result) report {
	t.Helper()
	var rep report
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("stdout isn't a JSON report: %v\n%s", err, r)
	}
	return rep
}

// statuses maps each mutant's id to its status.
func (rep report) statuses() map[string]string {
	out := map[string]string{}
	for _, m := range rep.Mutants {
		out[m.ID] = m.Status
	}
	return out
}

// A listing is one line of a dry run: where the mutant is, its hash and its id.
type listing struct {
	where, hash, id string
}

// listed reads a dry run's output. Each mutant gets a line, and a count closes the list.
func listed(t *testing.T, r result) []listing {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("the dry run failed:\n%s", r)
	}
	lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
	var out []listing
	for _, line := range lines[:len(lines)-1] {
		parts := strings.SplitN(line, "  ", 3)
		if len(parts) != 3 {
			t.Fatalf("a dry run line should read file:line, hash and id, and this one reads %q", line)
		}
		out = append(out, listing{parts[0], parts[1], parts[2]})
	}
	if want := fmt.Sprintf("%d mutants, none built", len(out)); lines[len(lines)-1] != want {
		t.Errorf("the dry run ends %q, want %q", lines[len(lines)-1], want)
	}
	return out
}

// ids is the sorted ids of a dry run's listings.
func ids(ls []listing) []string {
	var out []string
	for _, l := range ls {
		out = append(out, l.id)
	}
	slices.Sort(out)
	return out
}

// set is a map with every key true, the shape of a plan's Whole and Units.
func set(keys ...string) map[string]bool {
	out := map[string]bool{}
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// sorted returns a sorted copy.
func sorted(s ...string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// prepare plans a run over the module in dir without running anything.
func prepare(t *testing.T, dir string, o engine.RunOptions) *engine.Plan {
	t.Helper()
	o.Dir = dir
	plan, err := engine.Prepare(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// The fixture's mutants, by id.
const (
	baseArith     = "calc.Base: 40 + 2 -> 40 - 2"
	initErase     = "calc.init.0: { ... } -> { return }"
	initBool      = "calc.init.0: true -> false"
	readyErase    = "calc.Ready: { ... } -> { return *new(bool) }"
	addErase      = "calc.Add: { ... } -> { return *new(int) }"
	addArith      = "calc.Add: a + b -> a - b"
	modeErase     = "calc.Mode: { ... } -> { return *new(string) }"
	signErase     = "calc.Sign: { ... } -> { return *new(string) }"
	signBound     = "calc.Sign: n < 0 -> n <= 0"
	signEqual     = "calc.Sign: n == 0 -> n != 0"
	positiveErase = "weak.Positive: { ... } -> { return *new(bool) }"
	positiveBound = "weak.Positive: n > 0 -> n >= 0"
	doubleErase   = "weak.Double: { ... } -> { return *new(int) }"
	doubleArith   = "weak.Double: n * 2 -> n / 2"
	unusedErase   = "weak.Unused: { ... } -> { return *new(int) }"
	unusedArith   = "weak.Unused: n + 1 -> n - 1"
)

// calcMutants and weakMutants are every mutant of each primitive under the default operators.
var (
	calcMutants = []string{baseArith, initErase, initBool, readyErase, addErase, addArith, modeErase, signErase, signBound, signEqual}
	weakMutants = []string{positiveErase, positiveBound, doubleErase, doubleArith, unusedErase, unusedArith}
)
