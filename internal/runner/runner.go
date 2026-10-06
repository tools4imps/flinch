// Package runner runs the Contract suite against the clean code and then against each mutant. Prove
// checks that every Contract test passes, whole and alone, and records what each one reaches. Run
// builds each mutant through -overlay, so the working tree is never touched, and records which tests
// kill it and how.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tools4imps/flinch/internal/model"
)

// Options configures a run.
type Options struct {
	Root        string // module root, absolute
	Tags        []string
	Jobs        int
	Coefficient float64 // default 10
	Progress    io.Writer
	Work        string // a temp directory the runner owns
	// MemoryLimit is the most memory, in bytes, one test process may hold before flinch stops it.
	// Some mutants make code allocate without end, and one such process can take the whole machine
	// down with it. Zero turns the guard off.
	MemoryLimit int64
}

// A Suite is one primitive's Contract tests, which build into one test binary.
type Suite struct {
	Primitive, Dir, ImportPath string
	Tests                      []string // top-level Test, Example and Fuzz names
}

// Undecided is the error for a run flinch can't judge, such as a Contract test that fails against the
// clean code. It means exit 2.
type Undecided struct{ Reason string }

func (u *Undecided) Error() string { return u.Reason }

// A Job is one mutant and the Contract tests that reach it.
type Job struct {
	Mutant model.Mutant
	Tests  []model.TestRef
}

// A Baseline is what the clean runs recorded: the suites' clean binaries, each test's lone run time,
// the coverage blocks each test reached, and the packages each suite's binary links.
type Baseline struct {
	o      Options
	cache  string // the build cache for mutant builds
	suites []*suite
	byPrim map[string]*suite
	lone   map[model.TestRef]time.Duration
	blocks map[string][]block         // by module-relative file
	links  map[string][]model.TestRef // by import path

	mu      sync.Mutex
	batches map[string]*batchCheck
	seq     int // numbers job directories, so two Runs never share one
}

type suite struct {
	Suite
	n     int    // its position, which names its files under Work
	abs   string // the package directory, absolute
	clean string // the clean test binary
	cover string // the coverage test binary, "" when nothing is covered
}

// Prove runs each suite whole and then each of its tests alone against the clean code. A test that
// fails either time, or a suite that doesn't build, returns *Undecided naming it. The lone runs record
// coverage of the coverpkg packages and how long each test takes.
func Prove(ctx context.Context, o Options, suites []Suite, coverpkg []string) (*Baseline, error) {
	if o.Jobs <= 0 {
		o.Jobs = runtime.GOMAXPROCS(0)
	}
	if o.Coefficient <= 0 {
		o.Coefficient = 10
	}
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return nil, err
	}
	o.Root = root
	b := &Baseline{
		o:       o,
		byPrim:  map[string]*suite{},
		lone:    map[model.TestRef]time.Duration{},
		blocks:  map[string][]block{},
		links:   map[string][]model.TestRef{},
		batches: map[string]*batchCheck{},
		cache:   mutantCache(o.Work),
	}
	for _, d := range []string{"clean", "cover", "m", "tmp"} {
		if err := os.MkdirAll(filepath.Join(o.Work, d), 0o755); err != nil {
			return nil, err
		}
	}
	for i, s := range suites {
		s.Tests = append([]string(nil), s.Tests...)
		sort.Strings(s.Tests)
		st := &suite{Suite: s, n: i, abs: filepath.Join(root, filepath.FromSlash(s.Dir))}
		st.clean = filepath.Join(o.Work, "clean", fmt.Sprintf("%d.test", i))
		b.suites = append(b.suites, st)
		b.byPrim[s.Primitive] = st
	}
	dirs, err := b.link(ctx)
	if err != nil {
		return nil, err
	}
	b.progress("flinch: running %d Contract suites whole", len(b.suites))
	if err := parallel(ctx, o.Jobs, len(b.suites), func(ctx context.Context, i int) error {
		return b.whole(ctx, b.suites[i])
	}); err != nil {
		return nil, err
	}
	if err := b.alone(ctx, coverpkg, dirs); err != nil {
		return nil, err
	}
	return b, nil
}

