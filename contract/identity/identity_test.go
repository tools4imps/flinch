package identity_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/mutantid"
)

// Contract: identity/I1
func TestAnIDNamesTheUnitThenTheChange(t *testing.T) {
	got := ids(dryRun(t, fixture("idmod")))
	for _, want := range []string{
		"internal/shapes.limit: base + 1 -> base - 1",
		"internal/shapes.Box.Grow: b.n++ -> b.n--",
		"internal/shapes.List.Empty: len(l.items) == 0 -> len(l.items) != 0",
		"internal/shapes.Nest.func1: n < 9 -> n <= 9",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no mutant %q among\n%s", want, strings.Join(got, "\n"))
		}
	}
	form := regexp.MustCompile(`^internal/shapes\.[A-Za-z0-9_.]+: \S.* -> \S.*$`)
	for _, id := range got {
		if !form.MatchString(id) {
			t.Errorf("%q isn't <dir>.<unit>: <original> -> <replacement>", id)
		}
	}
}

// Contract: identity/I1
func TestTheSameChangeAgainEndsWithItsCount(t *testing.T) {
	var both []string
	for _, id := range ids(dryRun(t, fixture("idmod"))) {
		if unit(id, "internal/shapes") == "Both" && !strings.HasPrefix(id, "internal/shapes.Both: {") {
			both = append(both, id)
		}
	}
	sort.Strings(both)
	want := []string{
		"internal/shapes.Both: x > 0 -> x >= 0",
		"internal/shapes.Both: x > 0 -> x >= 0 #2",
		"internal/shapes.Both: x > 0 -> x >= 0 #3",
		"internal/shapes.Both: x-- -> x++",
		"internal/shapes.Both: x-- -> x++ #2",
		"internal/shapes.Both: x-- -> x++ #3",
	}
	if !reflect.DeepEqual(both, want) {
		t.Errorf("Both's mutants =\n%s\nwant\n%s", strings.Join(both, "\n"), strings.Join(want, "\n"))
	}
}

// Contract: identity/I1
func TestAnIDReadsBackAsTheSameID(t *testing.T) {
	for _, id := range []mutantid.ID{
		{Dir: "internal/check", Unit: "Keep", Original: "err != nil", Replacement: "err == nil", N: 1},
		{Dir: "internal/check", Unit: "judge.settled", Original: "return nil, err", Replacement: "return nil, nil", N: 2},
		{Dir: "cmd", Unit: "main.func3", Original: "a: b", Replacement: "a -> b", N: 12},
		{Dir: ".", Unit: "init.0", Original: "x", Replacement: "y", N: 1},
	} {
		s := id.String()
		p, err := mutantid.Parse(s)
		if err != nil {
			t.Errorf("Parse(%q): %v", s, err)
			continue
		}
		if p.Wildcard || p.Dir != id.Dir || p.Unit != id.Unit || p.ID != id {
			t.Errorf("Parse(%q) = %+v, want %+v", s, p, id)
		}
	}
	if got := (mutantid.ID{Dir: "internal/x", Unit: "F", Original: "a", Replacement: "b", N: 1}).String(); got != "internal/x.F: a -> b" {
		t.Errorf("the first occurrence prints as %q, with no count", got)
	}
	if got := (mutantid.ID{Dir: ".", Unit: "F", Original: "a", Replacement: "b", N: 2}).String(); got != "..F: a -> b #2" {
		t.Errorf("the root package's id is %q, want ..F: a -> b #2", got)
	}
	// Every mutant flinch lists reads back to what it printed.
	for _, id := range ids(dryRun(t, fixture("idmod"))) {
		p, err := mutantid.Parse(id)
		if err != nil || p.ID.String() != id || p.Dir != "internal/shapes" {
			t.Errorf("Parse(%q) = %+v, %v", id, p, err)
		}
	}
}

// Contract: identity/I1
func TestWhatIsntAnIDDoesntReadAsOne(t *testing.T) {
	for _, line := range []string{
		"internal/x.F",            // no change at all
		"internal/x.F: a",         // no arrow
		"internal/x.F: a->b",      // the arrow needs its spaces
		"internal/xF: a -> b",     // no unit after the directory
		"internal/x/: a -> b",     // the same
		".F: a -> b",              // no directory before the unit
		"internal/.F: a -> b",     // a directory doesn't end in a slash
		"internal/x.: a -> b",     // an empty unit
		"..: a -> b",              // the root package with no unit
		"internal/x.F: a -> b #1", // the first occurrence has no count
		"internal/x.F: a -> b #0", // nor does anything before it
		"internal/x.F:a -> b",     // the colon needs its space
		"internal/x.F: -> b",      // no original
		": internal/x.F: a -> b",  // nothing before the first colon
	} {
		if p, err := mutantid.Parse(line); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", line, p)
		}
	}
	// A whole unit's wildcard has no change in it, so it can't come back as an id.
	if p, err := mutantid.Parse("internal/x.F: *"); err == nil && !p.Wildcard {
		t.Errorf("Parse(internal/x.F: *) = %+v, an id with no change", p)
	}
}

