package report_test

import (
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/report"
)

// entry returns the lines of one finding: its first line, which starts with head, and the indented
// lines under it.
func entry(t *testing.T, lines []string, head string) string {
	t.Helper()
	for i, l := range lines {
		if strings.HasPrefix(l, head) {
			end := i + 1
			for end < len(lines) && strings.HasPrefix(lines[end], "    ") {
				end++
			}
			return strings.Join(lines[i:end], "\n")
		}
	}
	t.Fatalf("no finding starts with %q in:\n%s", head, strings.Join(lines, "\n"))
	return ""
}

// Contract: report/P1
func TestTheTextReportListsContractErrorsThenUnheldMutants(t *testing.T) {
	out := text(t, rich())
	inOrder(t, out,
		"Contract errors",
		"\n  contract/alpha/README.md:3: no Contract test names alpha/A3\n",
		"\n  contract/zeta/README.md:9: no Contract test names zeta/Z9\n",
		"\nUnheld (5)\n",
	)

	un := section(t, out, "Unheld (")
	// By primitive first, then file and line, each with its hash and then its id.
	inOrder(t, strings.Join(un, "\n"),
		"\n  calc  calc/a.go:5  "+rLived.Hash+"\n    "+rLived.ID+"\n",
		"\n  calc  calc/a.go:20  "+rErased.Hash+"\n    "+rErased.ID+"\n",
		"\n  calc  calc/b.go:3  "+rUnreached.Hash+"\n    "+rUnreached.ID+"\n",
		"\n  dial  dial/d.go:9  "+rDialLived.Hash+"\n    "+rDialLived.ID+"\n",
		"\n  zoo  a/a.go:1  "+rZoo.Hash+"\n    "+rZoo.ID+"\n",
	)

	// Each names the obligations whose tests ran it, and those tests.
	lived := entry(t, un, "  calc  calc/a.go:5  ")
	for _, want := range []string{"lived", "calc/C1, calc/C5, dial/D1", "TestA", "TestBlind1", "dial/TestB"} {
		if !strings.Contains(lived, want) {
			t.Errorf("the lived mutant's finding lacks %q:\n%s", want, lived)
		}
	}
	erased := entry(t, un, "  calc  calc/a.go:20  ")
	for _, want := range []string{"erased", "calc/C1", "TestA", "2 finer mutants skipped"} {
		if !strings.Contains(erased, want) {
			t.Errorf("the erased function's finding lacks %q:\n%s", want, erased)
		}
	}
	alone := rErased
	alone.Erase = true
	solo := entry(t, section(t, text(t, &report.Report{Matrix: &matrix.Result{Mutants: []matrix.Mutant{alone}}, Exit: 1}), "Unheld ("), "  calc  ")
	if !strings.Contains(solo, "erased") || strings.Contains(solo, "skipped") {
		t.Errorf("an erased function with no finer mutants should say it's erased and nothing about skipping:\n%s", solo)
	}
	unreached := entry(t, un, "  calc  calc/b.go:3  ")
	if !strings.Contains(unreached, "unreached") || strings.Contains(unreached, "calc/C1") {
		t.Errorf("the unreached mutant's finding should say it's unreached and name no obligation:\n%s", unreached)
	}
	dial := entry(t, un, "  dial  dial/d.go:9  ")
	if !strings.Contains(dial, "calc/C5, dial/D1") || !strings.Contains(dial, "calc/TestBlind1") {
		t.Errorf("a test from another primitive should carry its primitive:\n%s", dial)
	}

	// Killed, declared, skipped and undecided mutants aren't unheld.
	for _, m := range []matrix.Mutant{rOut1, rOut2, rKilled, rDeclEq, rDeclW1, rSkip1, rSkip2, rNoVerdict} {
		for _, l := range un {
			if strings.Contains(l, m.ID) {
				t.Errorf("%s, which is %s, is listed as unheld", m.ID, m.Status)
			}
		}
	}

	// A report that stopped at Contract errors lists them after its summary.
	stopped := rich()
	stopped.Matrix = nil
	out = text(t, stopped)
	inOrder(t, out, "stopped at Contract errors", "\nContract errors", "contract/alpha/README.md:3:", "contract/zeta/README.md:9:", "\nexit 1\n")
	if strings.Contains(out, "Unheld") {
		t.Errorf("a report without a matrix lists unheld mutants:\n%s", out)
	}
}

