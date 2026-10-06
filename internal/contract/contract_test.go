package contract

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/problem"
)

func load(t *testing.T) (*Contract, []problem.Problem) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "mod"))
	if err != nil {
		t.Fatal(err)
	}
	c, ps, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	return c, ps
}

func TestLoadFindsPrimitivesAndSkipsHelpersAndIgnoredNames(t *testing.T) {
	c, _ := load(t)
	if c.Dir != "contract" {
		t.Errorf("Dir = %q", c.Dir)
	}
	var names []string
	for _, p := range c.Primitives {
		names = append(names, p.Name)
	}
	if !reflect.DeepEqual(names, []string{"alpha", "beta"}) {
		t.Errorf("primitives = %q, want alpha and beta", names)
	}
	a := c.Primitive("alpha")
	if a == nil || a.Dir != "contract/alpha" || a.Readme != "contract/alpha/README.md" || a.Mutants != "contract/alpha/mutants.md" {
		t.Errorf("alpha = %+v", a)
	}
	if b := c.Primitive("beta"); b == nil || b.Mutants != "" {
		t.Errorf("beta = %+v", b)
	}
	if c.Primitive("helpers") != nil {
		t.Error("a directory without a README.md became a primitive")
	}
}

func TestLoadReadsObligationsOutsideFencesAndComments(t *testing.T) {
	c, _ := load(t)
	want := []Obligation{{"A1", 5}, {"A2", 6}, {"A3", 31}}
	if got := c.Primitive("alpha").Obligations; !reflect.DeepEqual(got, want) {
		t.Errorf("obligations = %v, want %v", got, want)
	}
}

func TestLoadReportsADuplicateObligation(t *testing.T) {
	c, ps := load(t)
	if got := c.Primitive("beta").Obligations; !reflect.DeepEqual(got, []Obligation{{"B1", 3}, {"B2", 4}}) {
		t.Errorf("obligations = %v", got)
	}
	if !has(ps, "contract/beta/README.md", 5, "beta/B1 is already an obligation at line 3") {
		t.Errorf("no duplicate problem in %v", ps)
	}
}

func TestLoadReadsCoversOnlyFromTheReadme(t *testing.T) {
	c, ps := load(t)
	want := []Ref{{"internal/alpha", 13}, {"internal/beta.F", 15}}
	if got := c.Primitive("alpha").Covers; !reflect.DeepEqual(got, want) {
		t.Errorf("covers = %v, want %v", got, want)
	}
	for _, w := range []struct {
		path string
		line int
		msg  string
	}{
		{"contract/alpha/mutants.md", 34, "a covers block counts only in contract/alpha/README.md"},
		{"contract/alpha/notes.md", 3, "a covers block counts only in contract/alpha/README.md"},
		{"contract/alpha/README.md", 33, "an equivalent block counts only in contract/alpha/mutants.md"},
		{"contract/alpha/notes.md", 7, "an unpromised block counts only in contract/alpha/mutants.md"},
	} {
		if !has(ps, w.path, w.line, w.msg) {
			t.Errorf("no problem %s:%d: %s in %v", w.path, w.line, w.msg, ps)
		}
	}
}

