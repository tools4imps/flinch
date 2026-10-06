package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tools4imps/flinch/internal/model"
)

// Restarts after a crash continue while they settle tests. A row gets no verdict once this many
// restarts have settled nothing, which happens when the binary hangs where Go's own timeout can't
// reach and no test can take the blame.
const maxRestarts = 3

// Run builds each job's mutant through -overlay and runs the Contract tests that reach it. It returns a
// row for every job, keyed by mutant hash. An error means *Undecided, such as a batch of tests that
// fails against the clean code, or ctx ending.
func (b *Baseline) Run(ctx context.Context, jobs []Job) (map[string]model.Row, error) {
	rows := make([]model.Row, len(jobs))
	t := &ticker{total: len(jobs)}
	err := parallel(ctx, b.o.Jobs, len(jobs), func(ctx context.Context, i int) error {
		row, err := b.job(ctx, jobs[i])
		if err != nil {
			return err
		}
		rows[i] = row
		b.tick(t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.Row, len(jobs))
	for i, j := range jobs {
		out[j.Mutant.Hash] = rows[i]
	}
	return out, nil
}

// An outcome is how one top-level test ended against a mutant.
type outcome struct {
	fail bool
	kind model.KillKind
	subs []string
}

// A batchCheck remembers the clean run of one batch, so each batch runs clean once however many
// mutants share it.
type batchCheck struct {
	mu   sync.Mutex
	done bool
	err  error
}

// group is one suite's share of a job.
type group struct {
	s     *suite
	tests []string
	bin   string
}

func (b *Baseline) job(ctx context.Context, j Job) (model.Row, error) {
	// The caller passes only reached mutants, and a mutant no test reaches is never built.
	if len(j.Tests) == 0 {
		return model.Row{Complete: true}, nil
	}
	groups, err := b.groups(j.Tests)
	if err != nil {
		return model.Row{}, err
	}
	for _, g := range groups {
		if err := b.checkBatch(ctx, g.s, g.tests); err != nil {
			return model.Row{}, err
		}
	}

	dir, err := b.jobDir()
	if err != nil {
		return model.Row{}, &Undecided{Reason: "can't make a work directory: " + err.Error()}
	}
	defer os.RemoveAll(dir)
	overlay, mutated, err := b.overlay(dir, j.Mutant)
	if err != nil {
		return model.Row{}, err
	}
	for _, g := range groups {
		g.bin = filepath.Join(dir, fmt.Sprintf("%d.test", g.s.n))
		if err := b.buildMutant(ctx, g.s.Dir, g.bin, overlay); err != nil {
			var ge *goError
			if !errors.As(err, &ge) {
				return model.Row{}, b.undecidedRun(err)
			}
			return model.Row{Verdict: "doesn't build: " + b.unmask(firstError(err), mutated, j.Mutant.File)}, nil
		}
	}

	res := map[model.TestRef]*outcome{}
	verdict := ""
	for _, g := range groups {
		outs, nv, err := b.batch(ctx, g.s, g.bin, g.tests, 1)
		if err != nil {
			return model.Row{}, err
		}
		for name, o := range outs {
			res[model.TestRef{Primitive: g.s.Primitive, Name: name}] = o
		}
		if verdict == "" {
			verdict = nv
		}
	}

	if verdict == "" && onlyTimeouts(res) {
		if verdict, err = b.confirm(ctx, groups, res); err != nil {
			return model.Row{}, err
		}
	}
	return row(groups, res, verdict), nil
}

// confirm reruns the tests that timed out, with twice the budget, when timeouts are a row's only
// kills. A kill that came only from timeouts might come from a slow machine rather than the mutant,
// so it counts only once the rerun times out or fails as well; a test that passes the rerun is no
// killer. It returns a reason for no verdict when the rerun can't settle a test.
func (b *Baseline) confirm(ctx context.Context, groups []*group, res map[model.TestRef]*outcome) (string, error) {
	verdict := ""
	for _, g := range groups {
		var again []string
		for _, t := range g.tests {
			if o := res[model.TestRef{Primitive: g.s.Primitive, Name: t}]; o != nil && o.fail {
				again = append(again, t)
			}
		}
		if len(again) == 0 {
			continue
		}
		outs, nv, err := b.batch(ctx, g.s, g.bin, again, 2)
		if err != nil {
			return "", err
		}
		for _, t := range again {
			ref := model.TestRef{Primitive: g.s.Primitive, Name: t}
			if o := outs[t]; o != nil {
				res[ref] = o
			} else {
				delete(res, ref)
			}
		}
		if verdict == "" {
			verdict = nv
		}
	}
	return verdict, nil
}

// groups splits a job's tests by suite, in suite order with names sorted, so a batch is the same
// whichever order the caller listed the tests in.
func (b *Baseline) groups(tests []model.TestRef) ([]*group, error) {
	by := map[*suite]map[string]bool{}
	for _, t := range tests {
		s := b.byPrim[t.Primitive]
		if s == nil {
			return nil, &Undecided{Reason: fmt.Sprintf("no Contract suite holds %s", t)}
		}
		if by[s] == nil {
			by[s] = map[string]bool{}
		}
		by[s][t.Name] = true
	}
	var gs []*group
	for s, names := range by {
		gs = append(gs, &group{s: s, tests: sortedKeys(names)})
	}
	sort.Slice(gs, func(i, j int) bool { return gs[i].s.n < gs[j].s.n })
	return gs, nil
}

// checkBatch runs a batch once against the clean binary before any mutant uses it. Tests that pass in
// their whole suite and alone can still fail in some smaller company, and a kill that comes from the
// company rather than the mutant would be a lie.
func (b *Baseline) checkBatch(ctx context.Context, s *suite, tests []string) error {
	key := s.Primitive + "\x00" + strings.Join(tests, "\x00")
	b.mu.Lock()
	c := b.batches[key]
	if c == nil {
		c = &batchCheck{}
		b.batches[key] = c
	}
	b.mu.Unlock()
	// Workers that need the same batch wait here for the first one's answer.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return c.err
	}
	p, err := b.runTests(ctx, s, s.clean, tests, b.budget(s, tests, 1))
	if err != nil {
		// A run that ctx stopped proves nothing, so a later Run checks the batch again.
		return b.undecidedRun(err)
	}
	names := make([]string, len(tests))
	for i, t := range tests {
		names[i] = s.Primitive + "/" + t
	}
	c.done = true
	c.err = judgeClean(p, s, tests, "when run in one process with "+strings.Join(names, ", "))
	return c.err
}

// batch runs tests against a mutant's binary until each has an outcome. When a crash ends the process
// early, the tests it names are killers and the tests that hadn't finished start again in a fresh
// process. A crash that names no test sends the unfinished tests one per process, where a crash can
// only be the lone test's doing. It returns a reason for no verdict when restarts stop settling
// anything.
func (b *Baseline) batch(ctx context.Context, s *suite, bin string, tests []string, scale float64) (map[string]*outcome, string, error) {
	out := map[string]*outcome{}
	pending := tests
	single := false
	stalls := 0
	for len(pending) > 0 {
		sets := [][]string{pending}
		if single {
			sets = nil
			for _, t := range pending {
				sets = append(sets, []string{t})
			}
		}
		settled, unnamed := 0, false
		for _, set := range sets {
			p, err := b.runTests(ctx, s, bin, set, b.budget(s, set, scale))
			if err != nil {
				return nil, "", b.undecidedRun(err)
			}
			n, named := settle(p, set, out)
			settled += n
			if !named {
				unnamed = true
			}
		}
		pending = unsettled(tests, out)
		if len(pending) == 0 {
			break
		}
		if unnamed {
			single = true
		}
		if settled == 0 {
			stalls++
			if stalls > maxRestarts {
				names := make([]string, len(pending))
				for i, t := range pending {
					names[i] = s.Primitive + "/" + t
				}
				return out, fmt.Sprintf("no verdict: %s never finished in %d restarts", strings.Join(names, ", "), maxRestarts), nil
			}
		}
	}
	return out, "", nil
}

// settle records the outcomes one process gave the tests it was asked to run. It reports how many
// tests it settled, and whether the process ended normally or its crash named a test.
func settle(p *proc, set []string, out map[string]*outcome) (int, bool) {
	n := 0
	kill := func(t string, kind model.KillKind) {
		out[t] = &outcome{fail: true, kind: kind, subs: p.failedSubtests(t)}
		n++
	}
	for _, t := range set {
		switch p.results[t] {
		case "pass", "skip":
			out[t] = &outcome{}
			n++
		case "fail":
			kind := model.Assertion
			if p.panicked(t) {
				kind = model.Panic
			}
			kill(t, kind)
		}
	}
	left := unsettled(set, out)
	switch {
	case len(left) == 0:
		return n, true
	case p.ended:
		// The process finished normally without running some of the tests. Nothing crashed, so
		// there's no one to blame, and a rerun is the only way on.
		return n, false
	case !p.anyStarted():
		// The process died before its first test started, as with a panic in an init function.
		// Every test it was asked to run would have died the same way.
		kind := model.Panic
		if p.guard {
			kind = model.Timeout
		}
		for _, t := range left {
			kill(t, kind)
		}
		return n, true
	case p.timeout:
		named := false
		for _, t := range p.timedOut {
			if out[t] == nil && contains(set, t) {
				kill(t, model.Timeout)
				named = true
			}
		}
		return n, named
	case p.guard:
		// Go's own timeout should have fired before the guard did, so the binary hung somewhere
		// that says nothing about which test was running.
		return n, false
	case len(set) == 1:
		// The lone test started and the process died under it, through a fatal error such as a
		// stack overflow, or an os.Exit.
		kill(set[0], model.Panic)
		return n, true
	}
	for _, t := range set {
		if o := out[t]; o != nil && o.fail && o.kind == model.Panic {
			return n, true
		}
	}
	return n, false
}

func unsettled(tests []string, out map[string]*outcome) []string {
	var left []string
	for _, t := range tests {
		if out[t] == nil {
			left = append(left, t)
		}
	}
	return left
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func onlyTimeouts(res map[model.TestRef]*outcome) bool {
	found := false
	for _, o := range res {
		if o.fail {
			if o.kind != model.Timeout {
				return false
			}
			found = true
		}
	}
	return found
}

// row assembles a mutant's row with tests and kills in a fixed order.
func row(groups []*group, res map[model.TestRef]*outcome, verdict string) model.Row {
	r := model.Row{Complete: verdict == "", Verdict: verdict}
	for _, g := range groups {
		for _, t := range g.tests {
			ref := model.TestRef{Primitive: g.s.Primitive, Name: t}
			o := res[ref]
			if o == nil {
				r.Complete = false
				continue
			}
			r.Ran = append(r.Ran, ref)
			if o.fail {
				r.Kills = append(r.Kills, model.Kill{Test: ref, Subtests: o.subs, Kind: o.kind})
			}
		}
	}
	sortRefs(r.Ran)
	sort.Slice(r.Kills, func(i, j int) bool {
		a, b := r.Kills[i].Test, r.Kills[j].Test
		if a.Primitive != b.Primitive {
			return a.Primitive < b.Primitive
		}
		return a.Name < b.Name
	})
	return r
}

// budget is how long a set of tests may run against a mutant: the sum of their lone run times, times
// the coefficient, plus a base, all times scale.
func (b *Baseline) budget(s *suite, tests []string, scale float64) time.Duration {
	var sum time.Duration
	for _, t := range tests {
		sum += b.lone[model.TestRef{Primitive: s.Primitive, Name: t}]
	}
	d := time.Duration(float64(sum)*b.o.Coefficient) + budgetBase
	return time.Duration(float64(d) * scale).Round(time.Millisecond)
}

func (b *Baseline) jobDir() (string, error) {
	b.mu.Lock()
	b.seq++
	n := b.seq
	b.mu.Unlock()
	dir := filepath.Join(b.o.Work, "m", fmt.Sprint(n))
	return dir, os.MkdirAll(dir, 0o755)
}

// overlay writes the mutated file under dir and an overlay file that swaps it in for the original. It
// returns the paths of both. The module itself is never written, so an interrupted run leaves it as it
// was.
func (b *Baseline) overlay(dir string, m model.Mutant) (string, string, error) {
	orig := filepath.Join(b.o.Root, filepath.FromSlash(m.File))
	src, err := os.ReadFile(orig)
	if err != nil {
		return "", "", &Undecided{Reason: fmt.Sprintf("can't read %s for mutant %s: %v", m.File, m.ID, err)}
	}
	if m.Start < 0 || m.Start > m.End || m.End > len(src) {
		return "", "", &Undecided{Reason: fmt.Sprintf("mutant %s doesn't fit %s, which may have changed during the run", m.ID, m.File)}
	}
	mutated := filepath.Join(dir, filepath.Base(orig))
	if err := os.WriteFile(mutated, m.Apply(src), 0o644); err != nil {
		return "", "", &Undecided{Reason: "can't write a mutant: " + err.Error()}
	}
	data, err := json.Marshal(map[string]map[string]string{"Replace": {orig: mutated}})
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", "", &Undecided{Reason: "can't write an overlay: " + err.Error()}
	}
	return path, mutated, nil
}

// unmask puts the module path of a mutated file back into a compiler message. The compiler names the
// replacement file under Work, a path that changes on every run and means nothing to the reader.
func (b *Baseline) unmask(msg, mutated, file string) string {
	if rel, err := filepath.Rel(b.o.Root, mutated); err == nil {
		msg = strings.ReplaceAll(msg, rel, file)
	}
	return strings.ReplaceAll(msg, mutated, file)
}

// A ticker paces progress lines.
type ticker struct {
	total, done int
	last        time.Time
}

func (b *Baseline) tick(t *ticker) {
	if b.o.Progress == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t.done++
	if t.done < t.total && time.Since(t.last) < 2*time.Second {
		return
	}
	t.last = time.Now()
	fmt.Fprintf(b.o.Progress, "flinch: %d/%d mutants\n", t.done, t.total)
}
