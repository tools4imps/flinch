package contract_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/contract"
)

// Contract: contract/C1
func TestPrimitivesAreTheDirectSubdirectoriesWithAReadme(t *testing.T) {
	c, ps := load(t, fixture(t, "layout"), "")
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
	if c.Dir != "contract" {
		t.Errorf("Dir = %q, want contract", c.Dir)
	}
	// helpers has no README.md, sub sits a level down, and the rest have names flinch skips.
	if got := names(c); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Errorf("primitives = %q, want alpha and beta", got)
	}
	alpha := c.Primitive("alpha")
	if alpha == nil {
		t.Fatal("no primitive named alpha")
	}
	if alpha.Name != "alpha" || alpha.Dir != "contract/alpha" || alpha.Readme != "contract/alpha/README.md" {
		t.Errorf("alpha = %+v", alpha)
	}
	if want := []contract.Obligation{{ID: "A1", Line: 8}}; !reflect.DeepEqual(alpha.Obligations, want) {
		t.Errorf("alpha's obligations = %v, want %v", alpha.Obligations, want)
	}
	// Files whose names start with . or _ are skipped like directories, so their blocks count nowhere.
	if len(alpha.Covers) > 0 || len(alpha.Declarations) > 0 {
		t.Errorf("alpha read a skipped file: covers %v, declarations %v", alpha.Covers, alpha.Declarations)
	}
	if beta := c.Primitive("beta"); beta == nil || beta.Dir != "contract/beta" {
		t.Errorf("beta = %+v", beta)
	}
	for _, name := range []string{"helpers", "sub", "testdata", "vendor", ".hidden", "_old", "gamma"} {
		if c.Primitive(name) != nil {
			t.Errorf("%s became a primitive", name)
		}
	}
}

// Contract: contract/C1
func TestTheContractFlagNamesAnotherDirectory(t *testing.T) {
	root := fixture(t, "layout")
	for _, dir := range []string{"spec", filepath.Join(root, "spec")} {
		c, ps := load(t, root, dir)
		if len(ps) > 0 {
			t.Errorf("Load(%q): problems = %v", dir, ps)
		}
		if c.Dir != "spec" || !reflect.DeepEqual(names(c), []string{"gamma"}) {
			t.Errorf("Load(%q) = %q with %q, want spec with gamma", dir, c.Dir, names(c))
		}
		if g := c.Primitive("gamma"); g == nil || g.Dir != "spec/gamma" || g.Readme != "spec/gamma/README.md" {
			t.Errorf("Load(%q): gamma = %+v", dir, g)
		}
	}
	// The names flinch skips are names inside the Contract. The directory --contract names can have one.
	c, ps := load(t, root, ".flinch")
	if len(ps) > 0 || !reflect.DeepEqual(names(c), []string{"delta"}) {
		t.Errorf("Load(.flinch) = %q with problems %v, want delta", names(c), ps)
	}
}

// Contract: contract/C1
func TestNoContractWhereFlinchLooksIsAnError(t *testing.T) {
	root := fixture(t, "layout")
	for _, dir := range []string{"nowhere", "go.mod"} {
		c, ps := load(t, root, dir)
		if len(c.Primitives) > 0 {
			t.Errorf("Load(%q) found primitives %q", dir, names(c))
		}
		if !reflect.DeepEqual(paths(ps), []string{dir}) {
			t.Errorf("Load(%q): problems = %v, want one at %s", dir, ps, dir)
		}
	}
	// A path through a file isn't a directory either.
	if _, ps := load(t, root, "go.mod/contract"); !reflect.DeepEqual(paths(ps), []string{"go.mod/contract"}) {
		t.Errorf("Load(go.mod/contract): problems = %v, want one at go.mod/contract", ps)
	}
	for _, dir := range []string{"nowhere", "go.mod", "go.mod/contract"} {
		if code, out := run(t, root, "contract", "--contract", dir); code != 1 || !strings.Contains(out, dir+": ") {
			t.Errorf("flinch contract --contract %s exited %d, want 1 with a Contract error:\n%s", dir, code, out)
		}
	}
	// Nor is a path through a symlink that points at itself.
	looped := tree(t, map[string]string{"loop": "-> loop"})
	if _, ps := load(t, looped, "loop/contract"); !reflect.DeepEqual(paths(ps), []string{"loop/contract"}) {
		t.Errorf("Load(loop/contract): problems = %v, want one at loop/contract", ps)
	}
	// Without a contract directory, the default finds nothing either.
	_, ps := load(t, fixture(t, "layout/spec"), "")
	if !reflect.DeepEqual(paths(ps), []string{"contract"}) {
		t.Errorf("problems = %v, want one at contract", ps)
	}
}

