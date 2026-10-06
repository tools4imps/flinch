package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = b.o.Root
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
	args := []string{"test", "-c", "-vet=off"}
	args = append(args, b.tagArgs()...)
	args = append(args, extra...)
	args = append(args, "-o", out, "./"+dir)
	_, err := b.goCmd(ctx, args...)
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
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// A test can start a process that keeps the output pipe open after the test binary exits. Waiting
	// on it would stall the run, so flinch stops reading shortly after the binary ends.
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
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