// Contract: report/P2
func TestTheOtherFindingsFollowEachListSorted(t *testing.T) {
	r := rich()
	r.Undecided = "calc/TestA fails on clean code"
	r.Exit = 2
	out := text(t, r)
	inOrder(t, out,
		"\nUnheld (5)\n",
		"\nNo verdict (1)\n",
		"\nBroken declarations",
		"\nHollow obligations (5)\n",
		"\nHeld only by crashes",
		"\nHeld only from outside",
		"\nBlind Contract tests (2)\n",
		"\nOutside the Contract",
		"\n\nflinch couldn't decide: calc/TestA fails on clean code\n\nexit 2\n",
	)
	if !strings.HasSuffix(out, "\nexit 2\n") {
		t.Errorf("the report should end with its exit code:\n%s", out)
	}

	nv := strings.Join(section(t, out, "No verdict"), "\n")
	inOrder(t, nv, "\n  calc  calc/e.go:4  "+rNoVerdict.Hash, "\n    "+rNoVerdict.ID, "it type-checked but wouldn't build")
	dialNV, zooNV := mu("dial", "dial/a.go", 1, 1, "D", "d", model.NoVerdict), mu("zoo", "a/z.go", 2, 1, "Z", "z", model.NoVerdict)
	two := text(t, &report.Report{Matrix: &matrix.Result{Mutants: []matrix.Mutant{zooNV, rNoVerdict, dialNV}}, Exit: 2})
	inOrder(t, strings.Join(section(t, two, "No verdict"), "\n"), "\n  calc  calc/e.go:4", "\n  dial  dial/a.go:1", "\n  zoo  a/z.go:2")

	inOrder(t, strings.Join(section(t, out, "Broken declarations"), "\n"),
		"\n  contract/calc/mutants.md:9: stale; delete the line",
		"\n  contract/zeta/mutants.md:4: zeta/TestZ kills this mutant; delete the line")

	inOrder(t, strings.Join(section(t, out, "Hollow obligations"), "\n"),
		"\n  alpha/A1  no Contract test names it\n",
		"\n  calc/C5  1 test ran against 2 mutants and killed none\n",
		"\n  calc/C2  no Contract test names it\n",
		"\n  calc/C3  its test reaches no mutant\n",
		"\n  calc/C4  its 2 tests reach no mutant\n",
	)

	crash := section(t, out, "Held only by crashes")
	inOrder(t, strings.Join(crash, "\n"), "\n  calc/C10  ", "\n  dial/D2  ", "\n  dial/D1  ")
	if n := strings.Count(strings.Join(crash, "\n"), "\n  calc/C1  "); n > 0 {
		t.Errorf("calc/C1 has an assertion kill and is listed as held only by crashes")
	}

	outside := strings.Join(section(t, out, "Held only from outside"), "\n")
	inOrder(t, outside,
		"\n  calc  calc/a.go:1  "+rOut2.Hash+"\n    "+rOut2.ID+"\n    killed only by dial/TestC (dial/D1)\n",
		"\n  calc  calc/c.go:7  "+rOut1.Hash+"\n    "+rOut1.ID+"\n    killed only by dial/TestB, dial/TestC (dial/D1, dial/D2)\n",
	)
	if strings.Contains(outside, rKilled.ID) {
		t.Errorf("a mutant its own primitive killed is listed as held only from outside:\n%s", outside)
	}

	inOrder(t, strings.Join(section(t, out, "Blind Contract tests"), "\n"),
		"\n  contract/calc/a_test.go:40  TestBlind1 (calc/C5)\n    ran in 2 complete rows of undeclared mutants and killed none\n",
		"\n  contract/calc/b_test.go:5  TestBlind2 (calc/C10)\n    ran in 1 complete row of undeclared mutants and killed none\n",
	)

	inOrder(t, strings.Join(section(t, out, "Outside the Contract"), "\n"), "\n  cmd/x\n", "\n  internal/zz\n")

	// Obligations keep Contract order, which no sort by id would give.
	ids := []string{"beta/Z1", "calc/K9", "calc/K10", "calc/K2", "calc/A1"}
	var obs []matrix.Obligation
	for _, id := range ids {
		obs = append(obs, matrix.Obligation{ID: id, Primitive: strings.Split(id, "/")[0], Judged: true, Hollow: true, CrashOnly: true})
	}
	ordered := text(t, &report.Report{Matrix: &matrix.Result{Obligations: obs}})
	want := ids
	for _, title := range []string{"Hollow obligations", "Held only by crashes"} {
		var got []string
		for _, l := range section(t, ordered, title)[1:] {
			if id, _, ok := strings.Cut(strings.TrimPrefix(l, "  "), "  "); ok && !strings.HasPrefix(l, "    ") {
				got = append(got, id)
			}
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s are listed as %q, want %q", title, got, want)
		}
	}
}

