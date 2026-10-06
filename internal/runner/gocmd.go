package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The guard gives a test process this long past its -test.timeout before flinch stops it. Go's own
// timeout fires first in every ordinary case; the guard is for a binary that hangs where Go's timer
// can't reach, such as in an init function, which runs before the timer starts.
const guardSlack = 10 * time.Second

// The time budget starts from this much before any lone run times are added, so a batch of very
// fast tests still has room for process startup on a busy machine.
const budgetBase = 2 * time.Second

// Tests that run without a budget of their own, such as the clean whole-suite run, get the limit
// go test itself would give them.
const defaultTimeout = 10 * time.Minute

// goCmd runs the go command in the module root. It returns standard output, and on failure an error
// that carries standard error.
func (b *Baseline) goCmd(ctx context.Context, args ...string) ([]byte, error) {
	return b.goCmdEnv(ctx, nil, args...)
}

// goCmdEnv is goCmd with extra environment variables.
func (b *Baseline) goCmdEnv(ctx context.Context, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = b.o.Root
	if env != nil {
		cmd.Env = withEnv(cmd.Dir, env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, &goError{msg: msg}
	}
	return stdout.Bytes(), nil
}

// A goError is the go command failing, with what it printed.
type goError struct{ msg string }

func (e *goError) Error() string { return e.msg }

func (b *Baseline) tagArgs() []string {
	if len(b.o.Tags) == 0 {
		return nil
	}
	return []string{"-tags=" + strings.Join(b.o.Tags, ",")}
}

// build compiles the test binary for the package in dir. Vet is off because go test -c skips it
// anyway, and saying so keeps a vet complaint about a mutant from ever passing for a build failure.
func (b *Baseline) build(ctx context.Context, dir, out string, extra ...string) error {
	return b.buildEnv(ctx, nil, dir, out, extra...)
}

// mutantCacheVar names the build cache a run hands to the flinch runs its tests start.
const mutantCacheVar = "FLINCH_GOCACHE"

// mutantCache is the build cache for mutant builds. Every mutant compiles its package and the
// packages that import it afresh, and in the user's cache those builds would pile up by the
// gigabyte, so a run builds mutants in a cache of its own that goes when the run ends. A run started
// by another run's tests shares the outer run's cache, so the standard library compiles once for the
// whole tree of runs.
func mutantCache(work string) (dir string, owned bool) {
	if dir := os.Getenv(mutantCacheVar); dir != "" {
		return dir, false
	}
	return filepath.Join(work, "gocache"), true
}

// defaultCacheLimit is how big the run's own build cache may grow before flinch empties it between
// chunks of mutants. Every mutant adds its own compiled packages, so over a long run the cache would
// otherwise grow until the disk is full.
const defaultCacheLimit = 2 << 30

// trimCache empties the run's own build cache when it has grown past the limit. Only the run that
// made the cache trims it: a run started by another run's tests shares the outer cache and leaves it
// alone, since the outer run may be building at the time.
func (b *Baseline) trimCache() {
	if !b.ownsCache {
		return
	}
	limit := b.o.CacheLimit
	if limit <= 0 {
		limit = defaultCacheLimit
	}
	var size int64
	filepath.WalkDir(b.cache, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	if size > limit {
		os.RemoveAll(b.cache)
	}
}

// buildMutant compiles a test binary through an overlay, in the mutant build cache. A build that
// fails gets one more try after stray .go files are cleared from the suites' directories, because a
// test another mutant made misbehave may have left one there.
func (b *Baseline) buildMutant(ctx context.Context, dir, out, overlay string) error {
	env := []string{"GOCACHE=" + b.cache}
	err := b.buildEnv(ctx, env, dir, out, "-overlay="+overlay)
	if err != nil && ctx.Err() == nil {
		b.sweepAllGo()
		err = b.buildEnv(ctx, env, dir, out, "-overlay="+overlay)
	}
	return err
}

// withEnv is the environment for a process started in dir with the given variables added. It sets
// PWD to dir, as os/exec does only when a command keeps the parent's environment. A stale PWD makes
// the go command spell its directory through symlinks resolved, and an overlay keyed by the path as
// written then matches nothing and is quietly ignored.
func withEnv(dir string, vars ...string) []string {
	return append(os.Environ(), append([]string{"PWD=" + dir}, vars...)...)
}

func (b *Baseline) buildEnv(ctx context.Context, env []string, dir, out string, extra ...string) error {
	args := []string{"test", "-c", "-vet=off"}
	args = append(args, b.tagArgs()...)
	args = append(args, extra...)
	args = append(args, "-o", out, "./"+dir)
	_, err := b.goCmdEnv(ctx, env, args...)
	if err == nil && !exists(out) {
		// go test -c writes nothing for a package without test files and still succeeds.
		return &goError{msg: "no test files in " + dir}
	}
	return err
}

// firstError is the first line of a build failure that says what's wrong, skipping the "# package"
// headers the go command prints above compiler errors.
func firstError(err error) string {
	for _, l := range strings.Split(err.Error(), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "# ") {
			return l
		}
	}
	return strings.TrimSpace(err.Error())
}

// listed is one package go list reports.
type listed struct {
	ImportPath string
	Dir        string
	Standard   bool
}

// deps lists the packages the test binary for dir links, the test variants folded into their
// packages, and the standard library left out.
func (b *Baseline) deps(ctx context.Context, dir string) ([]listed, error) {
	args := []string{"list", "-deps", "-test", "-json=ImportPath,Dir,Standard"}
	args = append(args, b.tagArgs()...)
	args = append(args, "./"+dir)
	out, err := b.goCmd(ctx, args...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ps []listed
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listed
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading go list output: %v", err)
		}
		if p.Standard {
			continue
		}
		// A test build lists "pkg [pkg.test]" next to "pkg"; both are the same source.
		if i := strings.Index(p.ImportPath, " ["); i >= 0 {
			p.ImportPath = p.ImportPath[:i]
		}
		if seen[p.ImportPath] {
			continue
		}
		seen[p.ImportPath] = true
		ps = append(ps, p)
	}
	return ps, nil
}

