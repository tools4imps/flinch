package tags_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
	"github.com/tools4imps/flinch/internal/tags"
)

// here is the test package's directory, where the fixtures are, before any test changes into one.
var here, _ = os.Getwd()

// scan finds the Contract tests of a module the way flinch contract does. A relative module names a
// fixture under testdata.
func scan(t *testing.T, module string) ([]model.Test, []problem.Problem) {
	t.Helper()
	if !filepath.IsAbs(module) {
		module = filepath.Join("testdata", module)
	}
	m, err := source.FindModule(module)
	if err != nil {
		t.Fatal(err)
	}
	c, ps, err := contract.Load(m.Root, "")
	if err != nil || len(ps) > 0 {
		t.Fatalf("the fixture's Contract doesn't load: %v %v", ps, err)
	}
	pkgs, err := source.Packages(m, source.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return tags.Scan(m, c, pkgs)
}

// find returns the Contract test with a name, or nil.
func find(tests []model.Test, name string) *model.Test {
	for i := range tests {
		if tests[i].Name == name {
			return &tests[i]
		}
	}
	return nil
}

// place is where a problem points, without its wording.
type place struct {
	Path string
	Line int
}

// at counts the problems at a place.
func at(ps []problem.Problem, path string, line int) int {
	n := 0
	for _, p := range ps {
		if p.Path == path && p.Line == line {
			n++
		}
	}
	return n
}

// in lists the lines of a file that problems point at, in order.
func in(ps []problem.Problem, path string) []int {
	out := []int{}
	for _, p := range ps {
		if p.Path == path {
			out = append(out, p.Line)
		}
	}
	return out
}

// run runs flinch in a module's directory and returns the exit code and stdout. A relative module
// names a fixture under testdata.
func run(t *testing.T, module string, args ...string) (int, string) {
	t.Helper()
	if !filepath.IsAbs(module) {
		module = filepath.Join(here, "testdata", module)
	}
	t.Chdir(module)
	var stdout, stderr bytes.Buffer
	code := cli.Main(args, strings.NewReader(""), &stdout, &stderr)
	if code == 2 {
		t.Fatalf("flinch %s exited 2: %s", strings.Join(args, " "), stderr.String())
	}
	return code, stdout.String()
}

// jsonErrors reads the Contract errors from a JSON report.
func jsonErrors(t *testing.T, out string) []problem.Problem {
	t.Helper()
	var rep struct {
		ContractErrors []problem.Problem `json:"contract_errors"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the JSON report doesn't parse: %v\n%s", err, out)
	}
	return rep.ContractErrors
}