// Contract: report/P3
func TestEveryFindingSaysWhatWouldClearIt(t *testing.T) {
	out := text(t, rich())
	blocks := strings.Split(out, "\n\n")[1:] // the summary comes first and is no finding
	titles := 0
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
		title := lines[0]
		if strings.HasPrefix(title, "exit ") {
			continue
		}
		titles++
		switch {
		case strings.Contains(title, "cleared by"):
			// Each finding is one line, and the title says how to clear every one of them.
		case strings.HasPrefix(title, "Outside the Contract"):
			if last := lines[len(lines)-1]; !strings.HasPrefix(last, "  to bring one in") {
				t.Errorf("the packages outside the Contract don't say how to bring one in:\n%s", block)
			}
		default:
			cleared := true
			for i := 1; i < len(lines); i++ {
				if !strings.HasPrefix(lines[i], "    ") {
					cleared = false // a finding starts; it's cleared once a "to clear" line follows
				}
				if strings.HasPrefix(lines[i], "    to clear: ") {
					cleared = true
				}
				if !cleared && (i+1 == len(lines) || !strings.HasPrefix(lines[i+1], "    ")) {
					t.Errorf("a finding in %q doesn't say what would clear it:\n%s", title, block)
					cleared = true
				}
			}
		}
	}
	if titles != 9 {
		t.Errorf("the report has %d sections of findings, want 9:\n%s", titles, out)
	}

	// A broken declaration's own line says to delete it, whichever way it broke.
	a := model.TestRef{Primitive: "calc", Name: "TestA"}
	killed, gone := mu("calc", "calc/a.go", 3, 2, "Add", "a + b", model.Killed), mu("calc", "calc/a.go", 4, 2, "Add", "a - b", model.Killed)
	res := matrix.Compute(matrix.Input{
		Obligations: []string{"calc/C1"},
		Tests:       []model.Test{{TestRef: a, File: "contract/calc/calc_test.go", Line: 9, Obligations: []string{"calc/C1"}}},
		Mutants:     []model.Mutant{killed.Mutant},
		Rows:        map[string]model.Row{killed.Hash: {Ran: []model.TestRef{a}, Complete: true, Kills: []model.Kill{{Test: a, Kind: model.Assertion}}}},
		Declarations: []matrix.Declaration{
			{Kind: "equivalent", Primitive: "calc", Path: "contract/calc/mutants.md", Line: 3, Reason: "r", ID: killed.ID},
			{Kind: "unpromised", Primitive: "calc", Path: "contract/calc/mutants.md", Line: 8, Reason: "r", ID: gone.ID},
		},
		Full: true,
	})
	broken := section(t, text(t, &report.Report{Matrix: res, Exit: 1}), "Broken declarations")
	if len(broken) != 3 {
		t.Fatalf("want two broken declarations, got:\n%s", strings.Join(broken, "\n"))
	}
	for _, l := range broken[1:] {
		if !strings.Contains(l, "delete the line") {
			t.Errorf("a broken declaration doesn't say what would clear it: %q", l)
		}
	}

	// The advice names the mutants.md each declaration would go in.
	for _, c := range []struct{ contract, prim, want string }{
		{"", "calc", "contract/calc/mutants.md"},
		{"", "zoo", "contract/zoo/mutants.md"},
		{"spec", "dial", "spec/dial/mutants.md"},
	} {
		r := rich()
		r.Contract = c.contract
		un := section(t, text(t, r), "Unheld (")
		var head string
		for _, l := range un {
			if strings.HasPrefix(l, "  "+c.prim+"  ") {
				head = l
				break
			}
		}
		if e := entry(t, un, head); !strings.Contains(e, "to clear: ") || !strings.Contains(e, c.want) {
			t.Errorf("with the Contract in %q, %s's finding should point at %s:\n%s", c.contract, c.prim, c.want, e)
		}
	}
}

// summary returns the lines of the text report up to its first blank line.
func summary(t *testing.T, r *report.Report) string {
	t.Helper()
	out := text(t, r)
	head, _, _ := strings.Cut(out, "\n\n")
	return head
}

