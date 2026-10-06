package mutate_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutate"
	"github.com/tools4imps/flinch/internal/units"
)

// Contract: mutate/M1
func TestFilesOutsideMutableGetNoMutants(t *testing.T) {
	l, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	all, _ := mutate.Generate(l, p, mutableAll(pkgs), ownAll, nil)

	mutable := mutableAll(pkgs)
	delete(mutable, "ops/arith.go")
	delete(mutable, "ops/units.go")
	some, _ := mutate.Generate(l, p, mutable, ownAll, nil)

	// Leaving files out changes nothing in the others, not even the numbers of the init functions
	// that follow the ones left out.
	var want []model.Mutant
	for _, m := range all {
		if m.File != "ops/arith.go" && m.File != "ops/units.go" {
			want = append(want, m)
		}
	}
	sameList(t, "with arith.go and units.go left out", ids(some), ids(want))
	if !slices.Equal(some, want) {
		t.Error("leaving files out changed the mutants in the other files")
	}
	if len(want) == len(all) {
		t.Fatal("the fixture's arith.go and units.go have no mutants to leave out")
	}
	find(t, some, "ops.init.3: limit > 5 -> limit >= 5")

	none, unviable := mutate.Generate(l, p, map[string]bool{}, ownAll, nil)
	if len(none) != 0 || unviable != 0 {
		t.Errorf("with no mutable files: %d mutants and %d unviable", len(none), unviable)
	}
}

// Contract: mutate/M1
func TestUnitsNoPrimitiveOwnsGetNoMutants(t *testing.T) {
	l, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	unowned := []string{"Count", "nested", "init.1", "List.Len", "Errors", "plain.Bump"}
	var asked []string
	owner := func(top string) (string, bool) {
		asked = append(asked, top)
		if slices.Contains(unowned, top) {
			return "", false
		}
		// Two primitives split the rest, so each mutant has to carry its own unit's owner.
		if strings.ToUpper(top[:1]) == top[:1] && top[:1] != "_" {
			return "upper", true
		}
		return "lower", true
	}
	all, _ := mutate.Generate(l, p, mutableAll(pkgs), ownAll, nil)
	some, _ := mutate.Generate(l, p, mutableAll(pkgs), owner, nil)

	var want []string
	for _, m := range all {
		if !slices.Contains(unowned, m.Top) {
			want = append(want, m.ID)
		}
	}
	sameList(t, "with some units unowned", ids(some), want)
	for _, m := range some {
		prim, _ := owner(m.Top)
		if m.Primitive != prim {
			t.Errorf("%s: primitive %q, want %q", m.ID, m.Primitive, prim)
		}
	}
	for _, id := range []string{
		"ops.Count.func1: b() + 1 -> b() - 1",
		"ops.nested.func2: true -> false",
		"ops.init.1: limit++ -> limit--",
	} {
		if slices.Contains(ids(some), id) {
			t.Errorf("%s belongs to an unowned unit but was mutated", id)
		}
	}
	for _, top := range asked {
		if strings.Contains(top, ".func") {
			t.Errorf("the owner was asked about %q, a function literal rather than a top-level unit", top)
		}
	}
	for _, top := range []string{"Arith", "myErr.Error", "init.3", "limit", "_", "Count"} {
		if !slices.Contains(asked, top) {
			t.Errorf("the owner was never asked about %q", top)
		}
	}

	none, _ := mutate.Generate(l, p, mutableAll(pkgs), func(string) (string, bool) { return "", false }, nil)
	if len(none) != 0 {
		t.Errorf("an owner that owns nothing still got %d mutants", len(none))
	}
}

