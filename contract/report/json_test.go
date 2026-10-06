package report_test

import (
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/report"
)

// Contract: report/P4
func TestJSONCarriesItsSchemaVersion(t *testing.T) {
	if !regexp.MustCompile(`^1\.[0-9]+$`).MatchString(report.SchemaVersion) {
		t.Fatalf("schema version %q isn't 1.<minor>", report.SchemaVersion)
	}
	for _, r := range []*report.Report{rich(), {Exit: 1, Problems: []problem.Problem{{Path: "contract/a/README.md", Line: 1, Message: "m"}}}} {
		out := jsonOf(t, r)
		// A consumer reads the keys it knows and ignores the rest.
		var known struct {
			SchemaVersion string `json:"schema_version"`
			Exit          int    `json:"exit"`
		}
		if err := json.Unmarshal([]byte(out), &known); err != nil {
			t.Fatalf("the report isn't JSON: %v\n%s", err, out)
		}
		if known.SchemaVersion != report.SchemaVersion || known.Exit != r.Exit {
			t.Errorf("schema_version %q and exit %d, want %q and %d", known.SchemaVersion, known.Exit, report.SchemaVersion, r.Exit)
		}
	}
}

// in is matrix input with every kind of mutant. When shuffled, every list in it is reversed: the
// same run, as a different job count or a different map order might hand it over.
func in(shuffled bool) matrix.Input {
	a, b, d := ref("calc", "TestA"), ref("calc", "TestB"), ref("dial", "TestD")
	m := func(unit, change string, line int) model.Mutant {
		top, _, _ := strings.Cut(unit, ".")
		id := mutantid.ID{Dir: "calc", Unit: unit, Original: change, Replacement: change + "'", N: 1}
		return model.Mutant{ID: id.String(), Hash: id.Hash(), Primitive: "calc", Dir: "calc", File: "calc/calc.go",
			Line: line, Col: 2, Unit: unit, Top: top, Original: change, Replace: change + "'", Operator: "arithmetic"}
	}
	killed, lived, unreached := m("Add", "a + b", 3), m("Add", "a - b", 4), m("Sub", "a - b", 8)
	declared, wild, nv := m("Mul", "a * b", 12), m("Div.func1", "a / b", 20), m("Mod", "a % b", 30)
	sameLine := m("Add", "b + c", 3) // the killed mutant's line, further along it
	sameLine.Col = 9
	sameCol := m("Add", "c + d", 3) // the killed mutant's line and column; its id sorts after
	erase := m("Big", "{ ... }", 40)
	erase.Erase = true
	skipped := m("Big", "x + 1", 41)

	tests := []model.Test{
		{TestRef: a, Kind: "Test", File: "contract/calc/calc_test.go", Line: 10, Obligations: []string{"calc/C1"}},
		{TestRef: b, Kind: "Test", File: "contract/calc/calc_test.go", Line: 20, Obligations: []string{"calc/C2", "calc/C1"}},
		{TestRef: d, Kind: "Example", File: "contract/dial/dial_test.go", Line: 5, Obligations: []string{"dial/D1"}},
	}
	rows := map[string]model.Row{
		killed.Hash: {Ran: []model.TestRef{a, b, d}, Complete: true, Kills: []model.Kill{
			{Test: a, Kind: model.Assertion, Subtests: []string{"TestA/x", "TestA/a"}},
			{Test: b, Kind: model.Assertion},
			{Test: d, Kind: model.Panic},
		}},
		lived.Hash:    {Ran: []model.TestRef{a, b, d}, Complete: true},
		declared.Hash: {Ran: []model.TestRef{a, b}, Complete: true},
		wild.Hash:     {Ran: []model.TestRef{b, a}, Complete: true},
		nv.Hash:       {Verdict: "it wouldn't build"},
		erase.Hash:    {Ran: []model.TestRef{a, d}, Complete: true},
	}
	reached := map[string][]model.TestRef{
		killed.Hash: {a, b, d}, lived.Hash: {a, b, d}, declared.Hash: {a, b}, wild.Hash: {a, b},
		nv.Hash: {d, a}, erase.Hash: {a, d}, skipped.Hash: {d, a},
	}
	input := matrix.Input{
		Obligations: []string{"calc/C1", "calc/C10", "calc/C2", "dial/D1"}, // as the READMEs list them
		Tests:       tests,
		Mutants:     []model.Mutant{sameLine, sameCol, killed, lived, unreached, declared, wild, nv, erase, skipped},
		Rows:        rows,
		Reached:     reached,
		Declarations: []matrix.Declaration{
			{Kind: "equivalent", Primitive: "calc", Path: "contract/calc/mutants.md", Line: 3, Reason: "## Same\n\nSame.", ID: declared.ID},
			{Kind: "unpromised", Primitive: "calc", Path: "contract/calc/mutants.md", Line: 9, Reason: "## Open\n\nOpen.", Wildcard: true, Dir: "calc", Unit: "Div"},
		},
		Full: true,
	}
	if !shuffled {
		return input
	}
	reverse := func(s []model.TestRef) []model.TestRef { s = slices.Clone(s); slices.Reverse(s); return s }
	// Obligations come in Contract order, which no run changes, so they stay as they are.
	input.Tests = slices.Clone(input.Tests)
	slices.Reverse(input.Tests)
	for i := range input.Tests {
		input.Tests[i].Obligations = slices.Clone(input.Tests[i].Obligations)
		slices.Reverse(input.Tests[i].Obligations)
	}
	input.Mutants = slices.Clone(input.Mutants)
	slices.Reverse(input.Mutants)
	input.Declarations = slices.Clone(input.Declarations)
	slices.Reverse(input.Declarations)
	input.Rows, input.Reached = map[string]model.Row{}, map[string][]model.TestRef{}
	for h, r := range rows {
		r.Ran = reverse(r.Ran)
		r.Kills = slices.Clone(r.Kills)
		slices.Reverse(r.Kills)
		for i := range r.Kills {
			r.Kills[i].Subtests = slices.Clone(r.Kills[i].Subtests)
			slices.Reverse(r.Kills[i].Subtests)
		}
		input.Rows[h] = r
	}
	for h, ts := range reached {
		input.Reached[h] = reverse(ts)
	}
	return input
}

