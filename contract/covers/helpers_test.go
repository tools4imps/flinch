package covers_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/covers"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

// fixture is the absolute path of the fixture module, so a test that has changed directory can
// still copy it.
var fixture, _ = filepath.Abs(filepath.Join("testdata", "mod"))

// copyFixture copies the fixture module to a new temporary directory and returns its root.
func copyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	return root
}

// writeFile writes text to a slash-separated path under root, making its directory first.
func writeFile(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// firstRef is the README.md line that holds the first covers line readme writes.
const firstRef = 6

// readme is a primitive's README.md with one obligation and a covers block holding lines, the
// first of them on line firstRef.
func readme(name string, lines []string) string {
	return "# " + name + "\n\n- **P1** A fixture obligation.\n\n```covers\n" + strings.Join(lines, "\n") + "\n```\n"
}

// contractTest is a Contract test file for a fixture primitive, tagged with its one obligation.
func contractTest(name string) string {
	return "package " + name + "_test\n\nimport \"testing\"\n\n// Contract: " + name + "/P1\nfunc TestP1(t *testing.T) {}\n"
}

// resolved is what flinch's static check learns about a module: its packages, and who owns what.
type resolved struct {
	module   source.Module
	pkgs     []source.Package
	parsed   map[string]*source.Parsed
	contract *contract.Contract
	own      *covers.Ownership
	problems []problem.Problem
}

// resolve copies the fixture, gives each named primitive a README.md with the given covers lines,
// and resolves the Contract the way flinch does.
func resolve(t *testing.T, prims map[string][]string) resolved {
	t.Helper()
	root := copyFixture(t)
	for name, lines := range prims {
		writeFile(t, root, "contract/"+name+"/README.md", readme(name, lines))
	}
	return resolveAt(t, root)
}

// resolveAt resolves the Contract of the module holding dir.
func resolveAt(t *testing.T, dir string) resolved {
	t.Helper()
	m, err := source.FindModule(dir)
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
	return resolved{module: m, pkgs: pkgs, parsed: parsed, contract: c, own: own, problems: ps}
}

// An owned case says which primitive should own a top-level unit, with "" for none.
type owned struct{ dir, top, want string }

// expect checks the owner of each unit.
func (r resolved) expect(t *testing.T, cases []owned) {
	t.Helper()
	for _, c := range cases {
		got, ok := r.own.Owner(c.dir, c.top)
		switch {
		case c.want == "" && ok:
			t.Errorf("Owner(%q, %q) = %q, want no owner", c.dir, c.top, got)
		case c.want != "" && (!ok || got != c.want):
			t.Errorf("Owner(%q, %q) = %q, %v; want %q", c.dir, c.top, got, ok, c.want)
		}
	}
}

// clean fails the test when resolving reported any problem.
func (r resolved) clean(t *testing.T) {
	t.Helper()
	for _, p := range r.problems {
		t.Errorf("unexpected problem: %v", p)
	}
}

// pkg returns the package in dir, failing the test when there's none.
func (r resolved) pkg(t *testing.T, dir string) source.Package {
	t.Helper()
	for _, p := range r.pkgs {
		if p.Dir == dir {
			return p
		}
	}
	t.Fatalf("no package in %s", dir)
	return source.Package{}
}

// sorted returns a sorted copy of s, for comparing lists whose order the Contract leaves open.
func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// run runs flinch in dir the way cmd/flinch would and returns its exit code and output.
func run(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errs bytes.Buffer
	code = cli.Main(args, strings.NewReader(""), &out, &errs)
	return code, out.String(), errs.String()
}

// symlink makes a symbolic link at name pointing to target.
func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
}
