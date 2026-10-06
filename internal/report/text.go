package report

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
)

// Text writes the report for people: a summary, then each kind of finding worst first, then the exit
// code. Obligations keep Contract order, other lists are sorted by path and line, and unheld mutants
// are grouped by primitive first.
func Text(w io.Writer, r *Report) error {
	b := bufio.NewWriter(w)
	t := &text{w: b, r: r}
	t.summary()
	t.contractErrors()
	if r.Matrix != nil {
		t.unheld()
		t.noVerdict()
		t.broken()
		t.hollow()
		t.crashOnly()
		t.outsideOnly()
		t.blind()
	}
	t.outside()
	if r.Undecided != "" {
		t.line("")
		t.line("flinch couldn't decide: %s", r.Undecided)
	}
	t.line("")
	t.line("exit %d", r.Exit)
	return b.Flush()
}

type text struct {
	w *bufio.Writer
	r *Report
}

func (t *text) line(format string, args ...any) {
	fmt.Fprintf(t.w, format, args...)
	t.w.WriteByte('\n')
}

// section starts a list of findings with a blank line and its title.
func (t *text) section(title string, n int) {
	t.line("")
	t.line("%s (%d)", title, n)
}

func (t *text) summary() {
	r := t.r
	m := r.Matrix
	if m == nil {
		t.line("flinch: stopped at Contract errors, before generating any mutant")
		t.line("  %s", t.scope())
		t.line("  %s", t.build())
		return
	}
	c := count(m)
	head := fmt.Sprintf("flinch: %s in covered code", plural(c.Mutants, "mutant", "mutants"))
	if r.Unviable > 0 {
		head += fmt.Sprintf(", plus %d unviable that never built", r.Unviable)
	}
	t.line("%s", head)
	t.line("  %d killed, %d unheld (%d lived, %d unreached, %d erased), %d skipped, %d no verdict",
		c.Killed, c.Unheld, c.Lived, c.Unreached, c.Erased, c.Skipped, c.NoVerdict)
	t.line("  %d declared: %d equivalent, %d unpromised, %d by wildcard", c.Declared, c.Equivalent, c.Unpromised, c.Wildcard)
	if s, ok := c.score(); ok {
		t.line("  score %.1f%%, killed / (killed + unheld) = %d / %d, for information only", s, c.Killed, c.Killed+c.Unheld)
	} else {
		t.line("  score none, with no killed or unheld mutants; it's for information only")
	}

	obs := fmt.Sprintf("  %s: %d hold code, %d hollow, %d held only by crashes, %d hold no mutant alone",
		plural(c.Obligations, "obligation", "obligations"), c.Holding, c.Hollow, c.CrashOnly, c.NoSole)
	if c.Unjudged > 0 {
		obs += fmt.Sprintf("; %d hold nothing in this scoped run and aren't judged", c.Unjudged)
	}
	t.line("%s", obs)
	t.soleHolds(m.Obligations)
	t.line("  %s: %d blind", plural(c.Tests, "Contract test", "Contract tests"), c.Blind)
	t.line("  %s", t.scope())
	t.line("  %s", t.build())
}

// soleHolds prints each obligation's sole-hold count, one line per primitive, in Contract order.
func (t *text) soleHolds(os []matrix.Obligation) {
	if len(os) == 0 {
		return
	}
	t.line("  sole holds per obligation:")
	var prim string
	var parts []string
	flush := func() {
		if len(parts) > 0 {
			t.line("    %s  %s", prim, strings.Join(parts, ", "))
		}
	}
	for _, o := range os {
		p, id := splitID(o.ID)
		if p != prim {
			flush()
			prim, parts = p, nil
		}
		parts = append(parts, fmt.Sprintf("%s %d", id, len(o.SoleHolds)))
	}
	flush()
}

func (t *text) scope() string {
	switch {
	case t.r.Since != "":
		return "scoped run, --since " + t.r.Since
	case t.r.Scoped:
		return "scoped run"
	default:
		return "full run"
	}
}

