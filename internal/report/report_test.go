package report

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/problem"
)

func mut(primitive, dir, file, unit, top, orig, repl string, line int) model.Mutant {
	id := mutantid.ID{Dir: dir, Unit: unit, Original: orig, Replacement: repl, N: 1}
	return model.Mutant{
		ID: id.String(), Hash: id.Hash(), Primitive: primitive, Dir: dir, File: file,
		Line: line, Col: 2, Unit: unit, Top: top, Original: orig, Replace: repl, Operator: "boundary",
	}
}

var (
	tSkip  = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestSkips"}, Kind: "Test", File: "contract/skip/skip_test.go", Line: 12, Obligations: []string{"skip/K1"}}
	tBlind = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestBlind"}, Kind: "Test", File: "contract/skip/skip_test.go", Line: 40, Obligations: []string{"skip/K2"}}
	tCrash = model.Test{TestRef: model.TestRef{Primitive: "skip", Name: "TestCrash"}, Kind: "Test", File: "contract/skip/skip_test.go", Line: 60, Obligations: []string{"skip/K10"}}
	tGate  = model.Test{TestRef: model.TestRef{Primitive: "gate", Name: "TestGate"}, Kind: "Test", File: "contract/gate/gate_test.go", Line: 8, Obligations: []string{"gate/G1"}}
)

// fixture builds a run with one finding of every kind, from the matrix's own rules.
func fixture(reorder bool) *Report {
	killed := mut("skip", "internal/skip", "internal/skip/skip.go", "Match", "Match", "a", "b", 10)
	lived := mut("skip", "internal/skip", "internal/skip/skip.go", "Match", "Match", "i > 0", "i >= 0", 48)
	unreached := mut("gate", "internal/check", "internal/check/keep.go", "Keep", "Keep", "return nil, err", "return nil, nil", 61)
	erase := mut("gate", "internal/check", "internal/check/judge.go", "median", "median", "{ ... }", "{ return 0 }", 248)
	erase.Erase, erase.Operator = true, "erase"
	skipped := mut("gate", "internal/check", "internal/check/judge.go", "median", "median", "x", "y", 250)
	outside := mut("gate", "internal/check", "internal/check/judge.go", "Judge", "Judge", "c", "d", 20)
	crash := mut("skip", "internal/skip", "internal/skip/skip.go", "Walk", "Walk", "e", "f", 70)
	declared := mut("skip", "internal/skip", "internal/skip/skip.go", "Walk", "Walk", "g", "h", 72)
	brokenM := mut("skip", "internal/skip", "internal/skip/skip.go", "Walk", "Walk", "k", "l", 74)

	mutants := []model.Mutant{killed, lived, unreached, erase, skipped, outside, crash, declared, brokenM}
	tests := []model.Test{tSkip, tBlind, tCrash, tGate}
	obligations := []string{"gate/G1", "gate/G2", "skip/K1", "skip/K2", "skip/K10"}
	if reorder {
		mutants = []model.Mutant{brokenM, declared, crash, outside, skipped, erase, unreached, lived, killed}
		tests = []model.Test{tGate, tCrash, tBlind, tSkip}
	}
	in := matrix.Input{
		Obligations: obligations,
		Tests:       tests,
		Mutants:     mutants,
		Rows: map[string]model.Row{
			killed.Hash:   {Ran: []model.TestRef{tSkip.TestRef, tBlind.TestRef}, Kills: []model.Kill{{Test: tSkip.TestRef, Kind: model.Assertion, Subtests: []string{"TestSkips/b", "TestSkips/a"}}}, Complete: true},
			lived.Hash:    {Ran: []model.TestRef{tSkip.TestRef}, Complete: true},
			erase.Hash:    {Ran: []model.TestRef{tGate.TestRef}, Complete: true},
			outside.Hash:  {Ran: []model.TestRef{tSkip.TestRef}, Kills: []model.Kill{{Test: tSkip.TestRef, Kind: model.Assertion}}, Complete: true},
			crash.Hash:    {Ran: []model.TestRef{tCrash.TestRef}, Kills: []model.Kill{{Test: tCrash.TestRef, Kind: model.Panic}}, Complete: true},
			declared.Hash: {Ran: []model.TestRef{tSkip.TestRef}, Complete: true},
			brokenM.Hash:  {Ran: []model.TestRef{tSkip.TestRef}, Kills: []model.Kill{{Test: tSkip.TestRef, Kind: model.Assertion}}, Complete: true},
		},
		Reached: map[string][]model.TestRef{skipped.Hash: {tGate.TestRef}},
		Declarations: []matrix.Declaration{
			{Kind: "equivalent", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 5, ID: declared.ID, Reason: "Same output"},
			{Kind: "unpromised", Primitive: "skip", Path: "contract/skip/mutants.md", Line: 9, ID: brokenM.ID, Reason: "Free"},
		},
		Full: true,
	}
	return &Report{
		Version: "v0.1.0", GoVersion: "go1.26.1", GOOS: "darwin", GOARCH: "arm64", Tags: []string{"integration"},
		Problems: []problem.Problem{
			{Path: "contract/skip/skip_test.go", Line: 3, Message: "the tag names skip/K9, which the Contract doesn't have"},
			{Path: "contract/gate/README.md", Line: 7, Message: "gate/G2 appears twice"},
		},
		Matrix:   matrix.Compute(in),
		Outside:  []string{"internal/atomicfile", "cmd/qualm"},
		Exit:     1,
		Unviable: 3,
		Timing:   map[string]time.Duration{"mutate": 1500 * time.Millisecond, "prove": 2 * time.Second},
	}
}