// Contract: report/P7
func TestTheSummaryCountsSoleHoldsDeclarationsAndTheScore(t *testing.T) {
	want := strings.Join([]string{
		"flinch: 15 mutants in covered code, plus 3 unviable that never built",
		"  3 killed, 5 unheld (2 lived, 2 unreached, 1 erased), 2 skipped, 1 no verdict",
		"  4 declared: 1 equivalent, 1 unpromised, 2 by wildcard",
		"  score 37.5%, killed / (killed + unheld) = 3 / 8, for information only",
		"  9 obligations: 4 hold code, 5 hollow, 3 held only by crashes, 7 hold no mutant alone",
		"  sole holds per obligation:",
		"    alpha  A1 0",
		"    calc  C1 1, C10 1, C5 0, C2 0, C3 0, C4 0",
		"    dial  D2 0, D1 0",
		"  8 Contract tests: 2 blind",
		"  full run",
		"  flinch 0.1.0, go1.26.1 darwin/arm64, tags a,b",
	}, "\n")
	if got := summary(t, rich()); got != want {
		t.Errorf("the summary is\n%s\nwant\n%s", got, want)
	}

	// The score is cut, never rounded, so 2 of 3 reads 66.6%.
	k1, k2, l := mu("calc", "calc/a.go", 1, 1, "A", "a", model.Killed), mu("calc", "calc/a.go", 2, 1, "A", "b", model.Killed),
		mu("calc", "calc/a.go", 3, 1, "A", "c", model.Lived)
	small := &report.Report{Version: "0.1.0", GoVersion: "go1.26.1", GOOS: "linux", GOARCH: "amd64", Scoped: true,
		Matrix: &matrix.Result{
			Mutants: []matrix.Mutant{k1, k2, l},
			Obligations: []matrix.Obligation{
				{ID: "calc/C1", Primitive: "calc", Holds: []string{k1.Hash, k2.Hash}, SoleHolds: []string{k1.Hash, k2.Hash}, Judged: true},
				{ID: "calc/C2", Primitive: "calc"},
			},
		}}
	want = strings.Join([]string{
		"flinch: 3 mutants in covered code",
		"  2 killed, 1 unheld (1 lived, 0 unreached, 0 erased), 0 skipped, 0 no verdict",
		"  0 declared: 0 equivalent, 0 unpromised, 0 by wildcard",
		"  score 66.6%, killed / (killed + unheld) = 2 / 3, for information only",
		"  2 obligations: 1 hold code, 0 hollow, 0 held only by crashes, 1 hold no mutant alone; 1 hold nothing in this scoped run and aren't judged",
		"  sole holds per obligation:",
		"    calc  C1 2, C2 0",
		"  0 Contract tests: 0 blind",
		"  scoped run",
		"  flinch 0.1.0, go1.26.1 linux/amd64, tags none",
	}, "\n")
	if got := summary(t, small); got != want {
		t.Errorf("the summary is\n%s\nwant\n%s", got, want)
	}

	// Equivalent and unpromised declarations are counted apart.
	declared := func(change, kind string) matrix.Mutant {
		m := mu("calc", "calc/d.go", 1, 1, "D", change, model.Declared)
		m.Declaration = &matrix.Declaration{Kind: kind, Primitive: "calc", Path: "contract/calc/mutants.md", Line: 1, ID: m.ID}
		return m
	}
	split := &report.Report{Matrix: &matrix.Result{Mutants: []matrix.Mutant{
		declared("a", "equivalent"), declared("b", "equivalent"), declared("c", "unpromised"),
	}}}
	if got := summary(t, split); !strings.Contains(got, "\n  3 declared: 2 equivalent, 1 unpromised, 0 by wildcard\n") {
		t.Errorf("the summary should count 2 equivalent and 1 unpromised declarations:\n%s", got)
	}

	// With nothing killed or unheld there is no score, and a run since a ref says so.
	empty := &report.Report{Version: "0.1.0", GoVersion: "go1.26.1", GOOS: "linux", GOARCH: "amd64", Since: "main",
		Matrix: &matrix.Result{Mutants: []matrix.Mutant{mu("calc", "calc/a.go", 1, 1, "A", "a", model.Skipped)}}}
	got := summary(t, empty)
	for _, w := range []string{"\n  score none", "\n  scoped run, --since main\n"} {
		if !strings.Contains(got, w) {
			t.Errorf("the summary lacks %q:\n%s", w, got)
		}
	}
	if strings.Contains(got, "sole holds") {
		t.Errorf("a run with no obligations lists sole holds:\n%s", got)
	}

	// A run that stopped at Contract errors still says what it was.
	stopped := &report.Report{Version: "0.1.0", GoVersion: "go1.26.1", GOOS: "linux", GOARCH: "amd64", Scoped: true, Exit: 1}
	want = "flinch: stopped at Contract errors, before generating any mutant\n  scoped run\n  flinch 0.1.0, go1.26.1 linux/amd64, tags none"
	if got := summary(t, stopped); got != want {
		t.Errorf("the summary is\n%s\nwant\n%s", got, want)
	}
}
