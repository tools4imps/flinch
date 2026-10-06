package run_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/runner"
)

// The run Contract tests build and run a fixture module, which is slow, so the tests share a few
// scenarios. Each scenario runs once per process, the first time a test asks for it, and the tests
// read what it recorded. A scenario that takes longer than patience fails every test that waits on
// it, and so does every scenario after it, so a runner that hangs costs one wait rather than one per
// test.
const patience = 150 * time.Second

// tmp holds every fixture copy and work directory this process makes. TestMain removes it.
var tmp string

// testdata is this package's testdata directory, absolute.
var testdata string

// hung is set once a scenario outlives its patience.
var hung atomic.Bool

func TestMain(m *testing.M) {
	sweep()
	dir, err := os.MkdirTemp("", fmt.Sprintf("flinch-run-contract-%d-", os.Getpid()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	tmp = dir
	// Everything this process and its children make in a temp directory lands in tmp, so sweep
	// finds it even when the process dies before it can clean up.
	os.Setenv("TMPDIR", tmp)
	// Every fixture copy lives at a new path. Building with -trimpath keeps the path out of the
	// build cache's keys, so each process reuses what earlier ones compiled.
	os.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -trimpath"))
	if testdata, err = filepath.Abs("testdata"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// A broken runner can write its work files by relative paths. Working in tmp keeps them out of
	// the source tree.
	if err := os.Chdir(tmp); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	go watch()
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// sweep removes what processes that died before their TestMain could clean up left behind. A
// mutant that crashes or hangs the runner kills the process, so a mutation run leaves many.
func sweep() {
	old, _ := filepath.Glob(filepath.Join(os.TempDir(), "flinch-run-contract-*-*"))
	for _, d := range old {
		pid, err := strconv.Atoi(strings.Split(filepath.Base(d), "-")[3])
		if err == nil && syscall.Kill(pid, 0) == syscall.ESRCH {
			os.RemoveAll(d)
		}
	}
}

// watch stops the process when goroutines run away, as they do when a broken runner starts workers
// without end. Waiting for the time budget would let them take the machine's memory first.
func watch() {
	for {
		time.Sleep(10 * time.Millisecond)
		if n := runtime.NumGoroutine(); n > 20000 {
			panic(fmt.Sprintf("%d goroutines: the runner is starting them without end", n))
		}
	}
}

// A scenario is work several tests read. It runs at most once per process.
type scenario[T any] struct {
	run  func(ctx context.Context) (T, error)
	once sync.Once
	done chan struct{}
	val  T
	err  error
}

func newScenario[T any](run func(ctx context.Context) (T, error)) *scenario[T] {
	return &scenario[T]{run: run, done: make(chan struct{})}
}

// get runs the scenario, or waits for the run another test started, and fails the test when the
// scenario failed or outlived its patience.
func (s *scenario[T]) get(t *testing.T) T {
	t.Helper()
	if hung.Load() {
		t.Fatal("an earlier scenario never finished")
	}
	s.once.Do(func() {
		go func() {
			defer close(s.done)
			ctx, cancel := context.WithTimeout(context.Background(), patience-10*time.Second)
			defer cancel()
			s.val, s.err = s.run(ctx)
		}()
	})
	wait(t, s.done)
	if s.err != nil {
		t.Fatal(s.err)
	}
	return s.val
}

// wait waits for done, and fails the test after patience or when nine tenths of the time left before
// the test binary's own deadline have passed, whichever comes first. Failing before the deadline
// keeps the binary alive, so the tests after this one fail at once instead of each waiting out a
// budget of its own.
func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	limit := patience
	if d, ok := t.Deadline(); ok && time.Until(d)*9/10 < limit {
		limit = time.Until(d) * 9 / 10
	}
	select {
	case <-done:
	case <-time.After(limit):
		hung.Store(true)
		t.Fatalf("gave up after %v", limit)
	}
}

// copyFixture copies testdata/<name> to a new directory under tmp and returns the copy's root.
func copyFixture(name string) (string, error) {
	base, err := os.MkdirTemp(tmp, name+"-")
	if err != nil {
		return "", err
	}
	src := filepath.Join(testdata, name)
	root := filepath.Join(base, name)
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	return root, err
}

// snapshot reads every file under root, the git directory included, to show that a run leaves the
// module as it found it.
func snapshot(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		files[p] = string(data)
		return err
	})
	return files, err
}

// gitInit makes root a git repository with every file staged, so a snapshot covers the index too.
func gitInit(ctx context.Context, root string) error {
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return nil
}

// mutant replaces the first occurrence of orig in a fixture file with repl. The tag tells apart two
// mutants that make the same change.
func mutant(root, file, orig, repl, tag string) (model.Mutant, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return model.Mutant{}, err
	}
	i := strings.Index(string(src), orig)
	if i < 0 {
		return model.Mutant{}, fmt.Errorf("%s has no %q", file, orig)
	}
	return splice(file, src, i, i+len(orig), repl, tag), nil
}

