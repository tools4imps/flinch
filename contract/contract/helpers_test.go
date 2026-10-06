package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/problem"
)

// here is the test package's directory, where the fixtures are, before any test changes into one.
var here, _ = os.Getwd()

// fixture is the absolute root of a fixture module under testdata.
func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(here, "testdata", filepath.FromSlash(name))
}

// load reads the Contract that dir names in a module root, failing the test when flinch can't read
// it at all.
func load(t *testing.T, root, dir string) (*contract.Contract, []problem.Problem) {
	t.Helper()
	c, ps, err := contract.Load(root, dir)
	if err != nil {
		t.Fatalf("Load(%q): %v", dir, err)
	}
	return c, ps
}

// tree writes a module into a fresh temporary directory and returns its root. A value starting with
// "-> " makes a symlink to the rest of the value, so a fixture can hold what git can't carry well.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/tree\n\ngo 1.25\n"
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if target, ok := strings.CutPrefix(body, "-> "); ok {
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// names lists a Contract's primitives in the order Load gave them.
func names(c *contract.Contract) []string {
	var out []string
	for _, p := range c.Primitives {
		out = append(out, p.Name)
	}
	return out
}

// place is where a problem points, without its wording.
type place struct {
	Path string
	Line int
}

func places(ps []problem.Problem) []place {
	out := []place{}
	for _, p := range ps {
		out = append(out, place{p.Path, p.Line})
	}
	return out
}

// paths lists the paths problems point at.
func paths(ps []problem.Problem) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Path)
	}
	return out
}

// message is the wording of the one problem at a place, or "" when there's none.
func message(ps []problem.Problem, path string, line int) string {
	for _, p := range ps {
		if p.Path == path && p.Line == line {
			return p.Message
		}
	}
	return ""
}

// run runs flinch in a module's directory and returns the exit code and what it printed on stdout.
func run(t *testing.T, root string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(root)
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