// Contract: contract/C1
func TestSkippedNamesAreNeverChecked(t *testing.T) {
	bad := "fine\n\xff\n"
	root := tree(t, map[string]string{
		"outside.md":                       "# outside\n",
		"contract/alpha/README.md":         "# alpha\n\n- **A1** holds\n",
		"contract/alpha/.DS_Store":         bad,
		"contract/alpha/_scratch.go":       "package scratch\n",
		"contract/alpha/.link.md":          "-> ../../outside.md",
		"contract/alpha/testdata/x.go":     "package x\n",
		"contract/alpha/testdata/bad.txt":  bad,
		"contract/alpha/testdata/link.md":  "-> ../../../outside.md",
		"contract/vendor/README.md":        bad,
		"contract/.cache/README.md":        "-> ../../outside.md",
		"contract/_old/README.md":          "# old\n\n- **O1** retired\n",
		"contract/_old/stray.go":           "package old\n",
		"contract/testdata/mod/README.md":  bad,
		"contract/alpha/sub/_ignored.md":   bad,
		"contract/alpha/sub/notes/.x.md":   bad,
		"contract/alpha/sub/notes/keep.md": "# fine\n",
	})
	c, ps := load(t, root, "")
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none: every one of them sits under a skipped name", ps)
	}
	if !reflect.DeepEqual(names(c), []string{"alpha"}) {
		t.Errorf("primitives = %q, want alpha", names(c))
	}
}

// Contract: contract/C2
func TestObligationLines(t *testing.T) {
	c, _ := load(t, fixture(t, "obligations"), "")
	want := []contract.Obligation{
		{ID: "A1", Line: 5},
		{ID: "AB12", Line: 6},
		{ID: "A8", Line: 14},
		{ID: "A10", Line: 16},
		{ID: "A11", Line: 17},
	}
	if got := c.Primitive("alpha").Obligations; !reflect.DeepEqual(got, want) {
		t.Errorf("alpha's obligations = %v\nwant %v", got, want)
	}
	if got := c.Primitive("beta").Obligations; !reflect.DeepEqual(got, []contract.Obligation{{ID: "A1", Line: 3}}) {
		t.Errorf("beta's obligations = %v, want A1 at line 3", got)
	}
}

// Contract: contract/C2
func TestTwoObligationsWithOneFullIDAreAnError(t *testing.T) {
	_, ps := load(t, fixture(t, "obligations"), "")
	// The second alpha/A1 is the error. beta/A1 is another full id, so it's fine.
	if want := []place{{"contract/alpha/README.md", 7}}; !reflect.DeepEqual(places(ps), want) {
		t.Errorf("problems = %v, want one at contract/alpha/README.md:7", ps)
	}
}

// Contract: contract/C3
func TestFencesHideTheLinesInsideThem(t *testing.T) {
	c, ps := load(t, fixture(t, "fences"), "")
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
	want := []contract.Obligation{{ID: "F1", Line: 3}, {ID: "F3", Line: 21}, {ID: "F4", Line: 25}, {ID: "F9", Line: 46}, {ID: "F7", Line: 52}}
	if got := c.Primitive("alpha").Obligations; !reflect.DeepEqual(got, want) {
		t.Errorf("obligations = %v\nwant %v", got, want)
	}
}

