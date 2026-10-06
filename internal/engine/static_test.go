package engine

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckReportsEveryContractErrorSorted(t *testing.T) {
	st, err := Check(Options{Dir: filepath.Join("testdata", "mod", "lib")})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range st.Problems {
		got = append(got, p.String())
	}
	want := []string{
		"contract/broken/README.md:6: nope names no package in the module; ",
		"contract/lib/README.md:4: no Contract test names lib/L2; ",
		"contract/lib/lib_test.go:16: TestUntagged is a Contract test with no tag; ",
		"contract/lib/mutants.md:1: this unpromised block has no heading above it; ",
		"other/other_test.go:5: a tag outside the Contract counts for nothing; ",
	}
	if len(got) != len(want) {
		t.Fatalf("problems =\n%s\nwant %d", strings.Join(got, "\n"), len(want))
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("problem %d = %q, want it to start %q", i, got[i], want[i])
		}
	}
}

func TestCheckKeepsWhatAFullRunNeeds(t *testing.T) {
	st, err := Check(Options{Dir: filepath.Join("testdata", "mod")})
	if err != nil {
		t.Fatal(err)
	}
	if st.Module.Path != "example.com/e" || st.Contract.Dir != "contract" {
		t.Errorf("module %q, Contract %q", st.Module.Path, st.Contract.Dir)
	}
	if st.Parsed["lib"] == nil || st.Parsed["contract/lib"] != nil {
		t.Errorf("Parsed holds %v", keys(st.Parsed))
	}
	if owner, ok := st.Ownership.Owner("lib", "Add"); !ok || owner != "lib" {
		t.Errorf("Owner(lib, Add) = %q, %v", owner, ok)
	}
	if got := st.Ownership.Outside(); !reflect.DeepEqual(got, []string{"other"}) {
		t.Errorf("Outside = %q", got)
	}
	if d := st.Declarations.Wildcard("lib", "helper"); d == nil || d.Line != 2 {
		t.Errorf("Wildcard(lib, helper) = %+v", d)
	}
	var tests []string
	for _, tt := range st.Tests {
		tests = append(tests, tt.String()+" "+strings.Join(tt.Obligations, ","))
	}
	want := []string{"broken/TestB broken/B1", "lib/TestAdd lib/L1", "lib/TestUntagged "}
	if !reflect.DeepEqual(tests, want) {
		t.Errorf("tests = %q, want %q", tests, want)
	}
}

func TestCheckReportsAMissingContractDirectory(t *testing.T) {
	st, err := Check(Options{Dir: filepath.Join("testdata", "mod"), Contract: "spec"})
	if err != nil {
		t.Fatal(err)
	}
	// The tags under contract/ now sit outside the Contract, so they're problems too.
	missing, outside := false, 0
	for _, p := range st.Problems {
		missing = missing || (p.Path == "spec" && strings.Contains(p.Message, "there's no Contract here"))
		if strings.Contains(p.Message, "a tag outside the Contract") {
			outside++
		}
	}
	if !missing || outside != 3 {
		t.Errorf("problems = %v", st.Problems)
	}
}

func TestCheckNeedsAModule(t *testing.T) {
	if _, err := Check(Options{Dir: t.TempDir()}); err == nil {
		t.Fatal("Check ran without a go.mod")
	}
}

func TestCheckRunsWithNoGoToolchain(t *testing.T) {
	t.Setenv("PATH", "")
	st, err := Check(Options{Dir: filepath.Join("testdata", "mod")})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Problems) != 5 {
		t.Errorf("found %d problems with no toolchain, want 5: %v", len(st.Problems), st.Problems)
	}
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