func runReport(shuffled bool, timing time.Duration) *report.Report {
	r := &report.Report{
		Version: "0.1.0", GoVersion: "go1.26.1", GOOS: "linux", GOARCH: "amd64",
		Tags:     []string{"b", "a"},
		Problems: []problem.Problem{{Path: "contract/z/README.md", Line: 2, Message: "z"}, {Path: "contract/a/README.md", Line: 7, Message: "a"}},
		Matrix:   matrix.Compute(in(shuffled)),
		Outside:  []string{"internal/z", "internal/a", "cmd/m"},
		Exit:     2,
		Timing:   map[string]time.Duration{"total": timing, "plan": timing / 4},
	}
	if shuffled {
		slices.Reverse(r.Tags)
		slices.Reverse(r.Problems)
		slices.Reverse(r.Outside)
	}
	return r
}

// untimed cuts the timing object, the report's last key, from a JSON report.
func untimed(t *testing.T, out string) string {
	t.Helper()
	i := strings.Index(out, `"timing":`)
	if i < 0 {
		t.Fatalf("the report has no timing object:\n%s", out)
	}
	return out[:i]
}

type jsonKill struct {
	Test        string
	Subtests    []string
	Obligations []string
	Kind        string
}

type jsonMutant struct {
	ID, Hash, Status string
	KilledBy         []jsonKill `json:"killed_by"`
	RanBy            []string   `json:"ran_by"`
	Obligations      []string
	Declaration      *struct {
		Kind, Path, Reason string
		Line               int
		Wildcard           bool
	}
	OutsideOnly bool `json:"outside_only"`
	Verdict     string
}

