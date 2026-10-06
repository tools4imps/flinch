package covers

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

func resolveFixture(t *testing.T) (*Ownership, []problem.Problem) {
	t.Helper()
	m, err := source.FindModule(filepath.Join("testdata", "mod"))
	if err != nil {
		t.Fatal(err)
	}
	c, ps, err := contract.Load(m.Root, "")
	if err != nil || len(ps) > 0 {
		t.Fatalf("Load: %v %v", ps, err)
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
	o, ps := Resolve(c, pkgs, parsed)
	problem.Sort(ps)
	return o, ps
}

func TestOwnerGoesToTheMostSpecificReference(t *testing.T) {
	o, _ := resolveFixture(t)
	for _, c := range []struct{ dir, top, want string }{
		{"a", "G", "p1"},      // only the package names it
		{"a", "F", "p2"},      // a function beats a package
		{"a", "T.M", "p2"},    // a type takes all its methods
		{"a", "T.N", "p3"},    // a method beats its type
		{"a", "V", "p3"},      // a variable beats a package
		{"a", "X", "p3"},      // naming Y names the spec, whose unit is X
		{"a", "init.0", "p3"}, // init names the package's init functions
		{"tree/y", "Y", "p3"}, // a package beats a tree
		{".", "R", "p3"},      // the root package, written as mutant ids write it
		{"b", "B", "p2"},
	} {
		if got, ok := o.Owner(c.dir, c.top); !ok || got != c.want {
			t.Errorf("Owner(%q, %q) = %q, %v; want %q", c.dir, c.top, got, ok, c.want)
		}
	}
}

func TestATieGoesToThePrimitiveFirstByName(t *testing.T) {
	o, _ := resolveFixture(t)
	for _, c := range []struct{ dir, top string }{{"tree/x", "X"}, {"tree/y/z", "Z"}} {
		if got, _ := o.Owner(c.dir, c.top); got != "p1" {
			t.Errorf("Owner(%q, %q) = %q, want p1", c.dir, c.top, got)
		}
	}
}

func TestGeneratedCodeIsNeverOwned(t *testing.T) {
	o, ps := resolveFixture(t)
	for _, top := range []string{"Gen", "GT.GM"} {
		if got, ok := o.Owner("a", top); ok {
			t.Errorf("Owner(a, %q) = %q; a generated file's units are never covered", top, got)
		}
	}
	for _, p := range ps {
		if strings.HasPrefix(p.Message, "a.Gen ") || strings.HasPrefix(p.Message, "a.Z ") {
			t.Errorf("a reference to something real was reported: %v", p)
		}
	}
}

func TestAReferenceThatNamesNothingIsAProblem(t *testing.T) {
	_, ps := resolveFixture(t)
	var got []string
	for _, p := range ps {
		got = append(got, p.String())
	}
	readme := "contract/p3/README.md"
	want := []string{
		readme + ":12: nope names no package in the module; " + forms,
		readme + ":13: a.Missing names nothing: a has no function, type or variable Missing",
		readme + ":14: a.T.Missing names nothing: a has no method T.Missing",
		readme + ":15: a.K names nothing: a has no function, type or variable K",
		readme + ":16: tree/nothing/... names no package in the module; " + forms,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCoveredAndOutside(t *testing.T) {
	o, _ := resolveFixture(t)
	if got, want := o.Covered(), []string{".", "a", "b", "tree/x", "tree/y", "tree/y/z"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Covered = %q, want %q", got, want)
	}
	// types is named but has nothing to own, and contract/helpers sits in the Contract.
	if got, want := o.Outside(), []string{"outside"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Outside = %q, want %q", got, want)
	}
}

func TestSplit(t *testing.T) {
	for in, want := range map[string][2]string{
		".":                 {".", ""},
		"..Name":            {".", "Name"},
		"internal/a":        {"internal/a", ""},
		"internal/a.F":      {"internal/a", "F"},
		"internal/a.T.M":    {"internal/a", "T.M"},
		"a.b/c.F":           {"a.b/c", "F"},
		"internal/a.F.func": {"internal/a", "F.func"},
	} {
		if dir, unit := split(in); dir != want[0] || unit != want[1] {
			t.Errorf("split(%q) = %q, %q; want %q", in, dir, unit, want)
		}
	}
}
