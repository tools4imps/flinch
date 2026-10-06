package tags_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Contract: tags/T1
func TestContractTestsAreTopLevelTestsExamplesAndFuzzTests(t *testing.T) {
	tests, _ := scan(t, "clean")
	var got []string
	for _, ct := range tests {
		got = append(got, fmt.Sprintf("%s %s %s:%d", ct, ct.Kind, ct.File, ct.Line))
	}
	sort.Strings(got)
	// TestMain, Testify, a helper and a method named like a test are no Contract tests, and neither
	// are the tests in contract/helpers, which has no README.md, or in a directory below alpha.
	want := []string{
		"alpha/ExampleOne Example contract/alpha/alpha_test.go:37",
		"alpha/ExampleUnordered Example contract/alpha/alpha_test.go:43",
		"alpha/Example_lower Example contract/alpha/alpha_test.go:52",
		"alpha/FuzzOne Fuzz contract/alpha/alpha_test.go:49",
		"alpha/Test Test contract/alpha/alpha_test.go:23",
		"alpha/TestMore Test contract/alpha/more_test.go:6",
		"alpha/TestOne Test contract/alpha/alpha_test.go:13",
		"alpha/TestTwo Test contract/alpha/alpha_test.go:19",
		"alpha/Test_underscore Test contract/alpha/alpha_test.go:26",
		"beta/TestBeta Test contract/beta/beta_test.go:6",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Contract tests =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, ct := range tests {
		if ct.Dir != "contract/"+ct.Primitive || ct.ImportPath != "example.com/clean/contract/"+ct.Primitive {
			t.Errorf("%s is in %s, %s; want its primitive's directory and package", ct, ct.Dir, ct.ImportPath)
		}
	}
}

// Contract: tags/T2
func TestATagNamesAnObligationFromDirectlyAboveItsTest(t *testing.T) {
	tests, ps := scan(t, "clean")
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
	for name, want := range map[string][]string{
		"TestOne":  {"alpha/A1"},
		"TestTwo":  {"alpha/A1", "alpha/A2"}, // other comment lines can share the group
		"Test":     {"alpha/A3"},             // a tag written twice names its obligation once
		"TestMore": {"alpha/A2"},
		"FuzzOne":  {"alpha/A3"},
		"TestBeta": {"beta/B1"},
	} {
		ct := find(tests, name)
		if ct == nil {
			t.Errorf("no Contract test %s", name)
			continue
		}
		if !reflect.DeepEqual(ct.Obligations, want) {
			t.Errorf("%s names %q, want %q", name, ct.Obligations, want)
		}
	}
}

// Contract: tags/T2
func TestATagSeparatedFromItsTestNamesNothing(t *testing.T) {
	tests, ps := scan(t, "broken")
	ct := find(tests, "TestSeparated")
	if ct == nil {
		t.Fatal("TestSeparated is a Contract test, but Scan lost it")
	}
	if len(ct.Obligations) > 0 {
		t.Errorf("TestSeparated names %q through a tag with a blank line under it, want nothing", ct.Obligations)
	}
	if at(ps, "contract/alpha/alpha_test.go", 18) == 0 {
		t.Errorf("no error for the untagged TestSeparated at line 18: %v", ps)
	}
}

// Contract: tags/T3
func TestEveryObligationIsNamedByATest(t *testing.T) {
	_, ps := scan(t, "broken")
	// Only TestCrossing, in alpha, names beta/B2, and that tag doesn't count.
	if got := in(ps, "contract/alpha/README.md"); !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("errors in alpha's README.md at lines %v, want 5 for alpha/A3: %v", got, ps)
	}
	if got := in(ps, "contract/beta/README.md"); !reflect.DeepEqual(got, []int{4}) {
		t.Errorf("errors in beta's README.md at lines %v, want 4 for beta/B2: %v", got, ps)
	}
	if _, ps := scan(t, "clean"); len(ps) > 0 {
		t.Errorf("every obligation in the clean module is named, but problems = %v", ps)
	}
}

