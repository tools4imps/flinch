package gate_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/engine"
	"github.com/tools4imps/flinch/internal/since"
)

// lines makes a file of n numbered lines.
func lines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// editLines rewrites a file, applying f to its lines.
func editLines(t *testing.T, path string, f func([]string) []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	write(t, path, strings.Join(f(ls), "\n")+"\n")
}

func changed(t *testing.T, root, ref string) *since.Changes {
	t.Helper()
	c, err := since.Changed(context.Background(), root, ref)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// touched lists the lines of file, from 1 to max, that count as changed.
func touched(c *since.Changes, file string, max int) []int {
	var out []int
	for i := 1; i <= max; i++ {
		if c.Touches(file, i) {
			out = append(out, i)
		}
	}
	return out
}

func sameLines(t *testing.T, file string, got, want []int) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changed lines of %s = %v, want %v", file, got, want)
	}
}

func span(from, to int) []int {
	var out []int
	for i := from; i <= to; i++ {
		out = append(out, i)
	}
	return out
}

// Contract: gate/G3
func TestSinceCountsTheNewSideLinesThatChanged(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(12))
	write(t, filepath.Join(root, "b.go"), lines(5))
	write(t, filepath.Join(root, "d.go"), lines(3))
	write(t, filepath.Join(root, "e.go"), lines(4))
	write(t, filepath.Join(root, "pkg", "c.go"), lines(4))
	newRepo(t, root)
	head := gitIn(t, root, "rev-parse", "HEAD")

	editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string {
		ls[2] = "line three"                                           // line 3
		ls[6], ls[7] = "line seven", "line eight"                      // lines 7 and 8
		return append(ls[:10], append([]string{"new"}, ls[10:]...)...) // a new line 11
	})
	editLines(t, filepath.Join(root, "b.go"), func(ls []string) []string { return append(ls[:1], ls[2:]...) })
	editLines(t, filepath.Join(root, "e.go"), func(ls []string) []string {
		ls[1] = "line two" // a staged change counts like any other
		return ls
	})
	gitIn(t, root, "add", "e.go")
	if err := os.Remove(filepath.Join(root, "d.go")); err != nil {
		t.Fatal(err)
	}

	c := changed(t, root, "HEAD")
	if c.Base != head {
		t.Errorf("the base is %s, want HEAD, %s", c.Base, head)
	}
	sameLines(t, "a.go", touched(c, "a.go", 14), []int{3, 7, 8, 11})
	sameLines(t, "b.go", touched(c, "b.go", 6), nil) // only a deletion
	sameLines(t, "d.go", touched(c, "d.go", 4), nil)
	sameLines(t, "e.go", touched(c, "e.go", 5), []int{2})
	sameLines(t, "pkg/c.go", touched(c, "pkg/c.go", 5), nil)
	if got, want := c.Paths(), []string{"a.go", "b.go", "d.go", "e.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("changed paths = %q, want %q", got, want)
	}
	if !c.TouchesDir(".") || !c.TouchesDir("") {
		t.Error("the module root doesn't count as changed")
	}
	if c.TouchesDir("pkg") {
		t.Error("pkg counts as changed, and nothing in it changed")
	}
}

// Contract: gate/G3
func TestSinceStartsFromTheMergeBase(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "x.go"), lines(6))
	write(t, filepath.Join(root, "y.go"), lines(6))
	newRepo(t, root)
	base := gitIn(t, root, "rev-parse", "HEAD")

	gitIn(t, root, "checkout", "-q", "-b", "feature")
	editLines(t, filepath.Join(root, "x.go"), func(ls []string) []string { ls[1] = "feature"; return ls })
	commitAll(t, root, "feature")
	gitIn(t, root, "checkout", "-q", "main")
	editLines(t, filepath.Join(root, "y.go"), func(ls []string) []string { ls[2] = "main moved on"; return ls })
	commitAll(t, root, "main")
	gitIn(t, root, "checkout", "-q", "feature")
	editLines(t, filepath.Join(root, "x.go"), func(ls []string) []string { ls[3] = "not committed yet"; return ls })

	c := changed(t, root, "main")
	if c.Base != base {
		t.Errorf("the base is %s, want the merge base %s", c.Base, base)
	}
	sameLines(t, "x.go", touched(c, "x.go", 7), []int{2, 4})
	sameLines(t, "y.go", touched(c, "y.go", 7), nil)
}