// A proc is one finished test process.
type proc struct {
	*stream
	guard bool // the wall-clock guard stopped it
	raw   []byte
}

// runTests runs a test binary in its package's directory, the way go test runs it, asking for the
// given top-level tests (all of them when tests is nil). An error means the process couldn't run at
// all, or ctx ended.
func (b *Baseline) runTests(ctx context.Context, s *suite, bin string, tests []string, timeout time.Duration, extra ...string) (*proc, error) {
	// -test.paniconexit0 is what go test passes too: a test that calls os.Exit(0) would otherwise look
	// like a clean finish.
	args := []string{"-test.v=test2json", "-test.paniconexit0", "-test.timeout=" + timeout.String()}
	if tests != nil {
		args = append(args, "-test.run="+runPattern(tests))
	}
	args = append(args, extra...)
	gctx, cancel := context.WithTimeout(ctx, timeout+guardSlack)
	defer cancel()
	cmd := exec.CommandContext(gctx, bin, args...)
	cmd.Dir = s.abs
	// Whatever a test leaves in the temp directory, and whatever the processes it starts leave there,
	// lands inside the run's own work directory and goes when the run ends, even when flinch had to
	// kill the test before it could clean up.
	tmp := filepath.Join(b.o.Work, "tmp")
	cmd.Env = withEnv(cmd.Dir, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp, mutantCacheVar+"="+b.cache)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// A test can start a process that keeps the output pipe open after the test binary exits. Waiting
	// on it would stall the run, so flinch stops reading shortly after the binary ends.
	cmd.WaitDelay = 2 * time.Second
	s.enter()
	err := cmd.Start()
	if err == nil {
		stop := watchMemory(cmd.Process, b.o.MemoryLimit)
		err = cmd.Wait()
		stop()
	}
	s.leave()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	guard := gctx.Err() != nil
	var exit *exec.ExitError
	if err != nil && !guard && !errors.As(err, &exit) && !errors.Is(err, exec.ErrWaitDelay) {
		return nil, err
	}
	return &proc{stream: parseStream(out.Bytes()), guard: guard, raw: out.Bytes()}, nil
}

// runPattern matches exactly the named top-level tests.
func runPattern(tests []string) string {
	q := make([]string, len(tests))
	for i, t := range tests {
		q[i] = regexp.QuoteMeta(t)
	}
	return "^(" + strings.Join(q, "|") + ")$"
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
