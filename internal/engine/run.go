package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/tools4imps/flinch/internal/gate"
	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/mutate"
	"github.com/tools4imps/flinch/internal/report"
	"github.com/tools4imps/flinch/internal/runner"
	"github.com/tools4imps/flinch/internal/since"
	"github.com/tools4imps/flinch/internal/typecheck"
	"github.com/tools4imps/flinch/internal/units"
	"github.com/tools4imps/flinch/internal/version"
)

// RunOptions are the settings for a full run, on top of what the static check needs.
type RunOptions struct {
	Options
	Since       string   // a git ref; empty for a run over all covered code
	Only        []string // primitives to mutate; empty for all of them
	Operators   []string // empty for the defaults
	Jobs        int
	Coefficient float64
	MemoryLimit int64 // bytes one test process may hold; zero for no limit
	Progress    io.Writer
}

// A Plan is the set of mutants a run would build, before any test runs.
type Plan struct {
	Static   *Static
	Mutants  []model.Mutant
	Unviable int
	Full     bool            // every covered unit, every default operator
	Whole    map[string]bool // primitives whose covered code the plan mutates whole
	Units    map[string]bool // "dir.Top" units the plan mutates whole
	// spans holds each function's body, so an erase mutant is reached by any test that ran any of it.
	spans map[string]span
}

type span struct {
	file                    string
	fromLine, fromCol       int
	toLine, toCol, lastLine int
}

// Prepare runs the static check and, when the Contract is sound, generates the mutants in scope.
// A Contract with problems gives a Plan with no mutants: nothing else can be trusted until the
// problems are fixed.
func Prepare(ctx context.Context, o RunOptions) (*Plan, error) {
	// A run started by another run's Contract tests builds everything in the outer run's cache. Each
	// such run works on a fresh copy of some module, at a new path, and its builds would otherwise
	// pile up in the user's cache under keys nothing ever uses again.
	if outer := os.Getenv("FLINCH_GOCACHE"); outer != "" {
		os.Setenv("GOCACHE", outer)
	}
	st, err := Check(o.Options)
	if err != nil {
		return nil, err
	}
	p := &Plan{
		Static: st, Whole: map[string]bool{}, Units: map[string]bool{},
		spans: map[string]span{},
	}
	if len(st.Problems) > 0 {
		return p, nil
	}
	for _, name := range o.Only {
		if st.Contract.Primitive(name) == nil {
			return nil, fmt.Errorf("--only %s: the Contract has no primitive by that name", name)
		}
	}
	ops := o.Operators
	if len(ops) == 0 {
		ops = mutate.Defaults
	}
	for _, op := range ops {
		if !slices.Contains(mutate.Defaults, op) {
			return nil, fmt.Errorf("--operators: %s is not an operator; flinch operators lists them", op)
		}
	}
	defaultOps := len(ops) == len(mutate.Defaults)

	dirs := st.Ownership.Covered()
	importPaths := make([]string, 0, len(dirs))
	byImport := map[string]string{}
	for _, pkg := range st.Packages {
		if slices.Contains(dirs, pkg.Dir) {
			importPaths = append(importPaths, pkg.ImportPath)
			byImport[pkg.ImportPath] = pkg.Dir
		}
	}
	if len(importPaths) == 0 {
		p.Full = len(o.Only) == 0 && o.Since == "" && defaultOps
		return p, nil
	}
	loader, typed, err := typecheck.Load(ctx, st.Module.Root, o.Tags, importPaths)
	if err != nil {
		return nil, err
	}
	var all []model.Mutant
	for _, ip := range importPaths {
		tp := typed[ip]
		if tp == nil {
			continue
		}
		dir := byImport[ip]
		mutable := map[string]bool{}
		for _, pkg := range st.Packages {
			if pkg.Dir == dir {
				for _, f := range pkg.Mutable {
					mutable[f] = true
				}
			}
		}
		owner := func(top string) (string, bool) { return st.Ownership.Owner(dir, top) }
		ms, unviable := mutate.Generate(loader, tp, mutable, owner, ops)
		all = append(all, ms...)
		p.Unviable += unviable
		for _, u := range units.Of(tp.Files, tp.Names) {
			if u.Body == nil || u.Kind == units.Closure {
				continue
			}
			from := tp.Fset.Position(u.Body.Lbrace)
			to := tp.Fset.Position(u.Body.Rbrace)
			p.spans[dir+"."+u.Name] = span{
				file: u.File, fromLine: from.Line, fromCol: from.Column,
				toLine: to.Line, toCol: to.Column + 1, lastLine: to.Line,
			}
		}
	}

	keep := func(m model.Mutant) bool { return true }
	if len(o.Only) > 0 {
		only := o.Only
		keep = func(m model.Mutant) bool { return slices.Contains(only, m.Primitive) }
	}
	if o.Since != "" {
		ch, err := since.Changed(ctx, st.Module.Root, o.Since)
		if err != nil {
			return nil, err
		}
		reproved := map[string]bool{}
		for _, prim := range st.Contract.Primitives {
			if ch.TouchesDir(prim.Dir) {
				reproved[prim.Name] = true
			}
		}
		before := keep
		keep = func(m model.Mutant) bool {
			if !before(m) {
				return false
			}
			if reproved[m.Primitive] {
				return true
			}
			if m.Erase {
				// A function with any changed line gets its erase mutant, so the erase-first order holds.
				if s, ok := p.spans[m.Dir+"."+m.Top]; ok {
					for line := s.fromLine; line <= s.lastLine; line++ {
						if ch.Touches(m.File, line) {
							return true
						}
					}
				}
			}
			return ch.Touches(m.File, m.Line)
		}
		for name := range reproved {
			if len(o.Only) == 0 || slices.Contains(o.Only, name) {
				p.Whole[name] = defaultOps
			}
		}
	} else {
		for _, prim := range st.Contract.Primitives {
			if len(o.Only) == 0 || slices.Contains(o.Only, prim.Name) {
				p.Whole[prim.Name] = defaultOps
			}
		}
	}

	generated := map[string]int{}
	kept := map[string]int{}
	for _, m := range all {
		key := m.Dir + "." + m.Top
		generated[key]++
		if keep(m) {
			kept[key]++
			p.Mutants = append(p.Mutants, m)
		}
	}
	for key, n := range generated {
		if kept[key] == n && defaultOps {
			p.Units[key] = true
		}
	}
	p.Full = len(o.Only) == 0 && o.Since == "" && defaultOps
	return p, nil
}