func TestLoadReadsDeclarationsWithTheirReasons(t *testing.T) {
	c, _ := load(t)
	logging := "Logging is open The Contract leaves log text open. It says nothing about it."
	want := []Declaration{
		{Kind: "unpromised", Text: "internal/alpha.F: *", Path: "contract/alpha/mutants.md", Line: 4, Reason: "", Block: 3},
		{Kind: "unpromised", Text: "internal/alpha.Log: *", Path: "contract/alpha/mutants.md", Line: 13, Reason: logging, Block: 12},
		{Kind: "unpromised", Text: `internal/alpha.Log: "a" -> "b"`, Path: "contract/alpha/mutants.md", Line: 16, Reason: logging, Block: 12},
		{Kind: "equivalent", Text: "internal/alpha.F: i < n -> i != n", Path: "contract/alpha/mutants.md", Line: 25, Reason: "Setext heading Equivalence here.", Block: 24},
		{Kind: "equivalent", Text: "internal/alpha.G: x -> y", Path: "contract/alpha/mutants.md", Line: 31, Reason: "Setext heading After the block, prose starts over.", Block: 30},
	}
	if got := c.Primitive("alpha").Declarations; !reflect.DeepEqual(got, want) {
		t.Errorf("declarations =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLoadReportsAGoFileThatIsntATest(t *testing.T) {
	_, ps := load(t)
	if !has(ps, "contract/alpha/stray.go", 0, "a primitive's directory holds only Contract tests") {
		t.Errorf("no stray .go problem in %v", ps)
	}
	for _, p := range ps {
		if strings.Contains(p.Path, "helper") || strings.Contains(p.Path, "/sub/") {
			t.Errorf("a Go file outside a primitive's own directory was reported: %v", p)
		}
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	_, ps := load(t)
	problem.Sort(ps)
	var got []string
	for _, p := range ps {
		got = append(got, p.String())
	}
	want := []string{
		"contract/alpha/README.md:33: an equivalent block counts only in contract/alpha/mutants.md; move it there",
		"contract/alpha/mutants.md:34: a covers block counts only in contract/alpha/README.md; move it there",
		"contract/alpha/notes.md:3: a covers block counts only in contract/alpha/README.md; move it there",
		"contract/alpha/notes.md:7: an unpromised block counts only in contract/alpha/mutants.md; move it there",
		"contract/alpha/stray.go: a primitive's directory holds only Contract tests; rename this file to end in _test.go or move it out of the Contract",
		"contract/beta/README.md:5: beta/B1 is already an obligation at line 3; give this one its own id",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestLoadReportsAMissingContract(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"", "elsewhere"} {
		c, ps, err := Load(root, dir)
		if err != nil {
			t.Fatal(err)
		}
		want := dir
		if want == "" {
			want = "contract"
		}
		if len(c.Primitives) != 0 || len(ps) != 1 || ps[0].Path != want || !strings.Contains(ps[0].Message, "there's no Contract here") {
			t.Errorf("Load(%q) = %v, %v", dir, c.Primitives, ps)
		}
	}
}

func TestLoadAcceptsAnAbsoluteDirectory(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("testdata", "mod"))
	c, _, err := Load(root, filepath.Join(root, "contract"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Dir != "contract" || len(c.Primitives) != 2 {
		t.Errorf("Load = %q with %d primitives", c.Dir, len(c.Primitives))
	}
}

func TestLoadReportsSymlinksAndFilesThatArentUTF8(t *testing.T) {
	root := t.TempDir()
	write(t, root, "contract/p/README.md", "# p\n\n- **P1** One.\n")
	write(t, root, "contract/p/p_test.go", "package p_test\n")
	write(t, root, "contract/p/bad.txt", "fine\nnot \xff fine\n")
	write(t, root, "contract/_skipped/bad.txt", "\xff")
	write(t, root, "elsewhere.md", "# outside\n")
	write(t, root, "contract/q/target.md", "# q\n")
	link(t, filepath.Join(root, "elsewhere.md"), filepath.Join(root, "contract", "p", "link.md"))
	link(t, filepath.Join(root, "elsewhere.md"), filepath.Join(root, "contract", "q", "README.md"))
	c, ps, err := Load(root, "contract")
	if err != nil {
		t.Fatal(err)
	}
	problem.Sort(ps)
	var got []string
	for _, p := range ps {
		got = append(got, p.Path+":"+strconv.Itoa(p.Line))
	}
	want := []string{"contract/p/bad.txt:2", "contract/p/link.md:0", "contract/q/README.md:0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems = %v, want %v", ps, want)
	}
	if q := c.Primitive("q"); q == nil || len(q.Obligations) != 0 {
		t.Errorf("q = %+v; a symlinked README.md still makes a primitive but is never read", q)
	}
}

func TestLoadReportsASymlinkedContractDirectory(t *testing.T) {
	root := t.TempDir()
	write(t, root, "real/p/README.md", "# p\n")
	link(t, filepath.Join(root, "real"), filepath.Join(root, "contract"))
	_, ps, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Path != "contract" || !strings.Contains(ps[0].Message, "symlink") {
		t.Errorf("problems = %v", ps)
	}
}

func has(ps []problem.Problem, path string, line int, msg string) bool {
	for _, p := range ps {
		if p.Path == path && p.Line == line && strings.Contains(p.Message, msg) {
			return true
		}
	}
	return false
}

func write(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}