// Contract: tags/T4
func TestEveryContractTestNamesOnlyItsOwnObligations(t *testing.T) {
	tests, ps := scan(t, "broken")
	file := "contract/alpha/alpha_test.go"
	for _, c := range []struct {
		name  string
		lines []int // an error at any of these lines will do
	}{
		{"TestUntagged", []int{8}},
		{"TestCrossing", []int{10, 11}},
		{"TestMisspelled", []int{20, 21}},
		{"ExampleUntagged", []int{33}},
	} {
		ct := find(tests, c.name)
		if ct == nil {
			t.Errorf("no Contract test %s", c.name)
			continue
		}
		if len(ct.Obligations) > 0 {
			t.Errorf("%s names %q, want nothing", c.name, ct.Obligations)
		}
		n := 0
		for _, line := range c.lines {
			n += at(ps, file, line)
		}
		if n == 0 {
			t.Errorf("no error for %s at lines %v: %v", c.name, c.lines, ps)
		}
	}
	// The tests in the clean module each name an obligation of their own primitive.
	tests, ps = scan(t, "clean")
	for _, ct := range tests {
		if len(ct.Obligations) == 0 {
			t.Errorf("%s names nothing", ct)
		}
		for _, ob := range ct.Obligations {
			if !strings.HasPrefix(ob, ct.Primitive+"/") {
				t.Errorf("%s names %s", ct, ob)
			}
		}
	}
	if len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
}

// Contract: tags/T4
func TestAPrimitiveWithNoObligationsStillGetsItsTagsChecked(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/empty\n\ngo 1.25\n")
	write("contract/empty/README.md", "# empty\n\nNo obligations yet.\n")
	write("contract/empty/empty_test.go", "package empty_test\n\nimport \"testing\"\n\n// Contract:empty\nfunc TestNothing(t *testing.T) {}\n")
	_, ps := scan(t, root)
	if at(ps, "contract/empty/empty_test.go", 5)+at(ps, "contract/empty/empty_test.go", 6) == 0 {
		t.Errorf("no error for TestNothing, which names no obligation: %v", ps)
	}
}

// Contract: tags/T5
func TestATagNamingNoObligationIsAnError(t *testing.T) {
	tests, ps := scan(t, "broken")
	file := "contract/alpha/alpha_test.go"
	for name, line := range map[string]int{"TestUnknown": 13, "TestNoSuchPrimitive": 38} {
		if at(ps, file, line) == 0 {
			t.Errorf("no error at line %d for the tag on %s: %v", line, name, ps)
		}
		if ct := find(tests, name); ct == nil || len(ct.Obligations) > 0 {
			t.Errorf("%s = %+v, want a Contract test that names nothing", name, ct)
		}
	}
}

// Contract: tags/T6
func TestATagOutsideTheContractIsAnError(t *testing.T) {
	tests, ps := scan(t, "broken")
	// never_test.go doesn't build under any tags, and its tag is still an error.
	for _, p := range []place{{"internal/x/x_test.go", 5}, {"internal/x/never_test.go", 7}} {
		if at(ps, p.Path, p.Line) != 1 {
			t.Errorf("want one error at %s:%d, got %v", p.Path, p.Line, ps)
		}
	}
	for _, ct := range tests {
		if ct.Primitive != "alpha" && ct.Primitive != "beta" {
			t.Errorf("%s came from outside the Contract", ct)
		}
	}
	// flinch reads tags only in test files, and skips what the go command ignores: a file whose
	// name starts with _ and anything under testdata.
	for _, file := range []string{"internal/x/note.go", "internal/x/_old_test.go", "internal/x/testdata/t_test.go"} {
		if lines := in(ps, file); len(lines) > 0 {
			t.Errorf("errors in %s at lines %v, want none", file, lines)
		}
	}
	// The unit test in the clean module has no tag, so it's fine.
	if _, ps := scan(t, "clean"); len(in(ps, "internal/x/x_test.go")) > 0 {
		t.Errorf("an untagged unit test is an error: %v", ps)
	}
}

// Contract: tags/T9
func TestATagOnNoContractTestIsAnError(t *testing.T) {
	_, ps := scan(t, "broken")
	// A tag with a blank line under it, a tag on a helper function, and a tag in a helper directory
	// inside the Contract all sit on no Contract test.
	for _, p := range []place{
		{"contract/alpha/alpha_test.go", 16},
		{"contract/beta/beta_test.go", 8},
		{"contract/helpers/h_test.go", 5},
	} {
		if at(ps, p.Path, p.Line) != 1 {
			t.Errorf("want one error at %s:%d, got %v", p.Path, p.Line, ps)
		}
	}
	if _, ps := scan(t, "clean"); len(ps) > 0 {
		t.Errorf("every tag in the clean module sits on a Contract test, but problems = %v", ps)
	}
}

