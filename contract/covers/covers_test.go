package covers_test

import (
	"go/build"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/covers"
)

// Contract: covers/V1
func TestALineNamesThePackageInItsDirectory(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"x"}, "r": {"."}})
	r.clean(t)
	r.expect(t, []owned{
		{"x", "F", "p"},
		{"x", "T.M", "p"},
		{"x", "Box.Get", "p"},
		{"x", "V", "p"},
		{"x", "init.0", "p"},
		{".", "R", "r"},
		{"x/y", "Y", ""},
		{"xy", "XY", ""},
	})
	if got, want := sorted(r.own.Covered()), []string{".", "x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Covered() = %q, want %q", got, want)
	}

	// The module's packages are its directories that hold Go files, the ones the go command would
	// list for ./..., in an order the Contract leaves open.
	var dirs []string
	for _, p := range r.pkgs {
		dirs = append(dirs, p.Dir)
	}
	want := []string{".", "contract/helpers", "gen", "out", "tree/leaf", "tree/leaf/deep", "x", "x-a", "x/y", "x/y/z", "xy"}
	if !slices.Equal(sorted(dirs), want) {
		t.Errorf("the module's packages are %q, want %q", dirs, want)
	}
}

// Contract: covers/V1
func TestATreeLineAddsEveryPackageBelowIt(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"x/..."}, "q": {"tree/..."}})
	r.clean(t)
	r.expect(t, []owned{
		{"x", "F", "p"},
		{"x/y", "Y", "p"},
		{"x/y/z", "Z", "p"},
		{"xy", "XY", ""},
		{"x-a", "XA", ""},
		{"tree/leaf", "Leaf", "q"},
		{"tree/leaf/deep", "Deep", "q"},
		{".", "R", ""},
	})
	want := []string{"tree/leaf", "tree/leaf/deep", "x", "x/y", "x/y/z"}
	if got := sorted(r.own.Covered()); !reflect.DeepEqual(got, want) {
		t.Errorf("Covered() = %q, want %q", got, want)
	}

	all := resolve(t, map[string][]string{"p": {"./..."}})
	all.clean(t)
	all.expect(t, []owned{{".", "R", "p"}, {"xy", "XY", "p"}, {"x/y/z", "Z", "p"}, {"out", "Out", "p"}})
}

// Contract: covers/V1
func TestLinesAreRelativeToTheModuleRootWhereverFlinchStarts(t *testing.T) {
	root := copyFixture(t)
	writeFile(t, root, "contract/p/README.md", readme("p", []string{".", "x/y"}))
	writeFile(t, root, "contract/p/p_test.go", contractTest("p"))

	r := resolveAt(t, filepath.Join(root, "x", "y", "z"))
	if r.module.Root != root || r.module.Path != "example.com/cov" {
		t.Errorf("the module found from x/y/z is %+v, want example.com/cov at %s", r.module, root)
	}
	r.clean(t)
	r.expect(t, []owned{{".", "R", "p"}, {"x/y", "Y", "p"}, {"x/y/z", "Z", ""}})
	if p := r.pkg(t, "x/y"); !slices.Equal(p.GoFiles, []string{"x/y/y.go"}) {
		t.Errorf("x/y's files are %q, want them named from the module root", p.GoFiles)
	}
	if p := r.pkg(t, "."); !slices.Equal(p.GoFiles, []string{"root.go"}) {
		t.Errorf("the root package's files are %q, want [root.go]", p.GoFiles)
	}

	// A dry run from below the root lists the mutants of exactly the packages the lines name.
	code, out, errs := run(t, filepath.Join(root, "x", "y", "z"), "--dry-run")
	if code != 0 {
		t.Fatalf("flinch --dry-run exited %d\n%s%s", code, out, errs)
	}
	files := map[string]bool{}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[:len(lines)-1] {
		file, _, _ := strings.Cut(line, ":")
		files[file] = true
	}
	if want := map[string]bool{"root.go": true, "x/y/y.go": true}; !reflect.DeepEqual(files, want) {
		t.Errorf("the dry run listed mutants in %v, want only root.go and x/y/y.go\n%s", files, out)
	}
}

// Contract: covers/V1
func TestAPackageHoldsTheFilesTheGoCommandWouldBuild(t *testing.T) {
	root := copyFixture(t)
	elsewhere := filepath.Join(t.TempDir(), "linked.go")
	writeFile(t, filepath.Dir(elsewhere), "linked.go", "package y\n\n// Linked sits outside the module.\nfunc Linked() int { return 1 }\n")
	symlink(t, elsewhere, filepath.Join(root, "x", "y", "linked.go"))
	symlink(t, filepath.Join(root, "missing.go"), filepath.Join(root, "x", "y", "broken.go"))
	symlink(t, filepath.Join(root, "tree"), filepath.Join(root, "x", "y", "dir.go"))
	writeFile(t, root, "x/y/notes.txt", "not Go\n")
	writeFile(t, root, "x/y/other_windows.go", "package y\n\nfunc Windows() int { return 2 }\n")

	writeFile(t, root, "contract/p/README.md", readme("p", []string{"x/y"}))
	r := resolveAt(t, root)
	r.clean(t)
	want := []string{"x/y/linked.go", "x/y/y.go"}
	if build.Default.GOOS == "windows" {
		want = []string{"x/y/linked.go", "x/y/other_windows.go", "x/y/y.go"}
	}
	if p := r.pkg(t, "x/y"); !slices.Equal(p.GoFiles, want) {
		t.Errorf("x/y's files are %q, want %q", p.GoFiles, want)
	}
	r.expect(t, []owned{{"x/y", "Linked", "p"}, {"x/y", "Y", "p"}})
}

