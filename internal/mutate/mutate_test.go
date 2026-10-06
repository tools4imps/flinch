package mutate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/typecheck"
)

var update = flag.Bool("update", false, "rewrite testdata/mutants.golden")

var fixturePaths = []string{"example.com/fix", "example.com/fix/ops"}

// mutable leaves ops/c.go out, so its function never gets a mutant.
var mutable = map[string]bool{
	"fix.go": true, "ops/a.go": true, "ops/b.go": true, "ops/d.go": true, "ops/e.go": true, "ops/zero.go": true,
}

// owned gives every top-level unit a primitive except Unowned.
func owned(top string) (string, bool) {
	if top == "Unowned" {
		return "", false
	}
	return "ops", true
}

func load(t *testing.T, root string) (*typecheck.Loader, map[string]*typecheck.Package) {
	t.Helper()
	// A go.work above the fixture would pull it into a workspace it isn't part of.
	t.Setenv("GOWORK", "off")
	l, pkgs, err := typecheck.Load(context.Background(), root, nil, fixturePaths)
	if err != nil {
		t.Fatal(err)
	}
	return l, pkgs
}

// generateAll returns the fixture's mutants for both packages, root package first.
func generateAll(t *testing.T, root string, ops []string) ([]model.Mutant, int) {
	t.Helper()
	l, pkgs := load(t, root)
	var all []model.Mutant
	total := 0
	for _, ip := range fixturePaths {
		ms, unviable := Generate(l, pkgs[ip], mutable, owned, ops)
		all = append(all, ms...)
		total += unviable
	}
	return all, total
}

func TestGolden(t *testing.T) {
	ms, unviable := generateAll(t, "testdata/fix", nil)
	var b strings.Builder
	fmt.Fprintf(&b, "# %d mutants, %d unviable\n", len(ms), unviable)
	for _, m := range ms {
		flags := ""
		if m.Erase {
			flags += " [erase]"
		}
		if m.Linked {
			flags += " [linked]"
		}
		fmt.Fprintf(&b, "%s:%d:%d %s: %s%s\n", m.File, m.Line, m.Col, m.Operator, m.ID, flags)
	}
	golden := filepath.Join("testdata", "mutants.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("mutants differ from %s (run with -update to see the difference in git)\ngot:\n%s", golden, got)
	}
}

func TestFieldsOfOneMutant(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	m := find(t, ms, "ops.Compare: a < b -> a <= b")
	src, err := os.ReadFile("testdata/fix/ops/a.go")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(src[m.Start:m.End]); got != "<" {
		t.Errorf("the mutant replaces %q, want %q", got, "<")
	}
	want := model.Mutant{
		ID: "ops.Compare: a < b -> a <= b", Hash: mutantid.Hash("ops.Compare: a < b -> a <= b"),
		Primitive: "ops", Dir: "ops", ImportPath: "example.com/fix/ops", File: "ops/a.go",
		Line: m.Line, Col: 7, Start: m.Start, End: m.Start + 1, Text: "<=", Operator: "boundary",
		Unit: "Compare", Top: "Compare", Original: "a < b", Replace: "a <= b",
	}
	if m != want {
		t.Errorf("got  %+v\nwant %+v", m, want)
	}
	if line := strings.Split(string(src), "\n")[m.Line-1]; !strings.Contains(line, "if a < b") {
		t.Errorf("line %d is %q, want the if statement", m.Line, line)
	}
}

func TestUnitsAndLinks(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	cases := []struct {
		id, unit, top string
		linked        bool
	}{
		{"ops.Counter.func1: n++ -> n--", "Counter.func1", "Counter", false},
		{"ops.limit: 10 * 2 -> 10 / 2", "limit", "limit", true},
		{"ops.positive.func1: x > 0 -> x >= 0", "positive.func1", "positive", false},
		{"ops.init.0: limit++ -> limit--", "init.0", "init.0", true},
		{"ops.init.1: limit > 5 -> limit >= 5", "init.1", "init.1", true},
		{"ops.init.1: { ... } -> { return }", "init.1", "init.1", true},
		{"ops.List.Len: { ... } -> { return *new(int) }", "List.Len", "List.Len", false},
		{"..Root: a == 1 -> a != 1", "Root", "Root", false},
	}
	for _, c := range cases {
		m := find(t, ms, c.id)
		if m.Unit != c.unit || m.Top != c.top || m.Linked != c.linked {
			t.Errorf("%s: unit %q, top %q, linked %v; want %q, %q, %v", c.id, m.Unit, m.Top, m.Linked, c.unit, c.top, c.linked)
		}
	}
}