// link records which packages each suite's binary links, for Linking, and returns the module-relative
// directory of every package any suite links, for reading coverage profiles.
func (b *Baseline) link(ctx context.Context) (map[string]string, error) {
	sets := map[string]map[model.TestRef]bool{}
	dirs := map[string]string{}
	for _, s := range b.suites {
		ps, err := b.deps(ctx, s.Dir)
		if err != nil {
			return nil, undecidedGo(err, "go list can't load the Contract suite for "+s.Primitive)
		}
		for _, p := range ps {
			if sets[p.ImportPath] == nil {
				sets[p.ImportPath] = map[model.TestRef]bool{}
			}
			for _, t := range s.Tests {
				sets[p.ImportPath][model.TestRef{Primitive: s.Primitive, Name: t}] = true
			}
			rel, err := filepath.Rel(b.o.Root, p.Dir)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			dirs[p.ImportPath] = filepath.ToSlash(rel)
		}
	}
	for ip, set := range sets {
		b.links[ip] = sortedRefs(set)
	}
	return dirs, nil
}

// whole builds a suite's clean binary and runs every test in it in one process.
func (b *Baseline) whole(ctx context.Context, s *suite) error {
	if err := b.build(ctx, s.Dir, s.clean); err != nil {
		return undecidedGo(err, "the Contract suite for "+s.Primitive+" doesn't build")
	}
	p, err := b.runTests(ctx, s, s.clean, nil, defaultTimeout)
	if err != nil {
		return b.undecidedRun(err)
	}
	return judgeClean(p, s, s.Tests, "when its suite runs whole")
}

// alone runs each test in a process of its own, through a coverage build when coverpkg names any
// package, and records the blocks it reached and how long it took.
func (b *Baseline) alone(ctx context.Context, coverpkg []string, dirs map[string]string) error {
	if len(coverpkg) > 0 {
		err := parallel(ctx, b.o.Jobs, len(b.suites), func(ctx context.Context, i int) error {
			s := b.suites[i]
			s.cover = filepath.Join(b.o.Work, "cover", fmt.Sprintf("%d.test", s.n))
			err := b.build(ctx, s.Dir, s.cover, "-cover", "-covermode=set", "-coverpkg="+strings.Join(coverpkg, ","))
			if err != nil {
				return undecidedGo(err, "the Contract suite for "+s.Primitive+" doesn't build with coverage")
			}
			return os.MkdirAll(filepath.Join(b.o.Work, "cover", fmt.Sprint(s.n)), 0o755)
		})
		if err != nil {
			return err
		}
	}
	type lone struct {
		s      *suite
		name   string
		took   time.Duration
		blocks map[string][]span
	}
	var runs []*lone
	for _, s := range b.suites {
		for _, t := range s.Tests {
			runs = append(runs, &lone{s: s, name: t})
		}
	}
	b.progress("flinch: running %d Contract tests alone", len(runs))
	err := parallel(ctx, b.o.Jobs, len(runs), func(ctx context.Context, i int) error {
		r := runs[i]
		bin, extra := r.s.clean, []string(nil)
		profile := filepath.Join(b.o.Work, "cover", fmt.Sprint(r.s.n), fmt.Sprintf("%d.out", i))
		if r.s.cover != "" {
			bin, extra = r.s.cover, []string{"-test.coverprofile=" + profile}
		}
		p, err := b.runTests(ctx, r.s, bin, []string{r.name}, defaultTimeout, extra...)
		if err != nil {
			return b.undecidedRun(err)
		}
		if err := judgeClean(p, r.s, []string{r.name}, "when run alone"); err != nil {
			return err
		}
		r.took = time.Duration(p.elapsed[r.name] * float64(time.Second))
		if r.s.cover == "" {
			return nil
		}
		data, err := os.ReadFile(profile)
		if err != nil {
			return &Undecided{Reason: fmt.Sprintf("%s/%s wrote no coverage profile when run alone: %v", r.s.Primitive, r.name, err)}
		}
		r.blocks, err = readProfile(data, dirs)
		if err != nil {
			return &Undecided{Reason: fmt.Sprintf("%s/%s: %v", r.s.Primitive, r.name, err)}
		}
		return nil
	})
	if err != nil {
		return err
	}
	cov := coverage{}
	for _, r := range runs {
		ref := model.TestRef{Primitive: r.s.Primitive, Name: r.name}
		b.lone[ref] = r.took
		cov.add(ref, r.blocks)
	}
	b.blocks = cov.blocks()
	return nil
}

