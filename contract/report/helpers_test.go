package report_test

import (
	"bytes"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/report"
)

func ref(prim, name string) model.TestRef { return model.TestRef{Primitive: prim, Name: name} }

// mu makes a matrix mutant with a real id and hash, in the package holding file.
func mu(prim, file string, line, col int, unit, change string, status model.Status) matrix.Mutant {
	dir := path.Dir(file)
	top, _, _ := strings.Cut(unit, ".")
	id := mutantid.ID{Dir: dir, Unit: unit, Original: change, Replacement: change + "'", N: 1}
	return matrix.Mutant{
		Mutant: model.Mutant{
			ID: id.String(), Hash: id.Hash(), Primitive: prim, Dir: dir, File: file, Line: line, Col: col,
			Unit: unit, Top: top, Original: change, Replace: change + "'", Operator: "arithmetic",
		},
		Status: status,
	}
}

func kill(t model.TestRef, kind model.KillKind, obligations ...string) matrix.Kill {
	return matrix.Kill{Kill: model.Kill{Test: t, Kind: kind}, Obligations: obligations}
}

func mtest(prim, name, file string, line int, ran int, blind bool, obligations ...string) matrix.Test {
	return matrix.Test{
		Test: model.Test{TestRef: ref(prim, name), Kind: "Test", Dir: "contract/" + prim, File: file, Line: line, Obligations: obligations},
		Ran:  ran, Judged: true, Blind: blind,
	}
}

// The mutants of the rich report, by name, so tests can look up their ids and hashes.
var (
	tA, tB, tC = ref("calc", "TestA"), ref("dial", "TestB"), ref("dial", "TestC")
	tBlind1    = ref("calc", "TestBlind1")

	rLived     = mu("calc", "calc/a.go", 5, 2, "Add", "a + b", model.Lived)
	rErased    = mu("calc", "calc/a.go", 20, 1, "Big", "{ ... }", model.Erased)
	rSkip1     = mu("calc", "calc/a.go", 21, 3, "Big", "x + 1", model.Skipped)
	rSkip2     = mu("calc", "calc/a.go", 22, 3, "Big.func1", "y - 1", model.Skipped)
	rUnreached = mu("calc", "calc/b.go", 3, 2, "Sub", "a - b", model.Unreached)
	rDialLived = mu("dial", "dial/d.go", 9, 2, "Turn", "n + 1", model.Lived)
	rZoo       = mu("zoo", "a/a.go", 1, 1, "Z", "z * 2", model.Unreached)
	rOut1      = mu("calc", "calc/c.go", 7, 2, "Mul", "a * b", model.Killed)
	rOut2      = mu("calc", "calc/a.go", 1, 4, "Add", "a + c", model.Killed)
	rNoVerdict = mu("calc", "calc/e.go", 4, 2, "Div", "a / b", model.NoVerdict)
	rKilled    = mu("calc", "calc/a.go", 6, 2, "Add", "a - c", model.Killed)
	rDeclEq    = mu("calc", "calc/f.go", 1, 1, "F", "f + 1", model.Declared)
	rDeclUn    = mu("calc", "calc/f.go", 2, 1, "F", "f + 2", model.Declared)
	rDeclW1    = mu("calc", "calc/g.go", 1, 1, "G", "g + 1", model.Declared)
	rDeclW2    = mu("calc", "calc/g.go", 2, 1, "G.func1", "g + 2", model.Declared)
)

