package declare

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/covers"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

func fixture(t *testing.T) (*contract.Contract, *covers.Ownership, map[string]*source.Parsed) {
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
		if len(p.GoFiles) > 0 {
			if parsed[p.Dir], err = source.Parse(m, p); err != nil {
				t.Fatal(err)
			}
		}
	}
	own, ps := covers.Resolve(c, pkgs, parsed)
	if len(ps) > 0 {
		t.Fatalf("Resolve: %v", ps)
	}
	return c, own, parsed
}

func TestCheckReportsWhatCanBeJudgedWithoutARun(t *testing.T) {
	c, own, parsed := fixture(t)
	ps := Check(c, own, parsed)
	problem.Sort(ps)
	var got []string
	for _, p := range ps {
		got = append(got, p.String())
	}
	f := "contract/p/mutants.md"
	want := []string{
		f + ":1: this unpromised block has no heading above it; put one there that says why these mutants don't count, with any explanation under it",
		f + `:13: a mutant id looks like "dir.Unit: original -> replacement"`,
		f + ":14: this declaration belongs in contract/q/mutants.md, because q covers b.B",
		f + ":15: a.Gone names no unit in the module; drop this declaration, or give it the unit's new name if it moved",
		f + ":16: no primitive covers c.C, so flinch never mutates it; drop this declaration",
		f + ":17: no primitive covers gen.Gen, so flinch never mutates it; drop this declaration",
		f + ":20: ..Root names no unit in the module; drop this declaration, or give it the unit's new name if it moved",
		f + ":21: this repeats the declaration at contract/p/mutants.md:12; drop one",
		f + ":28: an equivalent block lists exact mutant ids; list the equivalent mutants, or move a.G: * to an unpromised block",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestCheckFindsClosuresAndInitFunctions(t *testing.T) {
	c, own, parsed := fixture(t)
	for _, p := range Check(c, own, parsed) {
		if p.Line == 11 || p.Line == 19 {
			t.Errorf("a declaration on a closure or init function was reported: %v", p)
		}
	}
}

func TestExactMatchesWhateverTheWhitespace(t *testing.T) {
	c, _, _ := fixture(t)
	ix := NewIndex(c)
	for id, line := range map[string]int{
		"a.G: x - y -> x + y":          12,
		"a.G:  x   - y ->  x  +  y":    12,
		"a.F: x + 1 -> 1 + x":          27,
		"a.T.M: 1 -> 0":                18,
		"b.B: 1 -> 2":                  4,
		"a.F: x > 0 -> x >= 0":         2,
		"a.F: x > 0 -> x >= 0 #2":      0,
		"a.F: x > 0 -> x < 0":          0,
		"not an id":                    0,
		"a.F: *":                       0,
		"internal/elsewhere.F: a -> b": 0,
	} {
		d := ix.Exact(id)
		switch {
		case line == 0 && d != nil:
			t.Errorf("Exact(%q) = %+v, want nil", id, *d)
		case line != 0 && (d == nil || d.Line != line):
			t.Errorf("Exact(%q) = %+v, want line %d", id, d, line)
		}
	}
}

func TestWildcardCoversAUnitAndTheClosuresInIt(t *testing.T) {
	c, _, _ := fixture(t)
	ix := NewIndex(c)
	for _, w := range []struct {
		dir, unit string
		line      int
	}{
		{"a", "F", 10},
		{"a", "F.func1", 11}, // its own wildcard comes first
		{"a", "F.func2", 10}, // otherwise the top-level unit's
		{"a", "init.0", 19},
		{"a", "init.0.func1", 19},
		{"a", "G", 0}, // a wildcard in an equivalent block matches nothing
		{"a", "T.M", 0},
		{"b", "F", 0},
	} {
		d := ix.Wildcard(w.dir, w.unit)
		switch {
		case w.line == 0 && d != nil:
			t.Errorf("Wildcard(%q, %q) = %+v, want nil", w.dir, w.unit, *d)
		case w.line != 0 && (d == nil || d.Line != w.line):
			t.Errorf("Wildcard(%q, %q) = %+v, want line %d", w.dir, w.unit, d, w.line)
		}
	}
}

func TestAllListsEveryDeclarationInOrder(t *testing.T) {
	c, _, _ := fixture(t)
	all := NewIndex(c).All()
	if len(all) != 16 {
		t.Fatalf("All has %d declarations, want 16", len(all))
	}
	if all[0].Line != 2 || all[len(all)-1].Path != "contract/q/mutants.md" {
		t.Errorf("All = first %+v, last %+v", *all[0], *all[len(all)-1])
	}
	if all[0] != &c.Primitives[0].Declarations[0] {
		t.Error("All copies declarations instead of pointing into the Contract")
	}
}
