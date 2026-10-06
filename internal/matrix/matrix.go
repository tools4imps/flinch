// Package matrix turns a run's rows into the Contract's answer: where each mutant ended up, which
// obligations hold which mutants, which tests killed nothing, and which declarations broke or went
// stale. It reads plain values and calls nothing, so every rule here can be checked with made-up input.
package matrix

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/problem"
)

// A Declaration is one line of an `equivalent` or `unpromised` block in a primitive's mutants.md.
type Declaration struct {
	Kind      string // "equivalent" or "unpromised"
	Primitive string // the primitive whose mutants.md holds the line
	Path      string // that mutants.md, slash-separated, relative to the module root
	Line      int
	Reason    string
	Wildcard  bool
	Dir, Unit string // for a wildcard
	ID        string // for an exact id, canonical form
}

// Input is everything a run learned, in the plain form the matrix needs.
type Input struct {
	Obligations  []string                   // full ids, e.g. "skip/K1", in Contract order
	Tests        []model.Test               // every Contract test
	Mutants      []model.Mutant             // every generated mutant in scope, skipped ones included
	Rows         map[string]model.Row       // by mutant hash; absent for unreached and skipped mutants
	Reached      map[string][]model.TestRef // by mutant hash: the Contract tests whose lone run reached it
	Declarations []Declaration
	Full         bool            // the run mutated every covered unit
	Whole        map[string]bool // primitives whose covered code this run mutated whole
	WholeUnits   map[string]bool // "dir.Top" units mutated whole
}

// A Kill is one test failing against one mutant, with the obligations that test names.
type Kill struct {
	model.Kill
	Obligations []string
}

// A Mutant is one generated mutant and where it ended up.
type Mutant struct {
	model.Mutant
	Status model.Status
	Kills  []Kill // sorted by test
	// RanBy lists the Contract tests that reach the mutant's line, whether or not it ran: the ones
	// in its row and the ones whose lone run reached it. It's empty for an unreached mutant.
	RanBy []model.TestRef
	// Obligations lists the obligations named by the tests in RanBy.
	Obligations []string
	Declaration *Declaration // the declaration that matches it, if any
	// OutsideOnly marks a killed mutant that only other primitives' tests killed.
	OutsideOnly bool
	Verdict     string // why the mutant has no verdict, when it has none
}

// An Obligation is one promise in the Contract and what its tests held.
type Obligation struct {
	ID        string // full id, e.g. "skip/K1"
	Primitive string
	Tests     []model.TestRef // the Contract tests that name it
	Holds     []string        // hashes of the mutants its tests killed, sorted
	SoleHolds []string        // hashes of the held mutants no other obligation holds, sorted
	// Judged says the run mutated the obligation's primitive whole, so holding nothing means something.
	Judged bool
	Hollow bool // judged, and holds nothing
	// CrashOnly says every mutant it holds was killed by its tests only through a panic or a timeout.
	CrashOnly bool
}

// A Test is one Contract test and what it killed.
type Test struct {
	model.Test
	Kills []string // hashes of the mutants it killed, sorted
	// Ran counts the complete rows of undeclared mutants it ran in, the rows blindness is judged on.
	Ran int
	// Judged says the run mutated the test's primitive whole.
	Judged bool
	Blind  bool // judged, ran in a complete row of an undeclared mutant, and killed nothing
}

// A Result is the matrix for one run.
type Result struct {
	Mutants     []Mutant     // in input order
	Obligations []Obligation // in input order
	Tests       []Test       // sorted by file, line and name
	// Broken lists declarations that are errors: one a Contract test kills (D5), and one that
	// matches nothing in a run that mutated its unit whole (D6). Sorted by path and line.
	Broken []problem.Problem
}

// Compute builds the matrix. It never fails: odd input, such as a kill by a test the input doesn't
// list, is taken at face value.
func Compute(in Input) *Result {
	tests := map[model.TestRef]model.Test{}
	for _, t := range in.Tests {
		tests[t.TestRef] = t
	}

	r := &Result{Mutants: make([]Mutant, len(in.Mutants))}
	for i, m := range in.Mutants {
		r.Mutants[i] = Mutant{Mutant: m}
	}
	setStatus(r.Mutants, in)
	for i := range r.Mutants {
		fillKills(&r.Mutants[i], in, tests)
	}
	r.Broken = declare(r.Mutants, in)
	r.Obligations = obligations(r.Mutants, in)
	r.Tests = testResults(r.Mutants, in)
	problem.Sort(r.Broken)
	return r
}

