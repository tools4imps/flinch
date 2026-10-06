package mutate_test

import (
	"slices"
	"strings"
	"testing"
)

// Contract: mutate/M3
func TestMutantsThatFailToTypeCheckAreDropped(t *testing.T) {
	ms, unviable := generate(t, opsPath, nil)
	// A constant overflow, a division by a constant zero, and a function left without a terminating
	// statement in the plain form and the fallback form alike.
	for _, id := range []string{
		"ops.Small: 0 + 1 -> 0 - 1",
		"ops.Scale: x * 0 -> x / 0",
		`ops.Must: panic("not ok") -> (removed)`,
	} {
		if slices.Contains(ids(ms), id) {
			t.Errorf("%s doesn't type-check but was kept", id)
		}
	}
	if unviable != 3 {
		t.Errorf("%d mutants were unviable, want 3", unviable)
	}
	// Their neighbors type-check and stay.
	for _, id := range []string{
		"ops.Small: { ... } -> { return *new(uint8) }",
		"ops.Scale: { ... } -> { return *new(int) }",
		"ops.Must: { ... } -> { return *new(int) }",
	} {
		find(t, ms, id)
	}

	// Each operator counts only its own unviable mutants.
	for op, want := range map[string]int{"arithmetic": 2, "drop-call": 1, "erase": 0, "drop-error": 0, "bool": 0} {
		if _, got := generate(t, opsPath, []string{op}); got != want {
			t.Errorf("%s: %d unviable, want %d", op, got, want)
		}
	}
}

// The type-check runs over the whole package, the way the compiler would build it.
//
// Contract: mutate/M3
func TestTheTypeCheckJudgesTheWholePackage(t *testing.T) {
	l, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	units := string(p.Src["ops/units.go"])
	edit := func(old, new string) []byte {
		t.Helper()
		if !strings.Contains(units, old) {
			t.Fatalf("units.go doesn't contain %q", old)
		}
		return []byte(strings.Replace(units, old, new, 1))
	}
	cases := []struct {
		name string
		src  []byte
		want bool
	}{
		{"the file as it is", []byte(units), true},
		{"a changed operator", edit("10 * 2", "10 / 2"), true},
		{"a variable another file uses, removed", edit("var limit = 10 * 2", "var limitless = 10 * 2"), false},
		{"a type error", edit("return len(l.items)", `return "n"`), false},
		{"a syntax error", edit("return len(l.items)", "return len(l.items) +"), false},
		{"a stranded local", edit("b := func() int { return 1 }\n\t\treturn b() + 1", "b := func() int { return 1 }\n\t\treturn 1"), false},
		{"a feature the module's Go version has", edit("p.n++", "for range 3 {\n\t}"), true},
	}
	for _, c := range cases {
		if got := l.Viable(p, "ops/units.go", c.src); got != c.want {
			t.Errorf("%s: Viable = %v, want %v", c.name, got, c.want)
		}
	}
	if l.Viable(p, "ops/none.go", []byte(units)) {
		t.Error("a file the package doesn't have was viable")
	}
	if l.Viable(p, "ops/aaa.go", []byte(units)) || l.Viable(p, "ops/zzz.go", []byte(units)) {
		t.Error("a file name sorting before or after all of the package's was viable")
	}
}

// A module that says go 1.21 builds with Go 1.21's rules, so a mutant using something newer would
// never build and has to be dropped.
//
// Contract: mutate/M3
func TestTheTypeCheckUsesTheModulesGoVersion(t *testing.T) {
	l, pkgs := load(t, "testdata/old", "example.com/old")
	p := pkgs["example.com/old"]
	src := string(p.Src["old.go"])
	newer := strings.Replace(src, "for i := 0; i < n; i++ {", "for i := range n {", 1)
	older := strings.Replace(src, "i < n", "i <= n", 1)
	if newer == src || older == src {
		t.Fatal("old.go doesn't hold the loop the test edits")
	}
	if l.Viable(p, "old.go", []byte(newer)) {
		t.Error("ranging over an int, new in Go 1.22, was viable in a go 1.21 module")
	}
	if !l.Viable(p, "old.go", []byte(older)) {
		t.Error("a change Go 1.21 accepts wasn't viable")
	}
}