// Contract: contract/C3
func TestFencesCloseOnTheSameCharacterAtLeastAsManyTimes(t *testing.T) {
	c, _ := load(t, fixture(t, "fences"), "")
	var got []string
	for _, r := range c.Primitive("alpha").Covers {
		got = append(got, fmt.Sprintf("%d %s", r.Line, r.Text))
	}
	want := []string{
		// A tilde fence of four ignores three tildes and any backticks, and five tildes close it.
		"10 internal/a", "11 ~~~", "12 `````", "13 internal/b",
		// The info string is the first word, and up to three spaces of indent still make a fence.
		"17 internal/c",
		// A fence inside a comment is dead, so internal/d never shows up. One on the line after the
		// comment closes is live, and so is one right after an empty comment.
		"38 internal/m", "43 internal/k",
		// Four backticks ignore three, and a line of backticks with words after it closes nothing.
		"55 internal/e", "56 ```", "57 ```` with words", "58 internal/f",
		// Covers and coverslike aren't covers, and spaces before the info string don't matter.
		"70 internal/i",
		// A fence that never closes runs to the end of the file.
		"74 internal/j", "75 - **F8** inside a fence that never closes",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("covers lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Contract: contract/C4
func TestCoversBlocksCountOnlyInTheReadme(t *testing.T) {
	c, ps := load(t, fixture(t, "blocks"), "")
	want := []contract.Ref{{Text: "internal/a", Line: 6}, {Text: "internal/b.F", Line: 8}}
	if got := c.Primitive("alpha").Covers; !reflect.DeepEqual(got, want) {
		t.Errorf("alpha's covers = %v, want %v", got, want)
	}
	if got := c.Primitive("beta").Covers; !reflect.DeepEqual(got, []contract.Ref{{Text: "internal/c", Line: 6}}) {
		t.Errorf("beta's covers = %v", got)
	}
	for _, at := range []place{{"contract/alpha/mutants.md", 17}, {"contract/alpha/notes.md", 3}} {
		if msg := message(ps, at.Path, at.Line); !strings.Contains(msg, "contract/alpha/README.md") {
			t.Errorf("%s:%d: problem %q, want one that names contract/alpha/README.md", at.Path, at.Line, msg)
		}
	}
}

// Contract: contract/C4
func TestDeclarationBlocksCountOnlyInMutantsMd(t *testing.T) {
	c, ps := load(t, fixture(t, "blocks"), "")
	alpha := c.Primitive("alpha")
	var got []string
	for _, d := range alpha.Declarations {
		got = append(got, fmt.Sprintf("%s:%d %s %s", d.Path, d.Line, d.Kind, d.Text))
	}
	want := []string{
		"contract/alpha/mutants.md:6 equivalent internal/a.F: a -> b",
		"contract/alpha/mutants.md:8 equivalent internal/a.F: c -> d",
		"contract/alpha/mutants.md:14 unpromised internal/a.G: *",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("declarations =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if alpha.Mutants != "contract/alpha/mutants.md" {
		t.Errorf("alpha's mutants.md = %q", alpha.Mutants)
	}
	if beta := c.Primitive("beta"); beta.Mutants != "" || len(beta.Declarations) > 0 {
		t.Errorf("beta has no mutants.md, but Mutants = %q and declarations %v", beta.Mutants, beta.Declarations)
	}
	for _, at := range []place{{"contract/alpha/README.md", 11}, {"contract/alpha/README.md", 15}, {"contract/alpha/notes.md", 7}} {
		if msg := message(ps, at.Path, at.Line); !strings.Contains(msg, "contract/alpha/mutants.md") {
			t.Errorf("%s:%d: problem %q, want one that names contract/alpha/mutants.md", at.Path, at.Line, msg)
		}
	}
	// notes.txt isn't Markdown and sub/more.md sits a level down, so neither is searched.
	if len(ps) != 5 {
		t.Errorf("problems = %v, want five misplaced blocks", ps)
	}
}

// Contract: contract/C7
func TestEachDeclarationCarriesItsBlocksReason(t *testing.T) {
	c, ps := load(t, fixture(t, "reasons"), "")
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
	got := map[int]string{}
	for _, d := range c.Primitive("alpha").Declarations {
		got[d.Line] = d.Reason
	}
	want := map[int]string{
		4:  "Declared mutants",
		13: "Sorting again changes nothing Some prose over two lines.",
		14: "Sorting again changes nothing Some prose over two lines.",
		// The prose before the last block explained that block, so it starts over here.
		20: "Sorting again changes nothing Prose after a block.",
		27: "Left open",
		// A text block ends More prose. as well.
		38: "Also open",
	}
	for line, reason := range want {
		if got[line] != reason {
			t.Errorf("the declaration at line %d has reason %q, want %q", line, got[line], reason)
		}
	}
	// A line of dashes with no prose right above it underlines nothing, so Also open is still the
	// nearest heading.
	if !strings.HasPrefix(got[44], "Also open") {
		t.Errorf("the declaration at line 44 has reason %q, want one under Also open", got[44])
	}
	// beta's file opens with a line of dashes, which underlines nothing either.
	beta := c.Primitive("beta").Declarations
	if len(beta) != 1 || beta[0].Reason != "" {
		t.Errorf("beta's declarations = %+v, want one with no reason, since no heading is above it", beta)
	}
}

// Contract: contract/C5
func TestSymlinksInsideTheContractAreErrors(t *testing.T) {
	root := tree(t, map[string]string{
		"outside.md":               "# outside\n\n- **B1** from outside\n",
		"elsewhere/notes.md":       "# notes\n",
		"contract/alpha/README.md": "# alpha\n\n- **A1** holds\n",
		"contract/alpha/link.md":   "-> ../../outside.md",
		"contract/alpha/linked":    "-> ../../elsewhere",
		"contract/beta/README.md":  "-> ../../outside.md",
		"contract/helpers/h.md":    "-> ../../outside.md",
	})
	_, ps := load(t, root, "")
	got := paths(ps)
	sort.Strings(got)
	want := []string{"contract/alpha/link.md", "contract/alpha/linked", "contract/beta/README.md", "contract/helpers/h.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems at %q, want one at each symlink: %q", got, want)
	}
}

// Contract: contract/C5
func TestASymlinkedContractDirectoryIsAnError(t *testing.T) {
	root := tree(t, map[string]string{
		"real/alpha/README.md": "# alpha\n\n- **A1** holds\n",
		"contract":             "-> real",
	})
	c, ps := load(t, root, "")
	if !reflect.DeepEqual(paths(ps), []string{"contract"}) {
		t.Errorf("problems = %v, want one at contract", ps)
	}
	if len(c.Primitives) > 0 {
		t.Errorf("primitives = %q, read through the symlink", names(c))
	}
}

// Contract: contract/C5
func TestFilesThatArentUTF8AreErrors(t *testing.T) {
	root := tree(t, map[string]string{
		"contract/alpha/README.md": "# alpha\n\n- **A1** holds\n\xfe\n",
		"contract/alpha/notes.md":  "one\ntwo\nthree \xff\nfour\n",
		// U+FFFD written out is valid UTF-8, so the first bad byte is on line 2.
		"contract/alpha/data.txt":    "� fine\n\xc3(\n",
		"contract/alpha/fine.txt":    "� is a character like any other\n",
		"contract/helpers/blob":      "\x80",
		"contract/beta/README.md":    "# beta\n\n- **B1** holds\n",
		"contract/beta/beta_test.go": "package beta_test\n\n// caf\xe9\n",
	})
	_, ps := load(t, root, "")
	got := places(ps)
	sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
	want := []place{
		{"contract/alpha/README.md", 4},
		{"contract/alpha/data.txt", 2},
		{"contract/alpha/notes.md", 3},
		{"contract/beta/beta_test.go", 3},
		{"contract/helpers/blob", 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("problems at %v\nwant %v", got, want)
	}
}

// Contract: contract/C5
func TestGoFilesInAPrimitiveMustBeTests(t *testing.T) {
	root := tree(t, map[string]string{
		"contract/alpha/README.md":     "# alpha\n\n- **A1** holds\n",
		"contract/alpha/stray.go":      "package alpha\n",
		"contract/alpha/alpha_test.go": "package alpha_test\n",
		"contract/alpha/notes.txt":     "not Go\n",
		"contract/alpha/sub/s.go":      "package sub\n",
		"contract/helpers/h.go":        "package helpers\n",
		"contract/root.go":             "package contract\n",
	})
	_, ps := load(t, root, "")
	if !reflect.DeepEqual(paths(ps), []string{"contract/alpha/stray.go"}) {
		t.Errorf("problems = %v, want one at contract/alpha/stray.go", ps)
	}
}

// Contract: contract/C5
func TestAFileFlinchCantReadIsNeverPassedAsText(t *testing.T) {
	// flinch can't tell whether a file it can't read is UTF-8, so it must say something about it:
	// a Contract error, or an error that stops the run.
	for _, locked := range []string{"contract/alpha/locked.txt", "contract/alpha/sealed"} {
		root := tree(t, map[string]string{
			"contract/alpha/README.md":       "# alpha\n\n- **A1** holds\n",
			"contract/alpha/locked.txt":      "locked\n",
			"contract/alpha/sealed/notes.md": "# sealed\n",
		})
		p := filepath.Join(root, filepath.FromSlash(locked))
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(p, 0o755) })
		_, errFile := os.ReadFile(p)
		_, errDir := os.ReadDir(p)
		if errFile == nil || errDir == nil {
			t.Skip("this user can read what has no permissions")
		}
		_, ps, err := contract.Load(root, "")
		if err == nil && !slices.Contains(paths(ps), locked) {
			t.Errorf("Load passed %s, which it can't read: problems %v", locked, ps)
		}
	}
	// The same goes for a Contract directory flinch isn't allowed to look into.
	root := tree(t, map[string]string{"locked/contract/alpha/README.md": "# alpha\n\n- **A1** holds\n"})
	p := filepath.Join(root, "locked")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o755) })
	if _, ps, err := contract.Load(root, "locked/contract"); err == nil && !slices.Contains(paths(ps), "locked/contract") {
		t.Errorf("Load passed locked/contract, which it can't look into: problems %v", ps)
	}
}