func textOf(t *testing.T, r *Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := Text(&b, r); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestTextSummary(t *testing.T) {
	out := textOf(t, fixture(false))
	for _, want := range []string{
		"flinch: 9 mutants in covered code, plus 3 unviable that never built\n",
		"  4 killed, 3 unheld (1 lived, 1 unreached, 1 erased), 1 skipped, 0 no verdict\n",
		"  1 declared: 1 equivalent, 0 unpromised, 0 by wildcard\n",
		"  score 57.1%, killed / (killed + unheld) = 4 / 7, for information only\n",
		"  5 obligations: 2 hold code, 3 hollow, 1 held only by crashes, 3 hold no mutant alone\n",
		"    gate  G1 0, G2 0\n",
		"    skip  K1 3, K2 0, K10 1\n",
		"  4 Contract tests: 2 blind\n",
		"  full run\n",
		"  flinch v0.1.0, go1.26.1 darwin/arm64, tags integration\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q\n%s", want, out)
		}
	}
}

func TestTextFindingsInOrderAndEachSaysWhatClearsIt(t *testing.T) {
	out := textOf(t, fixture(false))
	t.Log("\n" + out)
	order := []string{
		"Contract errors, each cleared by fixing the file at the line it names (2)",
		"  contract/gate/README.md:7: gate/G2 appears twice",
		"  contract/skip/skip_test.go:3: the tag names skip/K9",
		"Unheld (3)",
		"  gate  internal/check/judge.go:248  ",
		"    erased: run by gate/G1 (TestGate), and none noticed it returning zero values from its first line; 1 finer mutant skipped",
		"    to clear: write the obligation that needs this function, delete it, or declare it unpromised in contract/gate/mutants.md",
		"  gate  internal/check/keep.go:61  ",
		"    internal/check.Keep: return nil, err -> return nil, nil",
		"    unreached: no Contract test runs this line",
		"    to clear: write the promise that needs this code, delete the code, or declare it unpromised in contract/gate/mutants.md",
		"  skip  internal/skip/skip.go:48  ",
		"    lived: run by skip/K1 (TestSkips), which passed",
		"    to clear: strengthen those tests, write the obligation this change breaks, or declare it in contract/skip/mutants.md",
		"Broken declarations, each cleared by deleting its line (1)",
		"  contract/skip/mutants.md:9: skip/TestSkips kills this mutant, so the Contract does promise this behavior; delete the line",
		"Hollow obligations (3)",
		"  gate/G1  1 test ran against 2 mutants and killed none",
		"  gate/G2  no Contract test names it",
		"  skip/K2  1 test ran against 1 mutant and killed none",
		"Held only by crashes, which never fails the build (1)",
		"  skip/K10  every kill by its tests was a panic or a timeout, none an assertion",
		"    to clear: give its tests assertions about results",
		"Held only from outside, which never fails the build (1)",
		"  gate  internal/check/judge.go:20  ",
		"    killed only by skip/TestSkips (skip/K1)",
		"    to clear: give gate an obligation whose test fails against this change",
		"Blind Contract tests (2)",
		"  contract/gate/gate_test.go:8  TestGate (gate/G1)",
		"  contract/skip/skip_test.go:40  TestBlind (skip/K2)",
		"    ran in 1 complete row of undeclared mutants and killed none",
		"    to clear: give it an assertion that can fail, or delete it",
		"Outside the Contract, which never fails the build (2)",
		"  cmd/qualm\n  internal/atomicfile\n",
		"  to bring one in, name it in a primitive's covers block",
		"\nexit 1\n",
	}
	at := 0
	for _, want := range order {
		i := strings.Index(out[at:], want)
		if i < 0 {
			t.Fatalf("after byte %d the report lacks %q", at, want)
		}
		at += i + len(want)
	}
	if !strings.HasSuffix(out, "\nexit 1\n") {
		t.Error("the report should end with the exit code")
	}
	// Every mutant finding carries the line that says what clears it, before the next finding or
	// the end of its section.
	head := regexp.MustCompile(`^  \S+  \S+:\d+  [0-9a-f]{12}$`)
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if !head.MatchString(l) {
			continue
		}
		cleared := false
		for _, next := range lines[i+1:] {
			if next == "" || head.MatchString(next) {
				break
			}
			cleared = cleared || strings.HasPrefix(next, "    to clear: ")
		}
		if !cleared {
			t.Errorf("finding %q says nothing about clearing it", l)
		}
	}
}