// Contract: report/P5
func TestJSONListsEveryMutantAndIsTheSameForTheSameRun(t *testing.T) {
	first := jsonOf(t, runReport(false, 1500*time.Millisecond))
	again := jsonOf(t, runReport(false, 9*time.Second))
	shuffled := jsonOf(t, runReport(true, 3*time.Second))
	if untimed(t, again) != untimed(t, first) {
		t.Errorf("two renderings of one run differ outside timing:\n%s\n%s", first, again)
	}
	if untimed(t, shuffled) != untimed(t, first) {
		t.Errorf("the same run handed over in another order gives other bytes:\n%s\n%s", first, shuffled)
	}

	var rep struct {
		Mutants        []jsonMutant
		ContractErrors []struct{ Path string } `json:"contract_errors"`
		Outside        []string
		Build          struct{ Tags []string }
		Tests          []struct{ Name, Kind string }
		Obligations    []struct {
			ID    string
			Tests []string
		}
		Timing map[string]float64
	}
	if err := json.Unmarshal([]byte(first), &rep); err != nil {
		t.Fatalf("the report isn't JSON: %v\n%s", err, first)
	}
	if len(rep.Mutants) != 10 {
		t.Fatalf("the report lists %d mutants, want all 10:\n%s", len(rep.Mutants), first)
	}
	byID := map[string]jsonMutant{}
	var order []string
	for _, m := range rep.Mutants {
		byID[m.ID] = m
		order = append(order, m.ID)
	}
	wantOrder := []string{
		"calc.Add: a + b -> a + b'", "calc.Add: c + d -> c + d'", "calc.Add: b + c -> b + c'", "calc.Add: a - b -> a - b'", "calc.Sub: a - b -> a - b'", "calc.Mul: a * b -> a * b'",
		"calc.Div.func1: a / b -> a / b'", "calc.Mod: a % b -> a % b'", "calc.Big: { ... } -> { ... }'", "calc.Big: x + 1 -> x + 1'",
	}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Errorf("mutants are listed as %q, want them by file and line, %q", order, wantOrder)
	}

	k := byID["calc.Add: a + b -> a + b'"]
	wantKills := []jsonKill{
		{Test: "calc/TestA", Subtests: []string{"TestA/a", "TestA/x"}, Obligations: []string{"calc/C1"}, Kind: "assertion"},
		{Test: "calc/TestB", Subtests: []string{}, Obligations: []string{"calc/C1", "calc/C2"}, Kind: "assertion"},
		{Test: "dial/TestD", Subtests: []string{}, Obligations: []string{"dial/D1"}, Kind: "panic"},
	}
	if k.Status != "killed" || !reflect.DeepEqual(k.KilledBy, wantKills) {
		t.Errorf("the killed mutant is %s, killed by %+v; want killed by %+v", k.Status, k.KilledBy, wantKills)
	}
	if want := []string{"calc/TestA", "calc/TestB", "dial/TestD"}; !reflect.DeepEqual(k.RanBy, want) {
		t.Errorf("the killed mutant ran by %q, want %q", k.RanBy, want)
	}
	if want := []string{"calc/C1", "calc/C2", "dial/D1"}; !reflect.DeepEqual(k.Obligations, want) {
		t.Errorf("the killed mutant's obligations are %q, want %q", k.Obligations, want)
	}
	if k.Declaration != nil || k.OutsideOnly {
		t.Errorf("the killed mutant has declaration %+v and outside_only %v", k.Declaration, k.OutsideOnly)
	}

	for id, want := range map[string]string{
		"calc.Add: a - b -> a - b'":       "lived",
		"calc.Sub: a - b -> a - b'":       "unreached",
		"calc.Mul: a * b -> a * b'":       "declared",
		"calc.Div.func1: a / b -> a / b'": "declared",
		"calc.Mod: a % b -> a % b'":       "no verdict",
		"calc.Big: { ... } -> { ... }'":   "erased",
		"calc.Big: x + 1 -> x + 1'":       "skipped",
	} {
		if got := byID[id].Status; got != want {
			t.Errorf("%s is %q, want %q", id, got, want)
		}
	}
	if u := byID["calc.Sub: a - b -> a - b'"]; u.RanBy == nil || len(u.RanBy) != 0 || u.KilledBy == nil {
		t.Errorf("an unreached mutant should list no tests as [], got ran_by %v and killed_by %v", u.RanBy, u.KilledBy)
	}
	if v := byID["calc.Mod: a % b -> a % b'"]; v.Verdict != "it wouldn't build" {
		t.Errorf("the mutant without a verdict says %q", v.Verdict)
	}
	eq := byID["calc.Mul: a * b -> a * b'"].Declaration
	if eq == nil || eq.Kind != "equivalent" || eq.Path != "contract/calc/mutants.md" || eq.Line != 3 || eq.Reason != "## Same\n\nSame." || eq.Wildcard {
		t.Errorf("the exact declaration reads %+v", eq)
	}
	wc := byID["calc.Div.func1: a / b -> a / b'"].Declaration
	if wc == nil || wc.Kind != "unpromised" || wc.Line != 9 || !wc.Wildcard {
		t.Errorf("the wildcard declaration reads %+v", wc)
	}

	if len(rep.ContractErrors) != 2 || rep.ContractErrors[0].Path != "contract/a/README.md" {
		t.Errorf("Contract errors should be sorted by path: %+v", rep.ContractErrors)
	}
	if want := []string{"cmd/m", "internal/a", "internal/z"}; !reflect.DeepEqual(rep.Outside, want) {
		t.Errorf("outside = %q, want %q", rep.Outside, want)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(rep.Build.Tags, want) {
		t.Errorf("tags = %q, want %q", rep.Build.Tags, want)
	}
	var obs []string
	for _, o := range rep.Obligations {
		obs = append(obs, o.ID)
	}
	if want := []string{"calc/C1", "calc/C10", "calc/C2", "dial/D1"}; !reflect.DeepEqual(obs, want) {
		t.Errorf("obligations are listed as %q, want %q", obs, want)
	}
	if want := []string{"calc/TestA", "calc/TestB"}; !reflect.DeepEqual(rep.Obligations[0].Tests, want) {
		t.Errorf("calc/C1's tests are %q, want %q", rep.Obligations[0].Tests, want)
	}
	if len(rep.Tests) != 3 || rep.Tests[2].Name != "dial/TestD" || rep.Tests[2].Kind != "Example" {
		t.Errorf("tests are %+v", rep.Tests)
	}
	if rep.Timing["total"] != 1.5 || rep.Timing["plan"] != 0.375 {
		t.Errorf("timing is %v, want seconds: total 1.5 and plan 0.375", rep.Timing)
	}
}