// Contract: tags/T7
func TestFlinchContractNeedsNoToolchainAndReportsWhatAFullRunWould(t *testing.T) {
	path := os.Getenv("PATH")
	t.Setenv("PATH", "")
	code, out := run(t, "broken", "contract", "--format", "json")
	static := jsonErrors(t, out)
	if code != 1 {
		t.Errorf("flinch contract exited %d with no toolchain on PATH, want 1", code)
	}
	// These are the errors the other tags obligations promise. The report can't leave one out.
	for _, p := range []place{
		{"contract/alpha/README.md", 5},
		{"contract/alpha/alpha_test.go", 8},
		{"contract/alpha/alpha_test.go", 13},
		{"contract/alpha/alpha_test.go", 18},
		{"contract/alpha/alpha_test.go", 24},
		{"contract/alpha/alpha_test.go", 27},
		{"contract/alpha/alpha_test.go", 33},
		{"contract/alpha/alpha_test.go", 36},
		{"contract/alpha/alpha_test.go", 38},
		{"contract/alpha/alpha_test.go", 16},
		{"contract/beta/beta_test.go", 8},
		{"contract/helpers/h_test.go", 5},
		{"contract/beta/README.md", 4},
		{"internal/x/never_test.go", 7},
		{"internal/x/x_test.go", 5},
	} {
		if at(static, p.Path, p.Line) == 0 {
			t.Errorf("flinch contract reported nothing at %s:%d", p.Path, p.Line)
		}
	}
	t.Setenv("PATH", path)
	code, out = run(t, "broken", "--format", "json", "--jobs", "1")
	if code != 1 {
		t.Errorf("flinch exited %d, want 1", code)
	}
	if full := jsonErrors(t, out); !reflect.DeepEqual(static, full) {
		t.Errorf("flinch contract reported\n%v\na full run reported\n%v", static, full)
	}
}

// Contract: tags/T7
func TestATestFileThatDoesntParseHidesNoOtherError(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                       "module example.com/unparsed\n\ngo 1.25\n",
		"contract/alpha/README.md":     "# alpha\n\n- **A1** one\n",
		"contract/alpha/alpha_test.go": "package alpha_test\n\nimport \"testing\"\n\n// Contract: alpha/A1\nfunc TestOne(t *testing.T) {}\n\nfunc TestTwo(t *testing.T) {}\n",
		"contract/alpha/open_test.go":  "package alpha_test\n\nimport \"testing\"\n\nfunc TestOpen(t *testing.T) {\n",
		"internal/x/x.go":              "package x\n",
		"internal/x/x_test.go":         "package x\n\n// Contract: alpha/A1\n",
		"internal/x/open_test.go":      "package x\n\nfunc TestOpen(t *testing.T) {\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", "")
	code, out := run(t, root, "contract", "--format", "json")
	errs := jsonErrors(t, out)
	if code != 1 || at(errs, "contract/alpha/alpha_test.go", 8) == 0 || at(errs, "internal/x/x_test.go", 3) == 0 {
		t.Errorf("flinch contract exited %d with errors %v, want 1 with the untagged TestTwo and the stray tag", code, errs)
	}
}

// Contract: tags/T8
func TestATaggedExampleWithNoOutputCommentIsAnError(t *testing.T) {
	_, ps := scan(t, "broken")
	file := "contract/alpha/alpha_test.go"
	// ExampleLate's output comment isn't its last comment, and ExampleBodiless has no body at all.
	for name, line := range map[string]int{"ExampleSilent": 24, "ExampleLate": 27, "ExampleBodiless": 36} {
		if at(ps, file, line) != 1 {
			t.Errorf("want one error for %s at line %d, got %v", name, line, ps)
		}
	}
	// An untagged example gets one error for its missing tag, and no second one.
	if n := at(ps, file, 33); n != 1 {
		t.Errorf("%d errors for ExampleUntagged at line 33, want 1", n)
	}
	// Output:, Unordered output: and output: in any case each make go test run the example.
	if _, ps := scan(t, "clean"); len(ps) > 0 {
		t.Errorf("problems = %v, want none", ps)
	}
}