// Contract: gate/G3
func TestARenamedFileKeepsOnlyItsEditedLines(t *testing.T) {
	cleanGit(t)
	for _, how := range []string{"committed", "staged", "moved"} {
		t.Run(how, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "old", "name.go"), lines(20))
			write(t, filepath.Join(root, "keep.go"), lines(3))
			newRepo(t, root)
			if how == "committed" {
				gitIn(t, root, "checkout", "-q", "-b", "feature")
			}
			if how == "moved" {
				if err := os.MkdirAll(filepath.Join(root, "new"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(root, "old", "name.go"), filepath.Join(root, "new", "name.go")); err != nil {
					t.Fatal(err)
				}
			} else {
				gitIn(t, root, "mv", "old", "new")
			}
			editLines(t, filepath.Join(root, "new", "name.go"), func(ls []string) []string { ls[4] = "line five"; return ls })
			ref := "HEAD"
			if how == "committed" {
				commitAll(t, root, "rename")
				ref = "main"
			}

			index := filepath.Join(root, ".git", "index")
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			c := changed(t, root, ref)
			if after, err := os.ReadFile(index); err != nil || !bytes.Equal(after, before) {
				t.Errorf("finding the changes rewrote the repository's index (%v)", err)
			}
			sameLines(t, "new/name.go", touched(c, "new/name.go", 21), []int{5})
			if got, want := c.Paths(), []string{"new/name.go", "old/name.go"}; !reflect.DeepEqual(got, want) {
				t.Errorf("changed paths = %q, want both sides of the rename, %q", got, want)
			}
			if !c.TouchesDir("old") || !c.TouchesDir("new") {
				t.Errorf("old touched = %v, new touched = %v; want both", c.TouchesDir("old"), c.TouchesDir("new"))
			}
		})
	}
}

// Contract: gate/G3
func TestAnUntrackedFileCountsAsNew(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, ".gitignore"), "skip.go\n")
	write(t, filepath.Join(root, "contract", "calc", "README.md"), "# calc\n")
	write(t, filepath.Join(root, "contract", "calculus", "README.md"), "# calculus\n")
	newRepo(t, root)

	write(t, filepath.Join(root, "contract", "calc", "new_test.go"), lines(3))
	write(t, filepath.Join(root, "skip.go"), lines(3))
	write(t, filepath.Join(root, "empty.go"), "")
	// A repository inside the module, which git lists as one untracked directory.
	write(t, filepath.Join(root, "nested", "n.go"), lines(2))
	newRepo(t, filepath.Join(root, "nested"))

	c := changed(t, root, "HEAD")
	sameLines(t, "contract/calc/new_test.go", touched(c, "contract/calc/new_test.go", 3), span(1, 3))
	sameLines(t, "skip.go", touched(c, "skip.go", 3), nil)
	if got, want := c.Paths(), []string{"contract/calc/new_test.go", "empty.go", "nested/"}; !reflect.DeepEqual(got, want) {
		t.Errorf("changed paths = %q, want %q", got, want)
	}
	for dir, want := range map[string]bool{
		"contract/calc": true, "contract/calc/": true, "contract": true,
		"contract/cal": false, "contract/calculus": false, "skip.go": false, "nested": true,
	} {
		if got := c.TouchesDir(dir); got != want {
			t.Errorf("TouchesDir(%q) = %v, want %v", dir, got, want)
		}
	}
}