// Contract: report/P7
func TestTheJSONSummaryCountsWhatTheTextOneDoes(t *testing.T) {
	type summary struct {
		Scoped    bool
		Since     string
		Mutants   int
		Killed    int
		Lived     int
		Unreached int
		Erased    int
		Skipped   int
		Declared  struct{ Total, Equivalent, Unpromised, Wildcard int }
		NoVerdict int `json:"no_verdict"`
		Unheld    int
		Unviable  int
		Score     *float64
		Obls      struct {
			Total, Holding, Hollow int
			CrashOnly              int `json:"crash_only"`
			WithoutSole            int `json:"without_sole_holds"`
			NotJudged              int `json:"empty_not_judged"`
			SoleHoldsSum           int `json:"sole_holds_total"`
		} `json:"obligations"`
		Tests struct{ Total, Blind int }
	}
	read := func(r *report.Report) summary {
		var v struct{ Summary summary }
		if err := json.Unmarshal([]byte(jsonOf(t, r)), &v); err != nil {
			t.Fatal(err)
		}
		return v.Summary
	}

	s := read(rich())
	got := []int{s.Mutants, s.Killed, s.Lived, s.Unreached, s.Erased, s.Skipped, s.NoVerdict, s.Unheld, s.Unviable,
		s.Declared.Total, s.Declared.Equivalent, s.Declared.Unpromised, s.Declared.Wildcard,
		s.Obls.Total, s.Obls.Holding, s.Obls.Hollow, s.Obls.CrashOnly, s.Obls.WithoutSole, s.Obls.NotJudged, s.Obls.SoleHoldsSum,
		s.Tests.Total, s.Tests.Blind}
	want := []int{15, 3, 2, 2, 1, 2, 1, 5, 3, 4, 1, 1, 2, 9, 4, 5, 3, 7, 0, 2, 8, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("summary counts are %v, want %v", got, want)
	}
	if s.Score == nil || *s.Score != 37.5 || s.Scoped || s.Since != "" {
		t.Errorf("score %v, scoped %v, since %q; want 37.5, a full run and no ref", s.Score, s.Scoped, s.Since)
	}

	two := rich()
	two.Matrix = &matrix.Result{Mutants: []matrix.Mutant{
		mu("calc", "calc/a.go", 1, 1, "A", "a", model.Killed), mu("calc", "calc/a.go", 2, 1, "A", "b", model.Killed),
		mu("calc", "calc/a.go", 3, 1, "A", "c", model.Erased),
	}}
	two.Scoped = true
	if s := read(two); s.Score == nil || *s.Score != 66.6 || !s.Scoped {
		t.Errorf("2 killed and 1 erased score %v, scoped %v; want 66.6, cut and not rounded, in a scoped run", s.Score, s.Scoped)
	}

	none := &report.Report{Since: "main", Matrix: &matrix.Result{}}
	if s := read(none); s.Score != nil || !s.Scoped || s.Since != "main" {
		t.Errorf("an empty run since main has score %v, scoped %v, since %q; want no score, scoped, main", s.Score, s.Scoped, s.Since)
	}
}