// Contract: covers/V2
func TestANameCoversAFunctionOrATypeWithAllItsMethods(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"x.F", "x.T", "x.Box"}})
	r.clean(t)
	r.expect(t, []owned{
		{"x", "F", "p"},
		{"x", "T.M", "p"},
		{"x", "T.P", "p"},
		{"x", "Box.Get", "p"},
		{"x", "G", ""},
		{"x", "U.N", ""},
		{"x", "V", ""},
		{"x", "init.0", ""},
	})

	root := resolve(t, map[string][]string{"p": {"..R"}})
	root.clean(t)
	root.expect(t, []owned{{".", "R", "p"}})
}

// Contract: covers/V2
func TestANameCoversAVariableSpecOrTheInitFunctions(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"x.Second", "x.V", "x.init"}})
	r.clean(t)
	r.expect(t, []owned{
		{"x", "First", "p"}, // Second's spec, whose unit is named after First
		{"x", "V", "p"},
		{"x", "init.0", "p"},
		{"x", "B", ""},
		{"x", "F", ""},
	})

	one := resolve(t, map[string][]string{"p": {"x.init.0", "x.B"}})
	one.clean(t)
	one.expect(t, []owned{{"x", "init.0", "p"}, {"x", "B", "p"}, {"x", "V", ""}})
}

// Contract: covers/V2
func TestATypeAndMethodNameOneMethod(t *testing.T) {
	value := resolve(t, map[string][]string{"p": {"x.U.N"}})
	value.clean(t)
	value.expect(t, []owned{{"x", "U.N", "p"}, {"x", "U.O", ""}})

	pointer := resolve(t, map[string][]string{"p": {"x.U.O", "x.Box.Get"}})
	pointer.clean(t)
	pointer.expect(t, []owned{{"x", "U.O", "p"}, {"x", "U.N", ""}, {"x", "Box.Get", "p"}})
}

// Contract: covers/V3
func TestTheMostSpecificReferenceWins(t *testing.T) {
	r := resolve(t, map[string][]string{
		"a": {"x/..."},
		"b": {"x"},
		"c": {"x.T", "x.F", "x.V", "x.B", "x.Second", "x.init"},
		"d": {"x.T.M", "x.init.0"},
	})
	r.clean(t)
	r.expect(t, []owned{
		{"x/y", "Y", "a"},     // only the tree names it
		{"x", "G", "b"},       // a package beats a tree
		{"x", "U.N", "b"},     // and takes the methods no other reference names
		{"x", "F", "c"},       // a function beats a package
		{"x", "T.P", "c"},     // so does a type, with all its methods
		{"x", "V", "c"},       // and a variable named for itself
		{"x", "B", "c"},       // whose spec may start with "_"
		{"x", "First", "c"},   // and naming any name in a spec names the spec
		{"x", "init.0", "c"},  // init names the init functions, and init.0 is no more specific
		{"x", "T.M", "d"},     // a method beats its type
		{"x", "Box.Get", "b"}, // the package keeps what nothing else names
		{"x/y/z", "Z", "a"},   // the tree reaches every level
		{"xy", "XY", ""},      // and no further
		{"tree/leaf", "Leaf", ""},
	})
}

// Contract: covers/V3
func TestATieGoesToThePrimitiveFirstByName(t *testing.T) {
	r := resolve(t, map[string][]string{
		"m": {"x/y/..."},
		"n": {"x/...", "x/y", "x.T.M", "x.F"},
		"o": {"x/y", "x.T.M", "x.F", "x/y/z/..."},
	})
	r.clean(t)
	r.expect(t, []owned{
		{"x/y", "Y", "n"},
		{"x", "T.M", "n"},
		{"x", "F", "n"},
		{"x/y/z", "Z", "m"},
		{"x", "G", "n"},
	})
}