func (t *text) build() string {
	tags := "none"
	if len(t.r.Tags) > 0 {
		tags = strings.Join(t.r.Tags, ",")
	}
	return fmt.Sprintf("flinch %s, %s %s/%s, tags %s", t.r.Version, t.r.GoVersion, t.r.GOOS, t.r.GOARCH, tags)
}

func (t *text) contractErrors() {
	if len(t.r.Problems) == 0 {
		return
	}
	ps := append([]problem.Problem(nil), t.r.Problems...)
	problem.Sort(ps)
	t.section("Contract errors, each cleared by fixing the file at the line it names", len(ps))
	for _, p := range ps {
		t.line("  %s", p)
	}
}

// byPrimitive orders mutants by primitive, then file, line, column and id.
func byPrimitive(ms []matrix.Mutant) {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].Primitive != ms[j].Primitive {
			return ms[i].Primitive < ms[j].Primitive
		}
		return lessMutant(ms[i], ms[j])
	})
}

func (t *text) unheld() {
	var ms []matrix.Mutant
	for _, m := range t.r.Matrix.Mutants {
		if m.Status.Unheld() {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		return
	}
	byPrimitive(ms)
	skipped := t.skippedIn()
	t.section("Unheld", len(ms))
	for _, m := range ms {
		t.mutantHead(m)
		file := t.r.mutantsFile(m.Primitive)
		switch m.Status {
		case model.Lived:
			t.line("    lived: %s, which passed", t.ranBy(m))
			t.line("    to clear: strengthen those tests, write the obligation this change breaks, or declare it in %s", file)
		case model.Unreached:
			t.line("    unreached: no Contract test runs this line")
			t.line("    to clear: write the promise that needs this code, delete the code, or declare it unpromised in %s", file)
		case model.Erased:
			note := ""
			if n := skipped[m.Dir+"."+m.Top]; n > 0 {
				note = "; " + plural(n, "finer mutant", "finer mutants") + " skipped"
			}
			t.line("    erased: %s, and none noticed it returning zero values from its first line%s", t.ranBy(m), note)
			t.line("    to clear: write the obligation that needs this function, delete it, or declare it unpromised in %s", file)
		}
	}
}

// skippedIn counts skipped mutants by "dir.Top", to say how many finer mutants an erased function hid.
func (t *text) skippedIn() map[string]int {
	n := map[string]int{}
	for _, m := range t.r.Matrix.Mutants {
		if m.Status == model.Skipped {
			n[m.Dir+"."+m.Top]++
		}
	}
	return n
}

func (t *text) mutantHead(m matrix.Mutant) {
	t.line("  %s  %s:%d  %s", m.Primitive, m.File, m.Line, m.Hash)
	t.line("    %s", m.ID)
}

// ranBy names the obligations whose tests ran a mutant, then the tests themselves. A test of the
// mutant's own primitive goes by its bare name; any other carries its primitive.
func (t *text) ranBy(m matrix.Mutant) string {
	if len(m.RanBy) == 0 {
		return "no Contract test runs this line"
	}
	var names []string
	for _, ref := range m.RanBy {
		names = append(names, testName(ref, m.Primitive))
	}
	obs := strings.Join(m.Obligations, ", ")
	if obs == "" {
		obs = "no obligation"
	}
	return fmt.Sprintf("run by %s (%s)", obs, strings.Join(names, ", "))
}

func testName(ref model.TestRef, primitive string) string {
	if ref.Primitive == primitive {
		return ref.Name
	}
	return ref.String()
}

func (t *text) noVerdict() {
	var ms []matrix.Mutant
	for _, m := range t.r.Matrix.Mutants {
		if m.Status == model.NoVerdict {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		return
	}
	byPrimitive(ms)
	t.section("No verdict", len(ms))
	for _, m := range ms {
		t.mutantHead(m)
		t.line("    no verdict: %s", m.Verdict)
		t.line("    to clear: fix what kept it from running to a verdict and run flinch again; until then the run exits 2")
	}
}

func (t *text) broken() {
	if len(t.r.Matrix.Broken) == 0 {
		return
	}
	ps := append([]problem.Problem(nil), t.r.Matrix.Broken...)
	problem.Sort(ps)
	t.section("Broken declarations, each cleared by deleting its line", len(ps))
	for _, p := range ps {
		t.line("  %s", p)
	}
}

func (t *text) hollow() {
	var os []matrix.Obligation
	for _, o := range t.r.Matrix.Obligations {
		if o.Hollow {
			os = append(os, o)
		}
	}
	if len(os) == 0 {
		return
	}
	t.section("Hollow obligations", len(os))
	for _, o := range os {
		reached := 0
		for _, m := range t.r.Matrix.Mutants {
			if ranAny(m.RanBy, o.Tests) {
				reached++
			}
		}
		switch {
		case len(o.Tests) == 0:
			t.line("  %s  no Contract test names it", o.ID)
		case reached == 0 && len(o.Tests) == 1:
			t.line("  %s  its test reaches no mutant", o.ID)
		case reached == 0:
			t.line("  %s  its %d tests reach no mutant", o.ID, len(o.Tests))
		default:
			t.line("  %s  %s ran against %s and killed none", o.ID, plural(len(o.Tests), "test", "tests"), plural(reached, "mutant", "mutants"))
		}
		t.line("    to clear: rewrite its tests to check what its code does, or drop an obligation that promises nothing")
	}
}

func ranAny(ran, tests []model.TestRef) bool {
	for _, a := range ran {
		for _, b := range tests {
			if a == b {
				return true
			}
		}
	}
	return false
}

func (t *text) crashOnly() {
	var os []matrix.Obligation
	for _, o := range t.r.Matrix.Obligations {
		if o.CrashOnly {
			os = append(os, o)
		}
	}
	if len(os) == 0 {
		return
	}
	t.section("Held only by crashes, which never fails the build", len(os))
	for _, o := range os {
		t.line("  %s  every kill by its tests was a panic or a timeout, none an assertion", o.ID)
		t.line("    to clear: give its tests assertions about results")
	}
}

func (t *text) outsideOnly() {
	var ms []matrix.Mutant
	for _, m := range t.r.Matrix.Mutants {
		if m.OutsideOnly {
			ms = append(ms, m)
		}
	}
	if len(ms) == 0 {
		return
	}
	ms = sortedMutants(ms)
	t.section("Held only from outside, which never fails the build", len(ms))
	for _, m := range ms {
		t.mutantHead(m)
		var killers, obs []string
		seen := map[string]bool{}
		for _, k := range m.Kills {
			killers = append(killers, k.Test.String())
			for _, o := range k.Obligations {
				if !seen[o] {
					seen[o] = true
					obs = append(obs, o)
				}
			}
		}
		sort.Strings(obs)
		t.line("    killed only by %s (%s)", strings.Join(killers, ", "), strings.Join(obs, ", "))
		t.line("    to clear: give %s an obligation whose test fails against this change", m.Primitive)
	}
}

func (t *text) blind() {
	var ts []matrix.Test
	for _, x := range t.r.Matrix.Tests {
		if x.Blind {
			ts = append(ts, x)
		}
	}
	if len(ts) == 0 {
		return
	}
	t.section("Blind Contract tests", len(ts))
	for _, x := range ts {
		t.line("  %s:%d  %s (%s)", x.File, x.Line, x.Name, strings.Join(x.Obligations, ", "))
		t.line("    ran in %s of undeclared mutants and killed none", plural(x.Ran, "complete row", "complete rows"))
		t.line("    to clear: give it an assertion that can fail, or delete it")
	}
}

func (t *text) outside() {
	if len(t.r.Outside) == 0 {
		return
	}
	ps := append([]string(nil), t.r.Outside...)
	sort.Strings(ps)
	t.section("Outside the Contract, which never fails the build", len(ps))
	for _, p := range ps {
		t.line("  %s", p)
	}
	t.line("  to bring one in, name it in a primitive's covers block")
}
