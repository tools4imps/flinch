package declare_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/covers"
	"github.com/tools4imps/flinch/internal/declare"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

// md lets a mutants.md sit in a Go raw string by writing its fences as ”' instead of backticks.
func md(s string) string { return strings.ReplaceAll(s, "'''", "```") }

// lineOf returns the number of the first line of text that starts with prefix.
func lineOf(t *testing.T, text, prefix string) int {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return i + 1
		}
	}
	t.Fatalf("no line starts with %q", prefix)
	return 0
}

// testdata is the absolute path of this directory's fixtures, so a test that has changed directory
// can still copy one.
var testdata, _ = filepath.Abs("testdata")

// nth returns the number of the nth line of text that reads exactly line, counting from 1.
func nth(t *testing.T, text, line string, n int) int {
	t.Helper()
	seen := 0
	for i, l := range strings.Split(text, "\n") {
		if l == line {
			if seen++; seen == n {
				return i + 1
			}
		}
	}
	t.Fatalf("text has fewer than %d lines reading %q", n, line)
	return 0
}

// copyFixture copies the module in testdata/<name> to a new temporary directory and returns its root.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(testdata, name))); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeFile writes text to a slash-separated path under root.
func writeFile(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// check copies the static fixture, gives each named primitive the mutants.md text, and returns the
// loaded Contract and what declare.Check reports, sorted.
func check(t *testing.T, mutants map[string]string) (*contract.Contract, []problem.Problem) {
	t.Helper()
	root := copyFixture(t, "static")
	for name, text := range mutants {
		writeFile(t, root, "contract/"+name+"/mutants.md", text)
	}
	m, err := source.FindModule(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := source.Packages(m, source.Config{})
	if err != nil {
		t.Fatal(err)
	}
	parsed := map[string]*source.Parsed{}
	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}
		if parsed[p.Dir], err = source.Parse(m, p); err != nil {
			t.Fatal(err)
		}
	}
	c, ps, err := contract.Load(m.Root, "")
	if err != nil || len(ps) > 0 {
		t.Fatalf("loading the fixture's Contract: %v %v", ps, err)
	}
	own, ps := covers.Resolve(c, pkgs, parsed)
	if len(ps) > 0 {
		t.Fatalf("resolving the fixture's covers blocks: %v", ps)
	}
	ps = declare.Check(c, own, parsed)
	problem.Sort(ps)
	return c, ps
}

// at lists each problem's path and line.
func at(ps []problem.Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, fmt.Sprintf("%s:%d", p.Path, p.Line))
	}
	return out
}

// expectAt checks that the problems sit at exactly these lines of one file, in order.
func expectAt(t *testing.T, ps []problem.Problem, path string, lines ...int) {
	t.Helper()
	var want []string
	for _, l := range lines {
		want = append(want, fmt.Sprintf("%s:%d", path, l))
	}
	if got := at(ps); !slices.Equal(got, want) {
		t.Errorf("problems at %q, want %q\n%v", got, want, ps)
	}
}

// report is the part of flinch's JSON report these tests read.
type report struct {
	ContractErrors     []jsonProblem `json:"contract_errors"`
	BrokenDeclarations []jsonProblem `json:"broken_declarations"`
	Mutants            []struct {
		ID       string `json:"id"`
		Unit     string `json:"unit"`
		Status   string `json:"status"`
		KilledBy []struct {
			Test string `json:"test"`
		} `json:"killed_by"`
		Declaration *struct {
			Kind     string `json:"kind"`
			Line     int    `json:"line"`
			Wildcard bool   `json:"wildcard"`
		} `json:"declaration"`
	} `json:"mutants"`
	Exit int `json:"exit"`
}

type jsonProblem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

func (p jsonProblem) String() string { return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message) }

// mutantsPath is where the run fixture's one primitive keeps its declarations.
const mutantsPath = "contract/calc/mutants.md"

// flinch copies the run fixture, gives its calc primitive the mutants.md text, runs flinch there
// with the given arguments and a JSON report, and returns the exit code and the report.
func flinch(t *testing.T, mutants string, args ...string) (int, report) {
	t.Helper()
	root := copyFixture(t, "run")
	writeFile(t, root, mutantsPath, mutants)
	t.Chdir(root)
	var out, errs bytes.Buffer
	args = append(args, "--format", "json")
	if len(args) == 0 || args[0] != "contract" {
		args = append(args, "--jobs", "1")
	}
	code := cli.Main(args, strings.NewReader(""), &out, &errs)
	var rep report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("flinch %q exited %d without a JSON report: %v\n%s%s", args, code, err, out.String(), errs.String())
	}
	if rep.Exit != code {
		t.Errorf("the report says exit %d, but flinch exited %d", rep.Exit, code)
	}
	return code, rep
}

// problemsAt checks that the problems sit at exactly these lines of the run fixture's mutants.md.
func problemsAt(t *testing.T, what string, ps []jsonProblem, lines ...int) {
	t.Helper()
	var got, want []string
	for _, p := range ps {
		got = append(got, fmt.Sprintf("%s:%d", p.Path, p.Line))
	}
	for _, l := range lines {
		want = append(want, fmt.Sprintf("%s:%d", mutantsPath, l))
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s at %q, want %q\n%v", what, got, want, ps)
	}
}