// rich is a report with every kind of finding, each list out of order, so a test can check what
// the report says and in what order.
func rich() *report.Report {
	lived, erased, dialLived := rLived, rErased, rDialLived
	lived.RanBy = []model.TestRef{tA, tBlind1, tB}
	lived.Obligations = []string{"calc/C1", "calc/C5", "dial/D1"}
	erased.Erase = true
	erased.RanBy = []model.TestRef{tA}
	erased.Obligations = []string{"calc/C1"}
	dialLived.RanBy = []model.TestRef{tBlind1, tB}
	dialLived.Obligations = []string{"calc/C5", "dial/D1"}

	out1, out2 := rOut1, rOut2
	out1.OutsideOnly = true
	out1.Kills = []matrix.Kill{kill(tB, model.Assertion, "dial/D2", "dial/D1"), kill(tC, model.Panic, "dial/D1")}
	out1.RanBy = []model.TestRef{tB, tC}
	out2.OutsideOnly = true
	out2.Kills = []matrix.Kill{kill(tC, model.Timeout, "dial/D1")}
	out2.RanBy = []model.TestRef{tC}

	nv := rNoVerdict
	nv.Verdict = "it type-checked but wouldn't build"
	killed := rKilled
	killed.Kills = []matrix.Kill{kill(tA, model.Assertion, "calc/C1")}
	killed.RanBy = []model.TestRef{tA}
	killed.Obligations = []string{"calc/C1"}

	declare := func(m matrix.Mutant, kind string, wildcard bool, line int) matrix.Mutant {
		m.Declaration = &matrix.Declaration{
			Kind: kind, Primitive: m.Primitive, Path: "contract/calc/mutants.md", Line: line,
			Reason: "## Open\n\nOpen on purpose.", Wildcard: wildcard,
		}
		if wildcard {
			m.Declaration.Dir, m.Declaration.Unit = m.Dir, m.Top
		} else {
			m.Declaration.ID = m.ID
		}
		return m
	}

	idle, idle2 := ref("calc", "TestIdle"), ref("calc", "TestIdle2")
	m := &matrix.Result{
		Mutants: []matrix.Mutant{
			out1, rZoo, dialLived, rUnreached, erased, rSkip1, rSkip2, lived, out2, nv, killed,
			declare(rDeclEq, "equivalent", false, 3), declare(rDeclUn, "unpromised", false, 4),
			declare(rDeclW1, "unpromised", true, 5), declare(rDeclW2, "unpromised", true, 5),
		},
		// Contract order: primitives by name, each one's obligations as its README lists them.
		Obligations: []matrix.Obligation{
			{ID: "alpha/A1", Primitive: "alpha", Judged: true, Hollow: true},
			{ID: "calc/C1", Primitive: "calc", Tests: []model.TestRef{tA}, Holds: []string{killed.Hash}, SoleHolds: []string{killed.Hash}, Judged: true},
			{ID: "calc/C10", Primitive: "calc", Tests: []model.TestRef{ref("calc", "TestCrash"), ref("calc", "TestBlind2")},
				Holds: []string{out2.Hash}, SoleHolds: []string{out2.Hash}, Judged: true, CrashOnly: true},
			{ID: "calc/C5", Primitive: "calc", Tests: []model.TestRef{tBlind1}, Judged: true, Hollow: true},
			{ID: "calc/C2", Primitive: "calc", Judged: true, Hollow: true},
			{ID: "calc/C3", Primitive: "calc", Tests: []model.TestRef{idle}, Judged: true, Hollow: true},
			{ID: "calc/C4", Primitive: "calc", Tests: []model.TestRef{idle, idle2}, Judged: true, Hollow: true},
			{ID: "dial/D2", Primitive: "dial", Tests: []model.TestRef{tB}, Holds: []string{out1.Hash}, Judged: true, CrashOnly: true},
			{ID: "dial/D1", Primitive: "dial", Tests: []model.TestRef{tB, tC}, Holds: []string{out1.Hash, out2.Hash}, Judged: true, CrashOnly: true},
		},
		Tests: []matrix.Test{
			mtest("calc", "TestA", "contract/calc/a_test.go", 10, 3, false, "calc/C1"),
			mtest("calc", "TestBlind1", "contract/calc/a_test.go", 40, 2, true, "calc/C5"),
			mtest("calc", "TestIdle", "contract/calc/a_test.go", 50, 0, false, "calc/C3", "calc/C4"),
			mtest("calc", "TestIdle2", "contract/calc/a_test.go", 60, 0, false, "calc/C4"),
			mtest("calc", "TestCrash", "contract/calc/a_test.go", 70, 1, false, "calc/C10"),
			mtest("calc", "TestBlind2", "contract/calc/b_test.go", 5, 1, true, "calc/C10"),
			mtest("dial", "TestB", "contract/dial/d_test.go", 10, 2, false, "dial/D1", "dial/D2"),
			mtest("dial", "TestC", "contract/dial/d_test.go", 20, 1, false, "dial/D1"),
		},
		Broken: []problem.Problem{
			{Path: "contract/zeta/mutants.md", Line: 4, Message: "zeta/TestZ kills this mutant; delete the line"},
			{Path: "contract/calc/mutants.md", Line: 9, Message: "stale; delete the line"},
		},
	}
	return &report.Report{
		Version: "0.1.0", GoVersion: "go1.26.1", GOOS: "darwin", GOARCH: "arm64", Tags: []string{"a", "b"},
		Problems: []problem.Problem{
			{Path: "contract/zeta/README.md", Line: 9, Message: "no Contract test names zeta/Z9"},
			{Path: "contract/alpha/README.md", Line: 3, Message: "no Contract test names alpha/A3"},
		},
		Matrix:   m,
		Outside:  []string{"internal/zz", "cmd/x"},
		Exit:     1,
		Timing:   map[string]time.Duration{"total": 1500 * time.Millisecond, "plan": 250 * time.Millisecond},
		Unviable: 3,
	}
}

func text(t *testing.T, r *report.Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := report.Text(&b, r); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func jsonOf(t *testing.T, r *report.Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := report.JSON(&b, r); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// section returns the lines of the text report's section whose title starts with title, the title
// line first, up to the blank line that ends it.
func section(t *testing.T, out, title string) []string {
	t.Helper()
	ls := strings.Split(out, "\n")
	for i, l := range ls {
		if strings.HasPrefix(l, title) {
			end := i + 1
			for end < len(ls) && ls[end] != "" {
				end++
			}
			return ls[i:end]
		}
	}
	t.Fatalf("the report has no section %q:\n%s", title, out)
	return nil
}

// inOrder checks that each string appears in out, each after the one before.
func inOrder(t *testing.T, out string, want ...string) {
	t.Helper()
	at := 0
	for _, w := range want {
		i := strings.Index(out[at:], w)
		if i < 0 {
			t.Errorf("%q is missing, or comes too early, in:\n%s", w, out)
			return
		}
		at += i + 1
	}
}