// Run carries a plan through the runner, the matrix and the gate, and returns the report.
func Run(ctx context.Context, o RunOptions) (*report.Report, error) {
	started := time.Now()
	timing := map[string]time.Duration{}
	env, err := goEnv(ctx)
	if err != nil {
		return nil, err
	}
	rep := &report.Report{
		Version: version.Version, GoVersion: env["GOVERSION"], GOOS: env["GOOS"], GOARCH: env["GOARCH"],
		Tags: o.Tags, Since: o.Since, Timing: timing, Contract: o.Contract,
	}
	plan, err := Prepare(ctx, o)
	if err != nil {
		return nil, err
	}
	st := plan.Static
	timing["plan"] = time.Since(started)
	rep.Problems = st.Problems
	rep.Scoped = !plan.Full
	rep.Unviable = plan.Unviable
	if st.Ownership != nil {
		rep.Outside = st.Ownership.Outside()
	}
	if len(st.Problems) > 0 {
		rep.Exit = gate.Decide(st.Problems, nil, "", plan.Full).Exit
		return rep, nil
	}

	work, err := os.MkdirTemp("", "flinch-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	progress := o.Progress
	if progress == nil {
		progress = io.Discard
	}
	ropts := runner.Options{
		Root: st.Module.Root, Tags: o.Tags, Jobs: o.Jobs, Coefficient: o.Coefficient,
		MemoryLimit: o.MemoryLimit, Progress: progress, Work: work,
	}

	var coverpkg []string
	for _, pkg := range st.Packages {
		if slices.Contains(st.Ownership.Covered(), pkg.Dir) {
			coverpkg = append(coverpkg, pkg.ImportPath)
		}
	}
	base, err := runner.Prove(ctx, ropts, suites(st.Tests), coverpkg)
	timing["prove"] = time.Since(started) - timing["plan"]
	if undecided := asUndecided(err); undecided != "" {
		rep.Undecided = undecided
		rep.Exit = 2
		return rep, nil
	} else if err != nil {
		return nil, err
	}

	reached := map[string][]model.TestRef{}
	for _, m := range plan.Mutants {
		var refs []model.TestRef
		switch {
		case m.Linked:
			refs = base.Linking(m.ImportPath)
		case m.Erase:
			if s, ok := plan.spans[m.Dir+"."+m.Top]; ok {
				refs = base.ReachSpan(s.file, s.fromLine, s.fromCol, s.toLine, s.toCol)
			}
		default:
			refs = base.Reach(m.File, m.Line, m.Col)
		}
		if len(refs) > 0 {
			reached[m.Hash] = refs
		}
	}

	// Erase mutants go first. A function whose erase mutant lives is reported once, and its finer
	// mutants are skipped.
	rows := map[string]model.Row{}
	var first []runner.Job
	for _, m := range plan.Mutants {
		if m.Erase && reached[m.Hash] != nil {
			first = append(first, runner.Job{Mutant: m, Tests: reached[m.Hash]})
		}
	}
	got, err := base.Run(ctx, first)
	if undecided := asUndecided(err); undecided != "" {
		rep.Undecided = undecided
		rep.Exit = 2
		return rep, nil
	} else if err != nil {
		return nil, err
	}
	erased := map[string]bool{}
	for _, m := range plan.Mutants {
		if !m.Erase {
			continue
		}
		if row, ok := got[m.Hash]; ok {
			rows[m.Hash] = row
			if len(row.Kills) == 0 && row.Verdict == "" {
				erased[m.Dir+"."+m.Top] = true
			}
		}
	}
	var second []runner.Job
	for _, m := range plan.Mutants {
		if !m.Erase && reached[m.Hash] != nil && !erased[m.Dir+"."+m.Top] {
			second = append(second, runner.Job{Mutant: m, Tests: reached[m.Hash]})
		}
	}
	got, err = base.Run(ctx, second)
	if undecided := asUndecided(err); undecided != "" {
		rep.Undecided = undecided
		rep.Exit = 2
		return rep, nil
	} else if err != nil {
		return nil, err
	}
	for hash, row := range got {
		rows[hash] = row
	}
	timing["mutants"] = time.Since(started) - timing["plan"] - timing["prove"]

	res := matrix.Compute(matrix.Input{
		Obligations:  obligations(st),
		Tests:        st.Tests,
		Mutants:      plan.Mutants,
		Rows:         rows,
		Reached:      reached,
		Declarations: declarations(st),
		Full:         plan.Full,
		Whole:        plan.Whole,
		WholeUnits:   plan.Units,
	})
	rep.Matrix = res
	rep.Exit = gate.Decide(st.Problems, res, "", plan.Full).Exit
	timing["total"] = time.Since(started)
	return rep, nil
}

func asUndecided(err error) string {
	var u *runner.Undecided
	if errors.As(err, &u) {
		return u.Reason
	}
	return ""
}

// suites groups the Contract tests by primitive, in the order the runner builds them.
func suites(tests []model.Test) []runner.Suite {
	var out []runner.Suite
	index := map[string]int{}
	for _, t := range tests {
		i, ok := index[t.Primitive]
		if !ok {
			i = len(out)
			index[t.Primitive] = i
			out = append(out, runner.Suite{Primitive: t.Primitive, Dir: t.Dir, ImportPath: t.ImportPath})
		}
		out[i].Tests = append(out[i].Tests, t.Name)
	}
	return out
}

func obligations(st *Static) []string {
	var out []string
	for _, prim := range st.Contract.Primitives {
		for _, ob := range prim.Obligations {
			out = append(out, prim.Name+"/"+ob.ID)
		}
	}
	return out
}

// declarations converts the Contract's declarations for the matrix. The static check has already
// rejected any line that doesn't parse, so a parse failure here is skipped.
func declarations(st *Static) []matrix.Declaration {
	var out []matrix.Declaration
	for _, prim := range st.Contract.Primitives {
		for _, d := range prim.Declarations {
			pat, err := mutantid.Parse(d.Text)
			if err != nil {
				continue
			}
			md := matrix.Declaration{
				Kind: d.Kind, Primitive: prim.Name, Path: d.Path, Line: d.Line, Reason: d.Reason,
				Wildcard: pat.Wildcard, Dir: pat.Dir, Unit: pat.Unit,
			}
			if !pat.Wildcard {
				md.ID = pat.ID.String()
			}
			out = append(out, md)
		}
	}
	return out
}

// goEnv reads the toolchain's version and target, which the report records because they decide
// which files build.
func goEnv(ctx context.Context) (map[string]string, error) {
	out, err := exec.CommandContext(ctx, "go", "env", "GOVERSION", "GOOS", "GOARCH").Output()
	if err != nil {
		return nil, fmt.Errorf("flinch needs the go command on PATH: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	env := map[string]string{}
	for i, key := range []string{"GOVERSION", "GOOS", "GOARCH"} {
		if i < len(lines) {
			env[key] = strings.TrimSpace(lines[i])
		}
	}
	return env, nil
}