// setStatus places each mutant before declarations are applied. A non-erase mutant with no row
// in a function whose erase mutant lived is skipped, even when nothing reached it, because an
// erased function is reported once, by its erase mutant.
func setStatus(ms []Mutant, in Input) {
	erased := map[string]bool{} // "dir.Top" of every function whose erase mutant lived
	for i := range ms {
		m := &ms[i]
		row, ok := in.Rows[m.Hash]
		switch {
		case !ok:
			// Settled below, once every erase mutant is known.
		case row.Verdict != "":
			m.Status, m.Verdict = model.NoVerdict, row.Verdict
		case len(row.Kills) > 0:
			m.Status = model.Killed
		case m.Erase:
			m.Status = model.Erased
			erased[m.Dir+"."+m.Top] = true
		default:
			m.Status = model.Lived
		}
	}
	for i := range ms {
		m := &ms[i]
		if _, ok := in.Rows[m.Hash]; ok {
			continue
		}
		switch {
		case !m.Erase && erased[m.Dir+"."+m.Top]:
			m.Status = model.Skipped
		case len(in.Reached[m.Hash]) == 0:
			m.Status = model.Unreached
		default:
			// Contract tests reach it, yet it has no row and no erased function explains why.
			m.Status = model.NoVerdict
			m.Verdict = "Contract tests reach this mutant, but it was never run"
		}
	}
}

// fillKills copies the row's kills with each killer's obligations, the tests that reach the mutant,
// and whether only other primitives' tests killed it.
func fillKills(m *Mutant, in Input, tests map[model.TestRef]model.Test) {
	row := in.Rows[m.Hash]
	ran := map[model.TestRef]bool{}
	for _, t := range row.Ran {
		ran[t] = true
	}
	for _, t := range in.Reached[m.Hash] {
		ran[t] = true
	}
	if m.Status == model.Killed {
		for _, k := range row.Kills {
			ran[k.Test] = true
			k.Subtests = sortedCopy(k.Subtests)
			m.Kills = append(m.Kills, Kill{Kill: k, Obligations: sortedCopy(tests[k.Test].Obligations)})
		}
		sort.SliceStable(m.Kills, func(i, j int) bool { return lessRef(m.Kills[i].Test, m.Kills[j].Test) })
		m.OutsideOnly = len(m.Kills) > 0
		for _, k := range m.Kills {
			if k.Test.Primitive == m.Primitive {
				m.OutsideOnly = false
			}
		}
	}
	obs := map[string]bool{}
	for t := range ran {
		m.RanBy = append(m.RanBy, t)
		for _, o := range tests[t].Obligations {
			obs[o] = true
		}
	}
	sort.Slice(m.RanBy, func(i, j int) bool { return lessRef(m.RanBy[i], m.RanBy[j]) })
	m.Obligations = keys(obs)
}

// declare matches declarations to mutants. An exact id is tried before any wildcard. It returns
// every broken and stale declaration as a problem at the declaration's line.
func declare(ms []Mutant, in Input) []problem.Problem {
	var broken []problem.Problem
	byID := map[string][]int{}
	for i, m := range ms {
		byID[m.ID] = append(byID[m.ID], i)
	}

	for di := range in.Declarations {
		d := &in.Declarations[di]
		if d.Wildcard {
			continue
		}
		matched := byID[d.ID]
		for _, i := range matched {
			m := &ms[i]
			if m.Declaration == nil {
				m.Declaration = copyOf(d)
			}
			switch {
			case m.Status == model.Killed:
				broken = append(broken, problem.Problem{Path: d.Path, Line: d.Line, Message: brokenMessage(m)})
			case m.Status.Unheld():
				m.Status = model.Declared
			}
		}
		if len(matched) == 0 && exactWhole(d, in) {
			broken = append(broken, problem.Problem{Path: d.Path, Line: d.Line,
				Message: "this run mutated the whole unit and no mutant has this id, so the declaration is stale; delete the line"})
		}
	}

	for di := range in.Declarations {
		d := &in.Declarations[di]
		if !d.Wildcard || d.Kind != "unpromised" {
			continue
		}
		covered, undecided := 0, false
		for i := range ms {
			m := &ms[i]
			if m.Dir != d.Dir || !inUnit(m.Unit, d.Unit) {
				continue
			}
			if m.Status == model.NoVerdict {
				undecided = true
			}
			if m.Declaration == nil && m.Status.Unheld() {
				m.Declaration = copyOf(d)
				m.Status = model.Declared
				covered++
			}
		}
		if covered == 0 && !undecided && wildcardWhole(d, in) {
			broken = append(broken, problem.Problem{Path: d.Path, Line: d.Line, Message: fmt.Sprintf(
				"%s has no unheld mutants for this wildcard to cover, so it is stale; delete the line",
				strings.TrimSuffix(mutantid.Prefix(d.Dir, d.Unit), ": "))})
		}
	}
	return broken
}

// brokenMessage says which Contract tests kill a declared mutant (D5).
func brokenMessage(m *Mutant) string {
	var names []string
	for _, k := range m.Kills {
		names = append(names, k.Test.String())
	}
	return fmt.Sprintf("%s %s this mutant, so the Contract does promise this behavior; delete the line",
		strings.Join(names, ", "), plural(len(names), "kills", "kill"))
}

