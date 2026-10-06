package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A mistake on the command line never reaches a verdict. flinch prints no report, exits 2, and says
// on the stderr it was given what was wrong. Nothing reaches the process's own stderr, so that
// message is the only one a person sees.
//
// Contract: cli/L2
func TestBadCommandLinesExitTwo(t *testing.T) {
	mod(t)
	commit(t)
	stray, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stderr
	os.Stderr = stray
	t.Cleanup(func() { os.Stderr = real })

	for _, c := range []struct {
		args    []string
		mention string
	}{
		{[]string{"--nope"}, "nope"},
		{[]string{"contract", "--nope"}, "nope"},
		{[]string{"version", "--nope"}, "nope"},
		{[]string{"operators", "--nope"}, "nope"},
		{[]string{"frob"}, "frob"},
		{[]string{"run", "extra"}, "extra"},
		{[]string{"--format", "xml"}, "xml"},
		{[]string{"contract", "--format", "yaml"}, "yaml"},
		{[]string{"--jobs", "0"}, "--jobs"},
		{[]string{"--jobs", "two"}, "two"},
		{[]string{"--timeout-coefficient", "0"}, "--timeout-coefficient"},
		{[]string{"contract", "--since", "HEAD", "--dry-run"}, "--dry-run, --since"},
		{[]string{"contract", "--operators", "erase"}, "--operators"},
		{[]string{"--only", "nope"}, "nope"},
		{[]string{"--only", "calc,nope", "--dry-run"}, "nope"},
		{[]string{"--operators", "erase,nope"}, "nope"},
		{[]string{"--since", "no-such-ref", "--dry-run"}, "no-such-ref"},
		{[]string{"--output", "missing/report.txt"}, "missing/report.txt"},
	} {
		r := flinch(c.args...)
		line := strings.Join(c.args, " ")
		if r.code != 2 || r.stdout != "" {
			t.Errorf("flinch %s should exit 2 and print no report:\n%s", line, r)
		}
		if !strings.HasPrefix(r.stderr, "flinch: ") || strings.Count(r.stderr, "flinch: ") != 1 || !strings.Contains(r.stderr, c.mention) {
			t.Errorf("flinch %s should say what's wrong with %q:\n%s", line, c.mention, r)
		}
	}
	if info, err := stray.Stat(); err != nil || info.Size() != 0 {
		t.Errorf("flinch wrote to the process's stderr, outside the stderr it was given")
	}
}

// The smallest values a flag takes are good ones.
//
// Contract: cli/L2
func TestGoodFlagValuesAreAccepted(t *testing.T) {
	mod(t)
	for _, args := range [][]string{
		{"--dry-run", "--jobs", "1"},
		{"--dry-run", "--timeout-coefficient", "0.5"},
		{"contract", "--format", "text"},
		{"contract", "--format", "json"},
	} {
		if r := flinch(args...); r.code != 0 {
			t.Errorf("flinch %s should pass:\n%s", strings.Join(args, " "), r)
		}
	}
}