// Contract: contract/C6
func TestErrorsPrintAsPathLineMessage(t *testing.T) {
	root := fixture(t, "errors")
	code, out := run(t, root, "contract", "--format", "json")
	errs := jsonErrors(t, out)
	if code != 1 || len(errs) == 0 {
		t.Fatalf("flinch contract exited %d with errors %v, want 1 with errors", code, errs)
	}
	code, out = run(t, root, "contract")
	if code != 1 {
		t.Errorf("flinch contract exited %d, want 1", code)
	}
	var printed []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  contract/") {
			printed = append(printed, strings.TrimSpace(line))
		}
	}
	// The stray Go file is an error about the whole file, so it has no line to print.
	var want []string
	whole := 0
	for _, e := range errs {
		if e.Line == 0 {
			want = append(want, fmt.Sprintf("%s: %s", e.Path, e.Message))
			whole++
			continue
		}
		want = append(want, fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message))
	}
	if whole != 1 {
		t.Errorf("%d errors about a whole file, want 1 for contract/beta/stray.go: %v", whole, errs)
	}
	if !reflect.DeepEqual(printed, want) {
		t.Errorf("printed\n%s\nwant\n%s", strings.Join(printed, "\n"), strings.Join(want, "\n"))
	}
}

// Contract: contract/C6
func TestARunReportsEveryErrorSortedByPathAndLine(t *testing.T) {
	_, out := run(t, fixture(t, "errors"), "contract", "--format", "json")
	// Each check finds its own errors in its own order: the loader, then the tags, then the covers
	// blocks. The report puts them all in one list by path and line.
	want := []place{
		{"contract/alpha/README.md", 4},
		{"contract/alpha/README.md", 7},
		{"contract/alpha/README.md", 11},
		{"contract/alpha/alpha_test.go", 8},
		{"contract/alpha/alpha_test.go", 10},
		{"contract/beta/README.md", 4},
		{"contract/beta/stray.go", 0},
	}
	if got := places(jsonErrors(t, out)); !reflect.DeepEqual(got, want) {
		t.Errorf("errors at\n%v\nwant\n%v", got, want)
	}
}
