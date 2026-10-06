package tags

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

func scan(t *testing.T, tags ...string) ([]model.Test, []problem.Problem) {
	t.Helper()
	m, err := source.FindModule(filepath.Join("testdata", "mod"))
	if err != nil {
		t.Fatal(err)
	}
	c, ps, err := contract.Load(m.Root, "")
	if err != nil || len(ps) > 0 {
		t.Fatalf("Load: %v %v", ps, err)
	}
	pkgs, err := source.Packages(m, source.Config{Tags: tags})
	if err != nil {
		t.Fatal(err)
	}
	tests, ps := Scan(m, c, pkgs)
	problem.Sort(ps)
	return tests, ps
}

func names(tests []model.Test) []string {
	var out []string
	for _, t := range tests {
		out = append(out, t.String())
	}
	return out
}

func find(tests []model.Test, name string) *model.Test {
	for i := range tests {
		if tests[i].Name == name {
			return &tests[i]
		}
	}
	return nil
}

func at(ps []problem.Problem, path string, line int) []string {
	var out []string
	for _, p := range ps {
		if p.Path == path && p.Line == line {
			out = append(out, p.Message)
		}
	}
	return out
}

func expect(t *testing.T, ps []problem.Problem, path string, line int, msg string) {
	t.Helper()
	for _, m := range at(ps, path, line) {
		if strings.Contains(m, msg) {
			return
		}
	}
	t.Errorf("no problem %s:%d: %s in\n%v", path, line, msg, ps)
}

const alpha = "contract/alpha/alpha_test.go"

func TestScanFindsTopLevelTestsExamplesAndFuzzTestsOnly(t *testing.T) {
	tests, _ := scan(t)
	want := []string{
		"alpha/TestOne", "alpha/TestReachesAcross", "alpha/TestUntagged", "alpha/TestUnknown",
		"alpha/TestMalformed", "alpha/ExampleNoOutput", "alpha/ExampleWithOutput", "alpha/ExampleUnordered",
		"alpha/ExampleUntaggedNoOutput", "alpha/FuzzOne", "alpha/Test_underscore", "alpha/TestDetached",
		"beta/TestB",
	}
	if got := names(tests); !reflect.DeepEqual(got, want) {
		t.Errorf("tests =\n%q\nwant\n%q", got, want)
	}
}

func TestScanFillsEachTest(t *testing.T) {
	tests, _ := scan(t)
	want := model.Test{
		TestRef:     model.TestRef{Primitive: "alpha", Name: "TestOne"},
		Kind:        "Test",
		Dir:         "contract/alpha",
		ImportPath:  "example.com/t/contract/alpha",
		File:        alpha,
		Line:        14,
		Obligations: []string{"alpha/A1", "alpha/A2"},
	}
	if got := find(tests, "TestOne"); got == nil || !reflect.DeepEqual(*got, want) {
		t.Errorf("TestOne = %+v, want %+v", got, want)
	}
	for name, kind := range map[string]string{"ExampleWithOutput": "Example", "FuzzOne": "Fuzz", "Test_underscore": "Test"} {
		if got := find(tests, name); got == nil || got.Kind != kind {
			t.Errorf("%s = %+v, want kind %s", name, got, kind)
		}
	}
}

func TestScanReportsAnObligationNoTestNames(t *testing.T) {
	_, ps := scan(t)
	expect(t, ps, "contract/alpha/README.md", 5, "no Contract test names alpha/A3")
	if len(at(ps, "contract/alpha/README.md", 3)) > 0 || len(at(ps, "contract/beta/README.md", 3)) > 0 {
		t.Errorf("a named obligation was reported: %v", ps)
	}
}

func TestScanReportsUntaggedTestsAndTagsForOtherPrimitives(t *testing.T) {
	tests, ps := scan(t)
	expect(t, ps, alpha, 20, "TestUntagged is a Contract test with no tag")
	expect(t, ps, alpha, 46, "ExampleUntaggedNoOutput is a Contract test with no tag")
	expect(t, ps, alpha, 17, "TestReachesAcross names beta/B1, but a Contract test names only obligations of its own primitive")
	if got := find(tests, "TestReachesAcross").Obligations; !reflect.DeepEqual(got, []string{"alpha/A1"}) {
		t.Errorf("TestReachesAcross names %q", got)
	}
}

func TestScanReportsUnknownAndMalformedTags(t *testing.T) {
	_, ps := scan(t)
	expect(t, ps, alpha, 22, "alpha/A9 isn't an obligation in contract/alpha/README.md")
	expect(t, ps, alpha, 25, "a tag reads // Contract: <primitive>/<id>")
	if got := at(ps, alpha, 26); len(got) > 0 {
		t.Errorf("TestMalformed, which has a tag, was also called untagged: %q", got)
	}
}

func TestScanReportsTagsThatSitOnNoContractTest(t *testing.T) {
	_, ps := scan(t)
	// Above Testify, TestMain and a method, after a blank line, and inside a body.
	for _, line := range []int{54, 57, 62, 65, 68} {
		expect(t, ps, alpha, line, "this tag sits on no Contract test")
	}
	expect(t, ps, "contract/helpers/h_test.go", 5, "this tag sits outside any primitive's directory")
}

func TestScanReportsTagsOutsideTheContractWhateverTheirBuildTags(t *testing.T) {
	_, ps := scan(t)
	expect(t, ps, "pkg/p_test.go", 5, "a tag outside the Contract counts for nothing")
	expect(t, ps, "pkg/never_test.go", 7, "a tag outside the Contract counts for nothing")
	for _, p := range ps {
		if p.Path == "pkg/p.go" || strings.Contains(p.Path, "testdata") || (p.Path == "pkg/p_test.go" && p.Line != 5) {
			t.Errorf("reported %v", p)
		}
	}
}

func TestScanReportsATaggedExampleWithNoOutput(t *testing.T) {
	_, ps := scan(t)
	expect(t, ps, alpha, 29, "ExampleNoOutput has no // Output: comment")
	for _, line := range []int{35, 41} {
		if got := at(ps, alpha, line); len(got) > 0 {
			t.Errorf("an example with output was reported at line %d: %q", line, got)
		}
	}
}

func TestScanReadsOnlyTestFilesThatBuild(t *testing.T) {
	tests, ps := scan(t, "flinchoff")
	if find(tests, "TestOff") == nil {
		t.Fatal("TestOff is missing under its build tag")
	}
	if len(at(ps, "contract/alpha/README.md", 5)) > 0 {
		t.Errorf("alpha/A3 is named under flinchoff but still reported: %v", ps)
	}
	plain, _ := scan(t)
	if find(plain, "TestOff") != nil {
		t.Error("TestOff appeared without its build tag")
	}
}

func TestTestKindFollowsGoTest(t *testing.T) {
	for name, want := range map[string]string{
		"Test": "Test", "TestX": "Test", "Test_x": "Test", "Test1": "Test", "Testify": "",
		"TestMain": "", "Example": "Example", "ExampleF_suffix": "Example", "Examples": "",
		"Fuzz": "Fuzz", "FuzzParse": "Fuzz", "Fuzzy": "", "helper": "",
	} {
		if got := testKind(name); got != want {
			t.Errorf("testKind(%q) = %q, want %q", name, got, want)
		}
	}
}