// splice is the mutant that replaces src[start:end] with repl.
func splice(file string, src []byte, start, end int, repl, tag string) model.Mutant {
	dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))
	line, col := position(src, start)
	id := dir + ".X: " + string(src[start:end]) + " -> " + repl + tag
	return model.Mutant{
		ID: id, Hash: mutantid.Hash(id), Dir: dir, ImportPath: fx + "/" + dir,
		File: file, Line: line, Col: col, Start: start, End: end, Text: repl,
	}
}

const fx = "example.com/fx"

// position is the 1-based line and column of a byte offset.
func position(src []byte, offset int) (int, int) {
	before := string(src[:offset])
	return 1 + strings.Count(before, "\n"), offset - strings.LastIndex(before, "\n")
}

// at is the position of the first occurrence of text in a fixture file.
func at(t *testing.T, root, file, text string) (line, col int) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(src), text)
	if i < 0 {
		t.Fatalf("%s has no %q", file, text)
	}
	return position(src, i)
}

// refs turns "a/TestAdd" names into test refs.
func refs(names ...string) []model.TestRef {
	out := []model.TestRef{}
	for _, n := range names {
		p, name, _ := strings.Cut(n, "/")
		out = append(out, model.TestRef{Primitive: p, Name: name})
	}
	return out
}

// sameRefs compares two lists of tests as sets. The Contract promises which tests a list holds and
// leaves their order open.
func sameRefs(got, want []model.TestRef) bool {
	return reflect.DeepEqual(names(got), names(want))
}

// subset reports whether every test in a is in b.
func subset(a, b []model.TestRef) bool {
	in := map[string]bool{}
	for _, n := range names(b) {
		in[n] = true
	}
	for _, n := range names(a) {
		if !in[n] {
			return false
		}
	}
	return true
}

func names(rs []model.TestRef) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.String())
	}
	sort.Strings(out)
	return out
}

// kills renders a row's kills as sorted "test kind [subtests]" lines.
func kills(r model.Row) []string {
	out := []string{}
	for _, k := range r.Kills {
		subs := append([]string(nil), k.Subtests...)
		sort.Strings(subs)
		s := k.Test.String() + " " + string(k.Kind)
		if len(subs) > 0 {
			s += " " + strings.Join(subs, ",")
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// An entry is one line of fx.log: what a fixture test, or a mutant, wrote as it ran.
type entry struct {
	event   string
	run     []string // the tests the process was asked to run, sorted; nil for a whole run
	timeout string
	tag     string
	pid     int
}

// lone reports whether the entry came from a lone run: one test, with the default ten minutes.
func (e entry) lone() bool { return len(e.run) == 1 && e.timeout == "10m0s" }

// budgeted reports whether the entry came from a run with a time budget: a batch's clean run, a
// mutant's run or a rerun.
func (e entry) budgeted() bool { return e.timeout != "10m0s" }

func readLog(root string) ([]entry, error) {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(root), "fx.log"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		f := strings.Fields(line)
		if len(f) != 5 {
			return nil, fmt.Errorf("unreadable log line %q", line)
		}
		e := entry{event: f[0]}
		for _, kv := range f[1:] {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "run":
				if v != "" {
					e.run = strings.Split(strings.TrimSuffix(strings.TrimPrefix(v, "^("), ")$"), "|")
					sort.Strings(e.run)
				}
			case "timeout":
				e.timeout = v
			case "tag":
				e.tag = v
			case "pid":
				e.pid, _ = strconv.Atoi(v)
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// suite lists a fixture primitive's tests the way the engine does.
func suite(prim string, tests ...string) runner.Suite {
	return runner.Suite{Primitive: prim, Dir: "contract/" + prim, ImportPath: fx + "/contract/" + prim, Tests: tests}
}

// undecided checks that err is *runner.Undecided and returns its message.
func undecided(t *testing.T, err error) string {
	t.Helper()
	var u *runner.Undecided
	if !errors.As(err, &u) {
		t.Fatalf("err = %v, want *runner.Undecided", err)
	}
	t.Logf("reason: %s", u.Error())
	return u.Error()
}