// Covers references, ownership and declarations all name units, so units are named the way the
// Contract names them.
//
// Contract: mutate/M1
func TestUnitsAreNamedTheWayCoversNameThem(t *testing.T) {
	_, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	us := units.Of(p.Files, p.Names)
	var got []string
	for _, u := range us {
		got = append(got, u.Name+" "+u.Top)
	}
	sameSet(t, "ops units", got, []string{
		"Arith Arith", "Warm Warm", "Bits Bits", "Signs Signs", "Compound Compound", "init.0 init.0",
		"Calls Calls", "Strand Strand", "Now Now", "Now.func1 Now", "Now.func2 Now",
		"Check Check",
		"Compare Compare", "Order Order", "Present Present", "Twice Twice",
		"Greet Greet", "Tag Tag", "Mixed Mixed",
		"Zero Zero", "Blank Blank", "Off Off", "Rate Rate", "Hex Hex", "Imag Imag", "NUL NUL", "Bare Bare",
		"Done Done", "Nothing Nothing", "NilErr NilErr", "NilPtr NilPtr", "Boxed Boxed", "BoxedFalse BoxedFalse",
		"BoxedEmpty BoxedEmpty", "One One", "Letter Letter", "Text Text", "Yes Yes", "Pointer Pointer", "Two Two",
		"Named Named", "Touch Touch", "Generic Generic", "Bound Bound", "Pair Pair",
		"errBad errBad", "Errors Errors", "myErr.Error myErr.Error", "Concrete Concrete", "Boxing Boxing",
		"Pass Pass", "Wrap Wrap", "Later Later", "Later.func1 Later",
		"Flags Flags", "Not Not", "Odd Odd", "Make Make", "Shadow Shadow",
		"Sum Sum", "Double Double", "Halve Halve", "Join Join", "Plus Plus",
		"Sig Sig",
		"Quit Quit",
		"Step Step",
		"limit limit", "positive positive", "positive.func1 positive", "second second", "_ _",
		"grouped grouped", "nested nested", "nested.func1 nested", "nested.func2 nested",
		"init.1 init.1", "init.2 init.2", "List.Len List.Len", "Pair2.Key Pair2.Key",
		"plain.Bump plain.Bump", "plain.Same plain.Same",
		"Count Count", "Count.func1 Count", "Count.func2 Count", "Count.func3 Count",
		"Small Small", "Scale Scale", "Must Must",
		"init.3 init.3",
	})
	if u := units.Find(us, "Missing"); u != nil {
		t.Errorf("Find found a unit called Missing: %+v", u)
	}

	// One mutant from each kind of unit, so a unit misnamed in units.Of shows up in an id.
	ms, _ := generate(t, opsPath, nil)
	for _, id := range []string{
		"ops.Arith: a + b -> a - b",
		"ops.myErr.Error: { ... } -> { return *new(string) }",
		"ops.List.Len: { ... } -> { return *new(int) }",
		"ops.Pair2.Key: { ... } -> { return *new(K) }",
		"ops.plain.Bump: p.n++ -> p.n--",
		"ops.plain.Same: true -> false",
		"ops.Count.func1: b() + 1 -> b() - 1",
		"ops.Count: a() * c() -> a() / c()",
		"ops.nested.func2: true -> false",
		"ops.positive.func1: x > 0 -> x >= 0",
		"ops.limit: 10 * 2 -> 10 / 2",
		"ops.second: 2 + 3 -> 2 - 3",
		"ops._: 4 - 1 -> 4 + 1",
		"ops.grouped: 6 * 7 -> 6 / 7",
		"ops.init.0: { ... } -> { return }",
		"ops.init.1: limit++ -> limit--",
		"ops.init.2: count-- -> count++",
		"ops.init.3: limit > 5 -> limit >= 5",
	} {
		find(t, ms, id)
	}
	for _, m := range ms {
		if !strings.HasPrefix(m.ID, "ops."+m.Unit+": ") || m.Dir != "ops" {
			t.Errorf("%s: unit %q in dir %q", m.ID, m.Unit, m.Dir)
		}
	}

	root, _ := generate(t, rootPath, nil)
	if got := ids(root); !slices.Equal(got, []string{"..Root: { ... } -> { return *new(bool) }", "..Root: a == 1 -> a != 1"}) {
		t.Errorf("the root package's mutants are %q", got)
	}
	if root[0].Dir != "." || root[0].File != "fix.go" {
		t.Errorf("the root package's mutants sit in dir %q, file %q", root[0].Dir, root[0].File)
	}
}