func TestTextWithoutMatrix(t *testing.T) {
	r := &Report{Version: "v0.1.0", GoVersion: "go1.26.1", GOOS: "linux", GOARCH: "amd64",
		Problems: []problem.Problem{{Path: "contract/a/README.md", Line: 2, Message: "bad"}},
		Since:    "main", Exit: 1}
	out := textOf(t, r)
	for _, want := range []string{
		"flinch: stopped at Contract errors, before generating any mutant\n",
		"  scoped run, --since main\n",
		"  flinch v0.1.0, go1.26.1 linux/amd64, tags none\n",
		"  contract/a/README.md:2: bad\n",
		"\nexit 1\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q\n%s", want, out)
		}
	}
}

func TestTextUndecided(t *testing.T) {
	r := fixture(false)
	r.Undecided, r.Exit = "skip/TestSkips failed on clean code", 2
	out := textOf(t, r)
	if !strings.HasSuffix(out, "\nflinch couldn't decide: skip/TestSkips failed on clean code\n\nexit 2\n") {
		t.Errorf("an undecided run should say why just before the exit code:\n%s", out)
	}
}

func jsonOf(t *testing.T, r *Report) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := JSON(&b, r); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

var timingObject = regexp.MustCompile(`(?s)"timing": \{.*?\}`)

func TestJSONIsByteIdenticalApartFromTiming(t *testing.T) {
	a, b := fixture(false), fixture(true)
	b.Timing = map[string]time.Duration{"mutate": time.Minute}
	ja, jb := jsonOf(t, a), jsonOf(t, b)
	sa := timingObject.ReplaceAll(ja, []byte(`"timing": {}`))
	sb := timingObject.ReplaceAll(jb, []byte(`"timing": {}`))
	if !bytes.Equal(sa, sb) {
		t.Errorf("the same run in another order gave different JSON:\n%s\n---\n%s", sa, sb)
	}
	if !bytes.Equal(ja, jsonOf(t, a)) {
		t.Error("the same report gave different bytes twice")
	}
}