// Contract: identity/I2
func TestUnitsAreNamedTheWayTheCompilerNamesThem(t *testing.T) {
	where := map[string]string{}
	for _, m := range dryRun(t, fixture("idmod")) {
		u := unit(m.ID, "internal/shapes")
		if _, seen := where[u]; !seen {
			where[u] = filepath.Base(m.File) + ":" + strconv.Itoa(m.Line)
		}
	}
	want := map[string]string{
		"limit":       "a.go:6",  // a package-level variable
		"check.func1": "a.go:8",  // the function literal in a variable's initializer
		"init.0":      "a.go:12", // init functions count across files in name order
		"Box.Grow":    "a.go:18", // a pointer receiver loses its *
		"List.Empty":  "a.go:24", // a generic receiver loses its type parameters
		"Both":        "a.go:27",
		"Nest":        "a.go:41",
		"Nest.func2":  "a.go:43", // literals count in source order, nested ones too
		"Nest.func1":  "a.go:44",
		"Spread":      "a.go:49",
		"Spaced":      "a.go:55",
		"init.1":      "b.go:3",
		"init.2":      "b.go:5",
	}
	if !reflect.DeepEqual(where, want) {
		t.Errorf("units =\n%v\nwant\n%v", where, want)
	}
}

// Contract: identity/I3
func TestOriginalAndReplacementCollapseWhitespace(t *testing.T) {
	got := ids(dryRun(t, fixture("idmod")))
	for _, want := range []string{
		// The expression spans two lines and an indent.
		"internal/shapes.Spread: a > 0 && b > 0 -> a > 0 || b > 0",
		// Whitespace inside a literal collapses too.
		`internal/shapes.Spaced: s == "a b" -> s != "a b"`,
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no mutant %q among\n%s", want, strings.Join(got, "\n"))
		}
	}
	for _, id := range got {
		if strings.ContainsAny(id, "\t\n\r") || strings.Contains(id, "  ") {
			t.Errorf("%q holds whitespace that isn't one space", id)
		}
	}
	if got := mutantid.Collapse(" \t a  +\n\tb \r\n"); got != "a + b" {
		t.Errorf("Collapse = %q, want %q", got, "a + b")
	}
	p, err := mutantid.Parse("  internal/x.F:   a  &&\tb   ->  a ||  b   #2 ")
	if want := (mutantid.ID{Dir: "internal/x", Unit: "F", Original: "a && b", Replacement: "a || b", N: 2}); err != nil || p.ID != want {
		t.Errorf("Parse with runs of whitespace = %+v, %v; want %+v", p.ID, err, want)
	}
}

// Contract: identity/I4
func TestAnEditOutsideAUnitLeavesItsIDsAlone(t *testing.T) {
	before := dryRun(t, fixture("idmod"))
	dir := copyTree(t, fixture("idmod"))
	a := filepath.Join(dir, "internal", "shapes", "a.go")
	// New lines above everything move every line in the file. A new function makes the same
	// changes as Both, and Spread's own body changes.
	edit(t, a, "const base = 2\n", "// Lines that push the rest down.\n\n\nconst base = 2\n\nfunc Again(x int) int {\n\tif x > 0 {\n\t\tx--\n\t}\n\treturn x\n}\n")
	edit(t, a, "return a > 0 &&\n\t\tb > 0", "return a > 1 &&\n\t\tb > 0 && a != b")
	after := dryRun(t, dir)

	changed := map[string]bool{"Spread": true, "Again": true}
	keep := func(ms []listed) []string {
		var out []string
		for _, m := range ms {
			if !changed[unit(m.ID, "internal/shapes")] {
				out = append(out, m.Hash+"  "+m.ID)
			}
		}
		sort.Strings(out)
		return out
	}
	if b, a := keep(before), keep(after); !reflect.DeepEqual(b, a) {
		t.Errorf("ids before the edit:\n%s\nafter:\n%s", strings.Join(b, "\n"), strings.Join(a, "\n"))
	}
	moved := false
	for i := range before {
		for _, m := range after {
			if m.ID == before[i].ID && m.Line != before[i].Line {
				moved = true
			}
		}
	}
	if !moved {
		t.Error("the edit moved no mutant, so it proves nothing")
	}
	if !slices.Contains(ids(after), "internal/shapes.Again: x > 0 -> x >= 0") {
		t.Errorf("the edit didn't add Again's mutants: %v", ids(after))
	}
}

// Contract: identity/I5
func TestJSONCarriesEachIDsHash(t *testing.T) {
	_, out := run(t, fixture("hashmod"), "--format", "json", "--jobs", "1")
	var rep struct {
		Mutants []struct {
			ID     string `json:"id"`
			Hash   string `json:"hash"`
			Status string `json:"status"`
		} `json:"mutants"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("the JSON report doesn't parse: %v\n%s", err, out)
	}
	if len(rep.Mutants) < 4 {
		t.Fatalf("the JSON report lists %d mutants, want at least 4:\n%s", len(rep.Mutants), out)
	}
	for _, m := range rep.Mutants {
		if m.Hash != sha12(m.ID) {
			t.Errorf("%q has hash %q, want %q", m.ID, m.Hash, sha12(m.ID))
		}
	}
}

// Contract: identity/I5
func TestTheTextReportCarriesEachIDsHash(t *testing.T) {
	_, out := run(t, fixture("hashmod"), "--jobs", "1")
	lines := strings.Split(out, "\n")
	head := regexp.MustCompile(`^  \S+  \S+:\d+  ([0-9a-f]+)$`)
	found := 0
	for i, line := range lines {
		m := head.FindStringSubmatch(line)
		if m == nil || i+1 == len(lines) {
			continue
		}
		found++
		id := strings.TrimSpace(lines[i+1])
		if m[1] != sha12(id) {
			t.Errorf("%q has hash %q in the text report, want %q", id, m[1], sha12(id))
		}
	}
	if found < 3 {
		t.Errorf("the text report shows %d unheld mutants with a hash, want 3:\n%s", found, out)
	}
}