// Contract: mutate/M10
func TestEachMutantBelongsToTheInnermostUnitHoldingIt(t *testing.T) {
	_, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	us := units.Of(p.Files, p.Names)
	ms, _ := generate(t, opsPath, nil)
	for _, m := range ms {
		u := units.Find(us, m.Unit)
		if u == nil {
			t.Errorf("%s: no unit called %q", m.ID, m.Unit)
			continue
		}
		tf := p.Fset.File(u.Pos)
		from, to := tf.Offset(u.Pos), tf.Offset(u.End)
		if u.File != m.File || m.Start < from || m.End > to {
			t.Errorf("%s: the change at %s[%d:%d] lies outside unit %s at %s[%d:%d]", m.ID, m.File, m.Start, m.End, u.Name, u.File, from, to)
		}
		// No unit inside the one named holds the change too.
		for _, inner := range us {
			if inner.Pos > u.Pos && inner.End <= u.End && inner.File == u.File &&
				tf.Offset(inner.Pos) <= m.Start && m.End <= tf.Offset(inner.End) {
				t.Errorf("%s: %s, inside %s, holds the change too", m.ID, inner.Name, u.Name)
			}
		}
		if m.Top != u.Top {
			t.Errorf("%s: top %q, want %q", m.ID, m.Top, u.Top)
		}
		linked := u.Kind == units.Var || u.Kind == units.Init
		if m.Linked != linked {
			t.Errorf("%s: Linked is %v, but the unit is a variable or init function: %v", m.ID, m.Linked, linked)
		}
	}

	// Calling a function literal on the spot is a change to the function around it.
	for _, id := range []string{
		"ops.Now: func() { *n++ }() -> (removed)",
		"ops.Now.func1: *n++ -> *n--",
		"ops.Now: func() int { return *n }() + 1 -> func() int { return *n }() - 1",
	} {
		find(t, ms, id)
	}

	// Innermost finds the smallest unit around a position, whatever order the units come in.
	reversed := slices.Clone(us)
	slices.Reverse(reversed)
	for _, u := range us {
		for _, list := range [][]units.Unit{us, reversed} {
			if got := units.Innermost(list, u.Pos); got == nil || got.Name != u.Name {
				t.Errorf("Innermost at the start of %s found %+v", u.Name, got)
			}
		}
	}
}

// The static check names units in source it never compiles, so a declaration go/types would reject
// still gets a name, and a method called init is a method rather than an init function.
//
// Contract: mutate/M1
func TestUnitsAreNamedInSourceThatDoesntCompile(t *testing.T) {
	src := "package p\n\nfunc () M() {}\n\nfunc (T) init() {}\n\nfunc init() {}\n"
	f, _ := parser.ParseFile(token.NewFileSet(), "bad.go", src, parser.SkipObjectResolution)
	if f == nil {
		t.Fatal("bad.go didn't parse at all")
	}
	var got []string
	for _, u := range units.Of([]*ast.File{f}, []string{"bad.go"}) {
		got = append(got, u.Name)
	}
	if want := []string{"M", "T.init", "init.0"}; !slices.Equal(got, want) {
		t.Errorf("units %q, want %q", got, want)
	}
}

// Contract: mutate/M1
func TestCodeOutsideUnitsIsNeverMutated(t *testing.T) {
	ms, _ := generate(t, opsPath, nil)
	var got []string
	for _, m := range ms {
		if m.File == "ops/outside.go" {
			got = append(got, m.ID)
		}
	}
	// The constant, the array type and the signature's array lengths all hold arithmetic.
	if want := []string{"ops.Sig: { ... } -> { return *new([1 + 1]int) }"}; !slices.Equal(got, want) {
		t.Errorf("outside.go got %q, want only Sig's erase mutant", got)
	}
	for _, m := range ms {
		if m.Unit == "count" || m.Unit == "errBad" {
			t.Errorf("%s: a variable without arithmetic in its initializer got a mutant", m.ID)
		}
	}
}