// Contract: covers/V4
func TestAReferenceThatNamesNothingIsAnError(t *testing.T) {
	refs := []struct {
		text  string
		names bool
	}{
		{"x", true},
		{"nope", false},
		{"nope/...", false},
		{"x.Nope", false},
		{"x.T.Nope", false},
		{"x.Nope.M", false},
		{"x.F.M", false},
		{"xy.init", false},
		{"x.init.1", false},
		{"..Nope", false},
		{"testdata/td", false},
		{"vendor/v", false},
		{"_under", false},
		{".dot", false},
		{"nested", false},
		{"nested/...", false},
		{"nested/inner", false},
		{"x.Second", true},
		{"tree/...", true},
		{"..R", true},
		{"x.W", true},
		{"x.B", true},
		{"x.Gen", true},
		{"x.GT.GM", true},
		{"x.init", true},
		{"x.init.0", true},
		{"x.U.O", true},
	}
	var lines []string
	var want []int
	for i, ref := range refs {
		lines = append(lines, ref.text)
		if !ref.names {
			want = append(want, firstRef+i)
		}
	}
	r := resolve(t, map[string][]string{"p": lines})
	var got []int
	for _, p := range r.problems {
		if p.Path != "contract/p/README.md" || p.Message == "" {
			t.Errorf("problem %v should name the README.md and say what's wrong", p)
		}
		got = append(got, p.Line)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		for _, line := range got {
			if !slices.Contains(want, line) {
				t.Errorf("%q names something but was reported", refs[line-firstRef].text)
			}
		}
		for _, line := range want {
			if !slices.Contains(got, line) {
				t.Errorf("%q names nothing but wasn't reported", refs[line-firstRef].text)
			}
		}
	}
}

// Contract: covers/V5
func TestTestsGeneratedFilesAndCgoAreNeverCovered(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"./..."}, "q": {"x.Gen", "x.GT", "x.GT.GM", "gen.Only"}})
	r.clean(t)
	r.expect(t, []owned{
		{"x", "F", "p"},
		{"x", "Gen", ""},
		{"x", "GT.GM", ""},
		{"x", "Cgo", ""},
		{"x", "helper", ""},
		{"x", "TestX", ""},
		{"gen", "Only", ""},
	})
	if got := r.own.Covered(); slices.Contains(got, "gen") {
		t.Errorf("Covered() = %q, but gen holds only generated code", got)
	}

	x := r.pkg(t, "x")
	if !slices.Equal(x.Mutable, []string{"x/x.go"}) {
		t.Errorf("x's mutable files are %q, want only x/x.go", x.Mutable)
	}
	if !slices.Equal(x.TestFiles, []string{"x/x_test.go"}) {
		t.Errorf("x's test files are %q, want x/x_test.go", x.TestFiles)
	}
	// The go command builds a file that imports "C" only when cgo is on, and flinch reads the
	// package the same way. Either way the file is never covered.
	files := []string{"x/gen.go", "x/x.go"}
	if build.Default.CgoEnabled {
		files = []string{"x/cgo.go", "x/gen.go", "x/x.go"}
	}
	if !slices.Equal(x.GoFiles, files) {
		t.Errorf("x's files are %q, want %q", x.GoFiles, files)
	}
	cgo := resolve(t, map[string][]string{"p": {"x.Cgo"}})
	if named := len(cgo.problems) == 0; named != build.Default.CgoEnabled {
		t.Errorf("with cgo on %v, x.Cgo gave problems %v", build.Default.CgoEnabled, cgo.problems)
	}
	cgo.expect(t, []owned{{"x", "Cgo", ""}})
}

// Contract: covers/V6
func TestPackagesNoReferenceCoversAreOutsideTheContract(t *testing.T) {
	r := resolve(t, map[string][]string{"p": {"x", "gen"}})
	r.clean(t)
	// gen is named, though it has nothing flinch may mutate, and contract/helpers is the Contract's.
	want := []string{".", "out", "tree/leaf", "tree/leaf/deep", "x-a", "x/y", "x/y/z", "xy"}
	if got := r.own.Outside(); !reflect.DeepEqual(got, want) {
		t.Errorf("Outside() = %q, want %q", got, want)
	}

	// The list is sorted by directory whatever order the packages arrive in.
	reversed := slices.Clone(r.pkgs)
	slices.Reverse(reversed)
	own, _ := covers.Resolve(r.contract, reversed, r.parsed)
	if got := own.Outside(); !reflect.DeepEqual(got, want) {
		t.Errorf("Outside() from packages in reverse = %q, want %q", got, want)
	}
}

// Contract: covers/V6
func TestPackagesOutsideTheContractNeverFailTheBuild(t *testing.T) {
	root := copyFixture(t)
	writeFile(t, root, "contract/p/README.md", readme("p", []string{"x", "gen"}))
	writeFile(t, root, "contract/p/p_test.go", contractTest("p"))
	code, out, errs := run(t, root, "contract")
	if code != 0 {
		t.Fatalf("flinch contract exited %d with packages outside the Contract\n%s%s", code, out, errs)
	}
	for _, dir := range []string{"out", "xy", "x/y/z"} {
		if !strings.Contains(out, "\n  "+dir+"\n") {
			t.Errorf("the report doesn't list %s as outside the Contract\n%s", dir, out)
		}
	}
	if strings.Contains(out, "contract/helpers") {
		t.Errorf("the report lists the Contract's own helpers as outside it\n%s", out)
	}
}