// Contract: gate/G3
func TestSinceKeepsToTheModuleBelowTheRepositoryTop(t *testing.T) {
	cleanGit(t)
	top := t.TempDir()
	root := filepath.Join(top, "mod")
	write(t, filepath.Join(top, "top.go"), lines(3))
	write(t, filepath.Join(top, "model", "m.go"), lines(3))
	write(t, filepath.Join(root, "a.go"), lines(3))
	newRepo(t, top)

	editLines(t, filepath.Join(top, "top.go"), func(ls []string) []string { ls[0] = "x"; return ls })
	editLines(t, filepath.Join(top, "model", "m.go"), func(ls []string) []string { ls[0] = "x"; return ls })
	editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string { ls[1] = "x"; return ls })
	write(t, filepath.Join(root, "u.go"), lines(2))
	write(t, filepath.Join(top, "u.go"), lines(2))

	c := changed(t, root, "HEAD")
	if got, want := c.Paths(), []string{"a.go", "u.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("changed paths = %q, want only the module's own, %q", got, want)
	}
	sameLines(t, "a.go", touched(c, "a.go", 4), []int{2})
	sameLines(t, "u.go", touched(c, "u.go", 2), []int{1, 2})
	sameLines(t, "top.go", touched(c, "top.go", 4), nil)
	sameLines(t, "el/m.go", touched(c, "el/m.go", 4), nil)
}

// Contract: gate/G3
func TestOddPathsAndAddedLinesThatLookLikeHeaders(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	names := []string{`we"ird.go`, "with space.go", "café.go", "tab\there.go", `back\slash.go`, "plain.go"}
	for _, n := range names {
		write(t, filepath.Join(root, n), lines(5))
	}
	newRepo(t, root)
	for _, n := range names {
		editLines(t, filepath.Join(root, n), func(ls []string) []string { ls[1] = "changed"; return ls })
	}
	// An added line holding "++ x" shows in the patch as "+++ x".
	editLines(t, filepath.Join(root, "plain.go"), func(ls []string) []string {
		return append(ls[:3], append([]string{"++ x", "++ b/elsewhere.go"}, ls[3:]...)...)
	})

	c := changed(t, root, "HEAD")
	for _, n := range names[:5] {
		sameLines(t, n, touched(c, n, 6), []int{2})
	}
	sameLines(t, "plain.go", touched(c, "plain.go", 8), []int{2, 4, 5})
	if got := c.Paths(); !reflect.DeepEqual(got, sortedCopy(names)) {
		t.Errorf("changed paths = %q, want %q", got, sortedCopy(names))
	}
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Contract: gate/G3
func TestNothingChangedMeansNothingTouched(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(3))
	newRepo(t, root)
	c := changed(t, root, "HEAD")
	if c.TouchesDir(".") || c.TouchesDir("") || len(c.Paths()) > 0 || c.Touches("a.go", 1) {
		t.Errorf("a clean tree counts as changed: paths %q", c.Paths())
	}
}

// Contract: gate/G3
func TestSinceNeedsARefWithAMergeBase(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(3))
	newRepo(t, root)
	gitIn(t, root, "checkout", "-q", "--orphan", "alone")
	write(t, filepath.Join(root, "b.go"), lines(2))
	commitAll(t, root, "unrelated")

	for _, ref := range []string{"", "no-such-ref", "main"} {
		if c, err := since.Changed(context.Background(), root, ref); err == nil {
			t.Errorf("--since %q found changes %q, want an error", ref, c.Paths())
		}
	}
}