// judgeClean returns *Undecided when any of tests failed or never finished in a clean run.
func judgeClean(p *proc, s *suite, tests []string, how string) error {
	var failed, unfinished []string
	for _, t := range tests {
		switch p.results[t] {
		case "pass", "skip":
		case "fail":
			failed = append(failed, s.Primitive+"/"+t)
		default:
			unfinished = append(unfinished, s.Primitive+"/"+t)
		}
	}
	if len(failed) > 0 {
		return &Undecided{Reason: fmt.Sprintf("%s %s against the clean code %s", strings.Join(failed, ", "), fails(len(failed)), how)}
	}
	if len(unfinished) == 0 {
		return nil
	}
	why, tail := "never finished", ""
	switch {
	case p.timeout:
		why = "timed out"
	case p.guard:
		why = "hung"
	case !p.anyStarted() && !p.ended:
		why, tail = "never started", "; the test binary said: "+firstLine(p.raw)
	}
	return &Undecided{Reason: fmt.Sprintf("%s %s against the clean code %s%s", strings.Join(unfinished, ", "), why, how, tail)}
}

func fails(n int) string {
	if n == 1 {
		return "fails"
	}
	return "fail"
}

func firstLine(raw []byte) string {
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(strings.ReplaceAll(l, marker, ""))
		if l != "" {
			return l
		}
	}
	return "no output"
}

// undecidedGo turns a go command failure into *Undecided, keeping ctx errors as they are.
func undecidedGo(err error, what string) error {
	var ge *goError
	if errors.As(err, &ge) {
		return &Undecided{Reason: what + ": " + firstError(err)}
	}
	return err
}

// undecidedRun turns a test process that couldn't run at all into *Undecided, keeping ctx errors as
// they are.
func (b *Baseline) undecidedRun(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &Undecided{Reason: "can't run a test binary: " + err.Error()}
}

// Reach returns the Contract tests whose lone run reached a coverage block holding the position.
// file is module-relative and slash-separated.
func (b *Baseline) Reach(file string, line, col int) []model.TestRef {
	set := map[model.TestRef]bool{}
	for _, bl := range b.blocks[file] {
		if bl.contains(line, col) {
			for _, t := range bl.tests {
				set[t] = true
			}
		}
	}
	return sortedRefs(set)
}

// ReachSpan returns the Contract tests whose lone run reached a coverage block that overlaps the span
// from one position to another, both inclusive. An erase mutant replaces a whole function body, so
// every test that ran any of the body reaches it.
func (b *Baseline) ReachSpan(file string, fromLine, fromCol, toLine, toCol int) []model.TestRef {
	want := span{fromLine, fromCol, toLine, toCol}
	set := map[model.TestRef]bool{}
	for _, bl := range b.blocks[file] {
		if bl.overlaps(want) {
			for _, t := range bl.tests {
				set[t] = true
			}
		}
	}
	return sortedRefs(set)
}

// Linking returns the Contract tests whose binary links the package. Coverage puts no counters in
// package-level variable initializers, and init functions run in every process, so linking the
// package is what reaching one of those means.
func (b *Baseline) Linking(importPath string) []model.TestRef {
	return append([]model.TestRef(nil), b.links[importPath]...)
}

// parallel calls fn for 0..n-1 on up to jobs goroutines. When calls fail it returns the error of the
// lowest index, so the same failures always produce the same report. The first failure stops calls
// that haven't started.
func parallel(ctx context.Context, jobs, n int, fn func(ctx context.Context, i int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make([]error, n)
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < jobs && w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				if err := fn(ctx, i); err != nil {
					errs[i] = err
					cancel()
				}
			}
		}()
	}
feed:
	for i := 0; i < n; i++ {
		select {
		case next <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(next)
	wg.Wait()
	var ctxErr error
	for _, err := range errs {
		if err == nil {
			continue
		}
		// A call that failed only because another call's failure stopped it says nothing new.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctxErr == nil {
				ctxErr = err
			}
			continue
		}
		return err
	}
	if ctxErr != nil {
		return ctxErr
	}
	// The caller's ctx may have ended before any call noticed.
	return context.Cause(ctx)
}

func (b *Baseline) progress(format string, args ...any) {
	if b.o.Progress == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	fmt.Fprintf(b.o.Progress, format+"\n", args...)
}
