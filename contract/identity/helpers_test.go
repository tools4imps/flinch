package identity_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
)

// here is the test package's directory, where the fixtures are, before any test changes into one.
var here, _ = os.Getwd()

// fixture is the absolute root of a fixture module under testdata.
func fixture(name string) string { return filepath.Join(here, "testdata", name) }

// run runs flinch in a module's directory and returns the exit code and stdout.
func run(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer
	code := cli.Main(args, strings.NewReader(""), &stdout, &stderr)
	if code == 2 {
		t.Fatalf("flinch %s exited 2: %s", strings.Join(args, " "), stderr.String())
	}
	return code, stdout.String()
}

// A listed mutant is one line of flinch --dry-run.
type listed struct {
	File string
	Line int
	Hash string
	ID   string
}

// dryRun lists the mutants flinch would build in a module.
func dryRun(t *testing.T, dir string) []listed {
	t.Helper()
	code, out := run(t, dir, "--dry-run")
	if code != 0 {
		t.Fatalf("flinch --dry-run exited %d:\n%s", code, out)
	}
	var ms []listed
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "  ", 3)
		if len(parts) != 3 {
			continue
		}
		file, no, ok := strings.Cut(parts[0], ":")
		n, err := strconv.Atoi(no)
		if !ok || err != nil {
			t.Fatalf("a dry-run line that doesn't start with file:line: %q", line)
		}
		ms = append(ms, listed{File: file, Line: n, Hash: parts[1], ID: parts[2]})
	}
	if len(ms) == 0 {
		t.Fatalf("flinch --dry-run listed no mutants:\n%s", out)
	}
	return ms
}

// ids lists the ids of mutants.
func ids(ms []listed) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return out
}

// unit is the unit an id names, the part between the directory and the colon.
func unit(id, dir string) string {
	rest := strings.TrimPrefix(id, dir+".")
	name, _, _ := strings.Cut(rest, ": ")
	return name
}

// sha12 is the first 12 hex digits of the SHA-256 of s, worked out here rather than by flinch.
func sha12(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}

// copyTree copies a fixture module into a fresh temporary directory, so a test can edit it.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// edit replaces the one occurrence of old in a file with something else.
func edit(t *testing.T, file, old, with string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), old) != 1 {
		t.Fatalf("%s holds %q %d times, want once", file, old, strings.Count(string(data), old))
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), old, with, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}