func TestSkipsWhatItShould(t *testing.T) {
	ms, unviable := generateAll(t, "testdata/fix", nil)
	for _, m := range ms {
		switch {
		case m.Unit == "Unowned":
			t.Errorf("a unit without an owner got a mutant: %s", m.ID)
		case m.File == "ops/c.go":
			t.Errorf("a file outside mutable got a mutant: %s", m.ID)
		case m.Unit == "Concat" && !m.Erase:
			t.Errorf("string concatenation got a mutant: %s", m.ID)
		case m.Unit == "Join" && !m.Erase:
			t.Errorf("+ on a type parameter that may be a string got a mutant: %s", m.ID)
		case m.Unit == "Odd" && !m.Erase:
			t.Errorf("a field named true got a mutant: %s", m.ID)
		case m.Unit == "Nothing":
			t.Errorf("an empty function got a mutant: %s", m.ID)
		case m.Unit == "Calls" && m.Operator == "drop-call" && !strings.HasPrefix(m.Original, "fmt."):
			t.Errorf("a logging call got a mutant: %s", m.ID)
		case m.Operator == "drop-error" && strings.Contains(m.Original, "nil") && !strings.Contains(m.Original, "errBad") && !strings.Contains(m.Original, "Errorf"):
			t.Errorf("a nil error got a mutant: %s", m.ID)
		case m.Unit == "Must" && m.Operator == "drop-call":
			t.Errorf("a removal that leaves no terminating statement survived the type-check: %s", m.ID)
		}
	}
	if unviable != 1 {
		t.Errorf("%d mutants were unviable, want 1: removing the panic in Must", unviable)
	}
}

// TestFallbacks checks that a removal that strands an import or a local is kept in its fallback
// form, under the same id, and that one the type-check accepts as is keeps its plain form.
func TestFallbacks(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	cases := []struct{ id, text string }{
		{"ops.Strand: fmt.Fprint(w, x) -> (removed)", "if false { fmt.Fprint(w, x) }"},
		{"ops.Quit: os.Exit(code) -> (removed)", "if false { os.Exit(code) }"},
		{`ops.Check: return errors.New("too big") -> return nil`,
			`func() error { if false { _ = errors.New("too big") }; return nil }()`},
		{"ops.Wrap: return errors.New( msg) -> return nil",
			"func() error { if false { _ = errors.New(\n\t\tmsg) }; return nil }()"},
		{`ops.Calls: fmt.Fprintln(w, "e") -> (removed)`, ""},
		{"ops.Errors: return 0, errBad -> return 0, nil", "nil"},
	}
	for _, c := range cases {
		m := find(t, ms, c.id)
		if m.Text != c.text {
			t.Errorf("%s: Text = %q, want %q", c.id, m.Text, c.text)
		}
		src, err := os.ReadFile(filepath.Join("testdata/fix", m.File))
		if err != nil {
			t.Fatal(err)
		}
		if got := mutantid.Collapse(string(src[m.Start:m.End])); !strings.Contains(m.Original, got) {
			t.Errorf("%s: replaces %q, which isn't part of the original %q", c.id, got, m.Original)
		}
	}
}

func TestEraseSkipsZeroBodies(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	erased := map[string]bool{}
	for _, m := range ms {
		if m.Erase {
			erased[m.Unit] = true
		}
	}
	for _, unit := range []string{"Zero", "Blank", "Off", "Rate", "Bare", "Done", "Nothing"} {
		if erased[unit] {
			t.Errorf("%s already returns zero values but got an erase mutant", unit)
		}
	}
	for _, unit := range []string{"Boxed", "One", "Pointer"} {
		if !erased[unit] {
			t.Errorf("%s returns something other than zero values but got no erase mutant", unit)
		}
	}
}

func TestOccurrenceNumbers(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	first := find(t, ms, "ops.Twice: x > 0 -> x >= 0")
	second := find(t, ms, "ops.Twice: x > 0 -> x >= 0 #2")
	if first.Line >= second.Line {
		t.Errorf("the first occurrence is on line %d and the second on %d", first.Line, second.Line)
	}
}

func TestEraseComesFirstInItsFunction(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	seen := map[string]bool{}
	for _, m := range ms {
		if m.Erase {
			if seen[m.Top] {
				t.Errorf("the erase mutant of %s comes after another of its mutants", m.Top)
			}
			if m.Unit != m.Top {
				t.Errorf("an erase mutant sits in a closure: %s", m.ID)
			}
		}
		seen[m.Top] = true
	}
}

