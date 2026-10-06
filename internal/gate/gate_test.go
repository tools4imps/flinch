package gate

import (
	"reflect"
	"testing"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
)

func withStatus(ss ...model.Status) *matrix.Result {
	r := &matrix.Result{}
	for _, s := range ss {
		r.Mutants = append(r.Mutants, matrix.Mutant{Status: s})
	}
	return r
}

func TestExitCodes(t *testing.T) {
	contractError := []problem.Problem{{Path: "contract/skip/README.md", Line: 3, Message: "duplicate obligation"}}
	cases := []struct {
		name      string
		problems  []problem.Problem
		r         *matrix.Result
		undecided string
		full      bool
		exit      int
	}{
		{"nothing wrong", nil, withStatus(model.Killed, model.Declared, model.Skipped), "", true, Pass},
		{"no matrix and no problems", nil, nil, "", false, Pass},
		{"contract error stops the run", contractError, nil, "", false, Fail},
		{"lived", nil, withStatus(model.Killed, model.Lived), "", false, Fail},
		{"unreached", nil, withStatus(model.Unreached), "", false, Fail},
		{"erased", nil, withStatus(model.Erased, model.Skipped), "", false, Fail},
		{"broken declaration", nil, &matrix.Result{Broken: contractError}, "", false, Fail},
		{"hollow in a full run", nil, &matrix.Result{Obligations: []matrix.Obligation{{ID: "skip/K1", Judged: true, Hollow: true}}}, "", true, Fail},
		{"hollow in a whole primitive", nil, &matrix.Result{Obligations: []matrix.Obligation{{ID: "skip/K1", Judged: true, Hollow: true}}}, "", false, Fail},
		{"blind in a full run", nil, &matrix.Result{Tests: []matrix.Test{{Judged: true, Blind: true}}}, "", true, Fail},
		{"crash-only and outside-only never fail", nil, &matrix.Result{
			Mutants:     []matrix.Mutant{{Status: model.Killed, OutsideOnly: true}},
			Obligations: []matrix.Obligation{{ID: "skip/K1", Judged: true, CrashOnly: true}},
		}, "", true, Pass},
		{"undecided", nil, withStatus(model.Killed), "TestA failed on clean code", true, Undecided},
		{"undecided wins over a failure", contractError, withStatus(model.Lived), "TestA failed on clean code", true, Undecided},
		{"a mutant with no verdict is undecided", nil, withStatus(model.Lived, model.NoVerdict), "", true, Undecided},
	}
	for _, c := range cases {
		v := Decide(c.problems, c.r, c.undecided, c.full)
		if v.Exit != c.exit {
			t.Errorf("%s: exit %d, want %d (reasons %q)", c.name, v.Exit, c.exit, v.Reasons)
		}
		if c.exit != Pass && len(v.Reasons) == 0 {
			t.Errorf("%s: a failing verdict should give its reasons", c.name)
		}
	}
}

func TestReasonsNameEveryKindOfFinding(t *testing.T) {
	r := &matrix.Result{
		Mutants:     []matrix.Mutant{{Status: model.Lived}, {Status: model.Unreached}, {Status: model.NoVerdict}},
		Broken:      []problem.Problem{{Path: "contract/skip/mutants.md", Line: 4}},
		Obligations: []matrix.Obligation{{Judged: true, Hollow: true}},
		Tests:       []matrix.Test{{Judged: true, Blind: true}, {Judged: true, Blind: true}},
	}
	v := Decide([]problem.Problem{{Path: "x"}}, r, "a test failed on clean code", true)
	want := []string{
		"flinch couldn't decide: a test failed on clean code",
		"1 mutant has no verdict",
		"1 Contract error",
		"2 unheld mutants",
		"1 broken declaration",
		"1 hollow obligation",
		"2 blind tests",
	}
	if v.Exit != Undecided || !reflect.DeepEqual(v.Reasons, want) {
		t.Errorf("Decide = %d %q\nwant 2 %q", v.Exit, v.Reasons, want)
	}
}