// Contract: gate/G1
// Contract: gate/G3
func TestGitConfigAndTheEnvironmentNeverMoveChangedLines(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(30))
	write(t, filepath.Join(root, "old", "r.go"), lines(20))
	write(t, filepath.Join(root, "sub", "s.go"), lines(10))
	newRepo(t, root)
	editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string {
		ls[4], ls[5], ls[20] = "five", "six", "twenty-one"
		return ls
	})
	gitIn(t, root, "mv", "old", "new")
	editLines(t, filepath.Join(root, "new", "r.go"), func(ls []string) []string { ls[9] = "ten"; return ls })
	editLines(t, filepath.Join(root, "sub", "s.go"), func(ls []string) []string { ls[0] = "one"; return ls })
	write(t, filepath.Join(root, "untracked.go"), lines(4))
	write(t, filepath.Join(root, "café.go"), lines(2))

	files := []string{"a.go", "new/r.go", "old/r.go", "sub/s.go", "untracked.go", "café.go"}
	view := func(root string) string {
		c := changed(t, root, "HEAD")
		var b strings.Builder
		fmt.Fprintf(&b, "paths %q\n", c.Paths())
		for _, f := range files {
			fmt.Fprintf(&b, "%s %v\n", f, touched(c, f, 31))
		}
		for _, d := range []string{".", "old", "new", "sub"} {
			fmt.Fprintf(&b, "dir %s %v\n", d, c.TouchesDir(d))
		}
		return b.String()
	}

	clean := view(root)
	if !strings.Contains(clean, "a.go [5 6 21]") || !strings.Contains(clean, "new/r.go [10]") {
		t.Fatalf("the clean view is wrong:\n%s", clean)
	}
	hostileGit(t, root)
	hostile := view(root)
	if hostile != clean {
		t.Errorf("git config and the environment moved the changed lines.\nclean:\n%s\nhostile:\n%s", clean, hostile)
	}
	sub := view(filepath.Join(root, "sub"))
	if !strings.Contains(sub, "paths [\"s.go\"]") {
		t.Errorf("from sub, with diff.relative set, the changed paths are wrong:\n%s", sub)
	}
}

// fakeGit puts a git on PATH that passes every command to the real one, except a command holding
// the argument on: then it fails loudly ("fail"), fails without a word ("quiet"), or prints the
// file out and succeeds ("print").
func fakeGit(t *testing.T, on, do, out string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = \"$FAKE_GIT_ON\" ]; then\n" +
		"    case \"$FAKE_GIT_DO\" in\n" +
		"      fail) echo \"fake git refuses $a\" >&2; exit 1 ;;\n" +
		"      quiet) exit 1 ;;\n" +
		"      print) cat \"$FAKE_GIT_OUT\"; exit 0 ;;\n" +
		"    esac\n  fi\ndone\nexec \"$FAKE_GIT_REAL\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(bin, "out")
	write(t, outFile, out)
	t.Setenv("FAKE_GIT_REAL", real)
	t.Setenv("FAKE_GIT_ON", on)
	t.Setenv("FAKE_GIT_DO", do)
	t.Setenv("FAKE_GIT_OUT", outFile)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Contract: gate/G2
// Contract: gate/G3
func TestAGitFailureStopsTheRunInsteadOfShrinkingIt(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(3))
	newRepo(t, root)
	editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string { ls[0] = "x"; return ls })
	write(t, filepath.Join(root, "u.go"), lines(1))

	header := "diff --git a/a.go b/a.go\n--- a/a.go\n"
	for _, c := range []struct{ name, on, do, out string }{
		{"merge-base prints nothing", "merge-base", "print", ""},
		{"rev-parse fails", "rev-parse", "fail", ""},
		{"listing changed files fails", "--name-status", "fail", ""},
		{"listing changed files fails quietly", "--name-status", "quiet", ""},
		{"a rename entry is cut short", "--name-status", "print", "R100\x00old.go"},
		{"the patch fails", "--patch", "fail", ""},
		{"a patch header lacks b/", "--patch", "print", header + "+++ a.go\n@@ -1 +1 @@\n"},
		{"a patch header is badly quoted", "--patch", "print", header + "+++ \"b/a\\q.go\"\n@@ -1 +1 @@\n"},
		{"a hunk header isn't closed", "--patch", "print", header + "+++ b/a.go\n@@ -1 +1\n"},
		{"a hunk start isn't a number", "--patch", "print", header + "+++ b/a.go\n@@ -1 +x,1 @@\n"},
		{"a hunk count isn't a number", "--patch", "print", header + "+++ b/a.go\n@@ -1 +1,y @@\n"},
		{"a hunk has no new side", "--patch", "print", header + "+++ b/a.go\n@@ -1 @@\n"},
		{"listing untracked files fails", "ls-files", "fail", ""},
		{"finding the index fails", "--git-path", "fail", ""},
		{"the index can't be read", "--git-path", "print", "/\n"},
		{"noting untracked files fails", "--intent-to-add", "fail", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			fakeGit(t, c.on, c.do, c.out)
			if ch, err := since.Changed(context.Background(), root, "HEAD"); err == nil {
				t.Errorf("Changed succeeded with paths %q, want an error", ch.Paths())
			}
		})
	}
	t.Run("git sees the user's environment", func(t *testing.T) {
		// The fake reads its settings from the environment, and passes everything through.
		fakeGit(t, "no-such-argument", "fail", "")
		ch, err := since.Changed(context.Background(), root, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if got, want := ch.Paths(), []string{"a.go", "u.go"}; !reflect.DeepEqual(got, want) {
			t.Errorf("changed paths = %q, want %q", got, want)
		}
	})
	t.Run("no temp directory", func(t *testing.T) {
		t.Setenv("TMPDIR", filepath.Join(root, "missing"))
		if ch, err := since.Changed(context.Background(), root, "HEAD"); err == nil {
			t.Errorf("Changed succeeded with paths %q, want an error", ch.Paths())
		}
	})
}