// TestKeepsLines checks M2: applying a mutant leaves every line outside the changed ones where it
// was and as it was.
func TestKeepsLines(t *testing.T) {
	ms, _ := generateAll(t, "testdata/fix", nil)
	for _, m := range ms {
		src, err := os.ReadFile(filepath.Join("testdata/fix", m.File))
		if err != nil {
			t.Fatal(err)
		}
		before := strings.Split(string(src), "\n")
		after := strings.Split(string(m.Apply(src)), "\n")
		if len(before) != len(after) {
			t.Errorf("%s: %d lines became %d", m.ID, len(before), len(after))
			continue
		}
		last := m.Line + strings.Count(string(src[m.Start:m.End]), "\n")
		for i := range before {
			if (i+1 < m.Line || i+1 > last) && before[i] != after[i] {
				t.Errorf("%s: line %d changed from %q to %q", m.ID, i+1, before[i], after[i])
			}
		}
	}
}

func TestOperatorsNarrow(t *testing.T) {
	all, _ := generateAll(t, "testdata/fix", nil)
	some, _ := generateAll(t, "testdata/fix", []string{"bool", "drop-call"})
	var want []model.Mutant
	for _, m := range all {
		if m.Operator == "bool" || m.Operator == "drop-call" {
			want = append(want, m)
		}
	}
	if len(want) == 0 {
		t.Fatal("the fixture has no bool or drop-call mutants")
	}
	if !reflect.DeepEqual(some, want) {
		t.Errorf("narrowing to bool and drop-call gave %d mutants, want %d", len(some), len(want))
	}
	none, _ := generateAll(t, "testdata/fix", []string{})
	if len(none) != 0 {
		t.Errorf("an empty operator list gave %d mutants", len(none))
	}
}

func TestDeterministicOrder(t *testing.T) {
	a, _ := generateAll(t, "testdata/fix", nil)
	b, _ := generateAll(t, "testdata/fix", nil)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two runs over the same source gave different mutants")
	}
	sorted := sort.SliceIsSorted(a, func(i, j int) bool {
		x, y := a[i], a[j]
		if x.File != y.File {
			return x.File < y.File
		}
		if x.Start != y.Start {
			return x.Start < y.Start
		}
		if x.Erase != y.Erase {
			return x.Erase
		}
		return x.Operator < y.Operator
	})
	if !sorted {
		t.Error("mutants aren't ordered by file, start offset and operator")
	}
}

// TestIDsSurviveEditsElsewhere checks I4: adding a function and changing another unit's body leave
// every other unit's ids alone, though their lines move.
func TestIDsSurviveEditsElsewhere(t *testing.T) {
	before, _ := generateAll(t, "testdata/fix", nil)
	dir := t.TempDir()
	copyDir(t, "testdata/fix", dir)
	edit(t, filepath.Join(dir, "ops/a.go"), "func Arith(", "func Extra(x int) int {\n\treturn x * 2\n}\n\nfunc Arith(")
	edit(t, filepath.Join(dir, "ops/a.go"), "\ts += \"!\"\n", "\ts += \"!\"\n\tif len(s) > 3 {\n\t\ts = s[:3]\n\t}\n")
	after, _ := generateAll(t, dir, nil)

	byUnit := func(ms []model.Mutant) map[string][]string {
		out := map[string][]string{}
		for _, m := range ms {
			if m.Unit != "Extra" && m.Unit != "Concat" {
				out[m.Unit] = append(out[m.Unit], m.ID)
			}
		}
		return out
	}
	if b, a := byUnit(before), byUnit(after); !reflect.DeepEqual(b, a) {
		t.Errorf("ids changed after an edit elsewhere\nbefore: %v\nafter:  %v", b, a)
	}
	moved := find(t, after, "ops.Compare: a < b -> a <= b").Line - find(t, before, "ops.Compare: a < b -> a <= b").Line
	if moved != 7 {
		t.Errorf("Compare moved %d lines, want 7, so the edit didn't land where the test meant", moved)
	}
}

func TestOneLine(t *testing.T) {
	cases := map[string]string{
		"[]int":                            "[]int",
		"struct {\n\ta int\n\tb string\n}": "struct { a int; b string; }",
		"struct {\n\ta int // count\n\tb string\n}": "struct { a int; b string; }",
		"struct {\n\ta int `json:\"a\n\"`\n}":       "struct { a int \"json:\\\"a\\n\\\"\"; }",
		"func(\n\ta int,\n) error":                  "func( a int, ) error",
		"map[string]struct {\n\tx int\n}":           "map[string]struct { x int; }",
	}
	for in, want := range cases {
		if got := oneLine(in); got != want {
			t.Errorf("oneLine(%q) = %q, want %q", in, got, want)
		}
	}
}

func find(t *testing.T, ms []model.Mutant, id string) model.Mutant {
	t.Helper()
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no mutant %q", id)
	return model.Mutant{}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func edit(t *testing.T, file, old, new string) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s doesn't contain %q", file, old)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(b), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}