func TestJSONShape(t *testing.T) {
	raw := jsonOf(t, fixture(false))
	if bytes.Contains(raw, []byte("null,")) && !bytes.Contains(raw, []byte(`"declaration": null`)) {
		t.Errorf("a list came out null:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte(" -> ")) {
		t.Error("ids should keep their arrow unescaped")
	}
	var doc struct {
		SchemaVersion string `json:"schema_version"`
		Summary       struct {
			Mutants  int      `json:"mutants"`
			Unheld   int      `json:"unheld"`
			Score    *float64 `json:"score"`
			Declared struct {
				Equivalent int `json:"equivalent"`
			} `json:"declared"`
		} `json:"summary"`
		Build struct {
			Go     string   `json:"go"`
			GOOS   string   `json:"goos"`
			GOARCH string   `json:"goarch"`
			Tags   []string `json:"tags"`
			Flinch string   `json:"flinch"`
		} `json:"build"`
		ContractErrors []struct {
			Path string `json:"path"`
			Line int    `json:"line"`
		} `json:"contract_errors"`
		Mutants []struct {
			ID       string `json:"id"`
			File     string `json:"file"`
			Line     int    `json:"line"`
			Status   string `json:"status"`
			KilledBy []struct {
				Test        string   `json:"test"`
				Subtests    []string `json:"subtests"`
				Obligations []string `json:"obligations"`
				Kind        string   `json:"kind"`
			} `json:"killed_by"`
			RanBy       []string `json:"ran_by"`
			Declaration *struct {
				Kind   string `json:"kind"`
				Path   string `json:"path"`
				Line   int    `json:"line"`
				Reason string `json:"reason"`
			} `json:"declaration"`
		} `json:"mutants"`
		Obligations []struct {
			ID        string   `json:"id"`
			SoleHolds []string `json:"sole_holds"`
			Hollow    bool     `json:"hollow"`
		} `json:"obligations"`
		Tests []struct {
			Name  string `json:"name"`
			Blind bool   `json:"blind"`
		} `json:"tests"`
		Outside []string           `json:"outside"`
		Timing  map[string]float64 `json:"timing"`
		Exit    int                `json:"exit"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != "1.0" || doc.Summary.Mutants != 9 || doc.Summary.Unheld != 3 ||
		doc.Summary.Score == nil || *doc.Summary.Score != 57.1 || doc.Summary.Declared.Equivalent != 1 {
		t.Errorf("summary = %+v", doc.Summary)
	}
	if doc.Build.Go != "go1.26.1" || doc.Build.GOOS != "darwin" || doc.Build.Flinch != "v0.1.0" || len(doc.Build.Tags) != 1 {
		t.Errorf("build = %+v", doc.Build)
	}
	if len(doc.ContractErrors) != 2 || doc.ContractErrors[0].Path != "contract/gate/README.md" {
		t.Errorf("contract errors should be sorted by path, got %+v", doc.ContractErrors)
	}
	for i := 1; i < len(doc.Mutants); i++ {
		a, b := doc.Mutants[i-1], doc.Mutants[i]
		if a.File > b.File || a.File == b.File && a.Line > b.Line {
			t.Errorf("mutants out of order at %d: %s:%d before %s:%d", i, a.File, a.Line, b.File, b.Line)
		}
	}
	first := doc.Mutants[0]
	if first.File != "internal/check/judge.go" || first.Line != 20 || first.Status != "killed" ||
		len(first.KilledBy) != 1 || first.KilledBy[0].Test != "skip/TestSkips" || first.KilledBy[0].Kind != "assertion" ||
		first.KilledBy[0].Obligations[0] != "skip/K1" {
		t.Errorf("first mutant = %+v", first)
	}
	var withSubtests, withDeclaration bool
	for _, m := range doc.Mutants {
		if len(m.KilledBy) > 0 && len(m.KilledBy[0].Subtests) == 2 && m.KilledBy[0].Subtests[0] == "TestSkips/a" {
			withSubtests = true
		}
		if m.Declaration != nil && m.Declaration.Kind == "equivalent" && m.Declaration.Reason == "Same output" && m.Status == "declared" {
			withDeclaration = true
		}
	}
	if !withSubtests || !withDeclaration {
		t.Errorf("want sorted subtests and a declaration with its reason; subtests %v, declaration %v", withSubtests, withDeclaration)
	}
	var ids []string
	for _, o := range doc.Obligations {
		ids = append(ids, o.ID)
	}
	if strings.Join(ids, " ") != "gate/G1 gate/G2 skip/K1 skip/K2 skip/K10" {
		t.Errorf("obligations in order %v", ids)
	}
	if doc.Outside[0] != "cmd/qualm" || doc.Timing["mutate"] != 1.5 || doc.Exit != 1 {
		t.Errorf("outside %v, timing %v, exit %d", doc.Outside, doc.Timing, doc.Exit)
	}
}

func TestJSONWithoutMatrixHasEmptyLists(t *testing.T) {
	raw := jsonOf(t, &Report{Exit: 1, Problems: []problem.Problem{{Path: "a", Line: 1, Message: "m"}}})
	for _, key := range []string{`"mutants": []`, `"obligations": []`, `"tests": []`, `"outside": []`, `"broken_declarations": []`, `"tags": []`, `"score": null`} {
		if !bytes.Contains(raw, []byte(key)) {
			t.Errorf("JSON lacks %s:\n%s", key, raw)
		}
	}
}