// Contract: gate/G3
func TestFilesASparseCheckoutLeavesOutDidntChange(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, "a.go"), lines(3))
	write(t, filepath.Join(root, "far", "b.go"), lines(3))
	newRepo(t, root)
	gitIn(t, root, "update-index", "--skip-worktree", "far/b.go")
	if err := os.Remove(filepath.Join(root, "far", "b.go")); err != nil {
		t.Fatal(err)
	}
	editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string { ls[0] = "x"; return ls })
	write(t, filepath.Join(root, "new.go"), lines(2))

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	c := changed(t, root, "HEAD")
	if got, want := c.Paths(), []string{"a.go", "new.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("changed paths = %q, want %q", got, want)
	}
	if left, err := os.ReadDir(tmp); err != nil || len(left) > 0 {
		t.Errorf("Changed left %v behind in the temp directory (%v)", left, err)
	}
}

// Contract: gate/G3
func TestARunSinceARefMutatesChangedLinesAndChangedPrimitives(t *testing.T) {
	cleanGit(t)
	dir := copyFixture(t, "testdata/mod")
	newRepo(t, dir)
	plan := func() []string {
		t.Helper()
		p, err := engine.Prepare(context.Background(), engine.RunOptions{
			Options: engine.Options{Dir: dir, Contract: "contract"}, Since: "HEAD",
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Static.Problems) > 0 {
			t.Fatalf("Contract errors: %v", p.Static.Problems)
		}
		var ids []string
		for _, m := range p.Mutants {
			ids = append(ids, m.ID)
		}
		return ids
	}

	if ids := plan(); len(ids) > 0 {
		t.Errorf("with nothing changed, --since HEAD mutates %q", ids)
	}
	replace(t, filepath.Join(dir, "calc", "calc.go"), "\tif a > b {\n", "\tif a > b { // the larger one\n")
	want := []string{"calc.Max: { ... } -> { return *new(int) }", "calc.Max: a > b -> a >= b"}
	if ids := plan(); !reflect.DeepEqual(ids, want) {
		t.Errorf("with one line of calc.Max changed, --since HEAD mutates %q, want %q", ids, want)
	}
	commitAll(t, dir, "comment")
	replace(t, filepath.Join(dir, "contract", "calc", "README.md"), "calc picks numbers.", "calc picks numbers for people.")
	want = []string{
		"calc.Max: { ... } -> { return *new(int) }", "calc.Max: a > b -> a >= b",
		"calc.Twice: { ... } -> { return *new(int) }", "calc.Twice: n * 2 -> n / 2",
	}
	if ids := plan(); !reflect.DeepEqual(ids, want) {
		t.Errorf("with calc's Contract changed, --since HEAD mutates %q, want all of calc's code, %q", ids, want)
	}
}

// Contract: gate/G3
func TestAttributesFilesNeverHideChangedLines(t *testing.T) {
	cleanGit(t)
	for _, where := range []string{"repository", "info", "global"} {
		t.Run(where, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "a.go"), lines(5))
			if where == "repository" {
				write(t, filepath.Join(root, ".gitattributes"), "*.go -diff\n")
			}
			newRepo(t, root)
			switch where {
			case "info":
				write(t, filepath.Join(root, ".git", "info", "attributes"), "*.go -diff\n")
			case "global":
				attributes := filepath.Join(t.TempDir(), "attributes")
				write(t, attributes, "*.go binary\n")
				cfg := filepath.Join(t.TempDir(), "gitconfig")
				write(t, cfg, "[core]\n\tattributesFile = "+attributes+"\n")
				t.Setenv("GIT_CONFIG_GLOBAL", cfg)
			}
			editLines(t, filepath.Join(root, "a.go"), func(ls []string) []string { ls[2] = "three"; return ls })
			write(t, filepath.Join(root, "b.go"), lines(2))

			c := changed(t, root, "HEAD")
			sameLines(t, "a.go", touched(c, "a.go", 6), []int{3})
			sameLines(t, "b.go", touched(c, "b.go", 2), []int{1, 2})
		})
	}
}