// inUnit reports whether a mutant's unit is the declared unit or a closure inside it.
func inUnit(unit, declared string) bool {
	return unit == declared || strings.HasPrefix(unit, declared+".")
}

// exactWhole reports whether a run mutated the whole unit an exact declaration names, which is when
// a declaration that matches nothing is stale. A unit is whole when its top-level unit is, so every
// leading part of the unit name is tried, since the top-level unit is one of them.
func exactWhole(d *Declaration, in Input) bool {
	if in.Full || in.Whole[d.Primitive] {
		return true
	}
	p, err := mutantid.Parse(d.ID)
	if err != nil {
		return false
	}
	return unitWhole(p.Dir, p.Unit, in)
}

func wildcardWhole(d *Declaration, in Input) bool {
	return in.Full || in.Whole[d.Primitive] || unitWhole(d.Dir, d.Unit, in)
}

func unitWhole(dir, unit string, in Input) bool {
	parts := strings.Split(unit, ".")
	for n := 1; n <= len(parts); n++ {
		if in.WholeUnits[dir+"."+strings.Join(parts[:n], ".")] {
			return true
		}
	}
	return false
}

// obligations credits each kill to the obligations of the test that made it (X1), then judges
// sole holds, hollowness (X2, X4) and crash-only holds (X5).
func obligations(ms []Mutant, in Input) []Obligation {
	holders := map[string]map[string]bool{} // mutant hash to the obligations holding it
	byOb := map[string]map[string]bool{}    // obligation to the hashes it holds
	solid := map[string]bool{}              // obligations with a hold from an assertion
	for _, m := range ms {
		if m.Status != model.Killed {
			continue
		}
		for _, k := range m.Kills {
			for _, o := range k.Obligations {
				if holders[m.Hash] == nil {
					holders[m.Hash] = map[string]bool{}
				}
				holders[m.Hash][o] = true
				if byOb[o] == nil {
					byOb[o] = map[string]bool{}
				}
				byOb[o][m.Hash] = true
				if k.Kind != model.Panic && k.Kind != model.Timeout {
					solid[o] = true
				}
			}
		}
	}

	named := map[string][]model.TestRef{}
	for _, t := range in.Tests {
		for _, o := range t.Obligations {
			named[o] = append(named[o], t.TestRef)
		}
	}

	out := make([]Obligation, 0, len(in.Obligations))
	for _, id := range in.Obligations {
		o := Obligation{ID: id, Primitive: primitiveOf(id), Holds: keys(byOb[id])}
		o.Tests = append([]model.TestRef(nil), named[id]...)
		sort.Slice(o.Tests, func(i, j int) bool { return lessRef(o.Tests[i], o.Tests[j]) })
		for _, h := range o.Holds {
			if len(holders[h]) == 1 {
				o.SoleHolds = append(o.SoleHolds, h)
			}
		}
		o.Judged = in.Full || in.Whole[o.Primitive]
		o.Hollow = o.Judged && len(o.Holds) == 0
		o.CrashOnly = len(o.Holds) > 0 && !solid[id]
		out = append(out, o)
	}
	return out
}

// testResults records each test's kills and judges blindness (X3, X4). Only complete rows of
// undeclared mutants count, because a test can't be faulted for passing against a mutant the
// Contract says no test can catch, nor for a row it never finished.
func testResults(ms []Mutant, in Input) []Test {
	kills := map[model.TestRef]map[string]bool{}
	ran := map[model.TestRef]int{}
	for _, m := range ms {
		for _, k := range m.Kills {
			if kills[k.Test] == nil {
				kills[k.Test] = map[string]bool{}
			}
			kills[k.Test][m.Hash] = true
		}
		row, ok := in.Rows[m.Hash]
		if !ok || !row.Complete || m.Declaration != nil {
			continue
		}
		for _, t := range row.Ran {
			ran[t]++
		}
	}

	out := make([]Test, 0, len(in.Tests))
	for _, t := range in.Tests {
		r := Test{Test: t, Kills: keys(kills[t.TestRef]), Ran: ran[t.TestRef]}
		r.Obligations = sortedCopy(t.Obligations)
		r.Judged = in.Full || in.Whole[t.Primitive]
		r.Blind = r.Judged && r.Ran > 0 && len(r.Kills) == 0
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return lessRef(a.TestRef, b.TestRef)
	})
	return out
}

// primitiveOf reads the primitive from a full obligation id such as "skip/K1".
func primitiveOf(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		return id[:i]
	}
	return id
}

func lessRef(a, b model.TestRef) bool {
	if a.Primitive != b.Primitive {
		return a.Primitive < b.Primitive
	}
	return a.Name < b.Name
}

func keys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedCopy(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func copyOf(d *Declaration) *Declaration {
	c := *d
	return &c
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