// Contract: gate/G3
func TestTheWorkingTreeIsWhatCounts(t *testing.T) {
	cleanGit(t)
	root := t.TempDir()
	write(t, filepath.Join(root, ".gitignore"), "skip.go\n")
	for _, f := range []string{"staged.go", "both.go", "unstaged.go"} {
		write(t, filepath.Join(root, f), lines(6))
	}
	newRepo(t, root)

	editLines(t, filepath.Join(root, "staged.go"), func(ls []string) []string { ls[1] = "two"; return ls })
	gitIn(t, root, "add", "staged.go")
	// Staged on line 2, then put back in the working tree, where line 5 changes instead.
	editLines(t, filepath.Join(root, "both.go"), func(ls []string) []string { ls[1] = "two"; return ls })
	gitIn(t, root, "add", "both.go")
	editLines(t, filepath.Join(root, "both.go"), func(ls []string) []string { ls[1], ls[4] = "line 2", "five"; return ls })
	editLines(t, filepath.Join(root, "unstaged.go"), func(ls []string) []string { ls[3] = "four"; return ls })
	write(t, filepath.Join(root, "untracked.go"), lines(3))
	write(t, filepath.Join(root, "skip.go"), lines(3))

	c := changed(t, root, "HEAD")
	sameLines(t, "staged.go", touched(c, "staged.go", 6), []int{2})
	sameLines(t, "both.go", touched(c, "both.go", 6), []int{5})
	sameLines(t, "unstaged.go", touched(c, "unstaged.go", 6), []int{4})
	sameLines(t, "untracked.go", touched(c, "untracked.go", 3), []int{1, 2, 3})
	sameLines(t, "skip.go", touched(c, "skip.go", 3), nil)
	if got, want := c.Paths(), []string{"both.go", "staged.go", "unstaged.go", "untracked.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("changed paths = %q, want %q", got, want)
	}
}
