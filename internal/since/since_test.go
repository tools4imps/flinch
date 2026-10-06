package since

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The tests point git at a config file of their own, so the machine's config can't make a commit
// fail or change a diff. The hostile one sets everything a user might set that would move a diff.
const plainConfig = `[user]
	name = flinch
	email = flinch@example.com
[commit]
	gpgsign = false
[init]
	defaultBranch = main
`

const hostileConfig = plainConfig + `[color]
	ui = always
	diff = always
[diff]
	external = false
	noprefix = true
	mnemonicPrefix = true
	algorithm = patience
	interHunkContext = 20
	context = 7
	renames = false
	relative = true
	indentHeuristic = false
	renameLimit = 1
[core]
	quotePath = true
	excludesFile = EXCLUDES
`

// setup gives the test a git environment with the named config and returns a fresh repository.
func setup(t *testing.T, config string, hostileEnv bool) string {
	t.Helper()
	home := t.TempDir()
	// A global excludes file that hides the scenario's untracked file shows whether flinch reads it.
	excludes := filepath.Join(home, "ignore")
	if err := os.WriteFile(excludes, []byte("fresh.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, "gitconfig")
	config = strings.ReplaceAll(config, "EXCLUDES", excludes)
	if err := os.WriteFile(cfg, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	if hostileEnv {
		t.Setenv("GIT_DIFF_OPTS", "--unified=9")
		t.Setenv("GIT_EXTERNAL_DIFF", "false")
	} else {
		t.Setenv("GIT_DIFF_OPTS", "")
		t.Setenv("GIT_EXTERNAL_DIFF", "")
		os.Unsetenv("GIT_DIFF_OPTS")
		os.Unsetenv("GIT_EXTERNAL_DIFF")
	}
	repo := t.TempDir()
	run(t, repo, "init", "-q")
	return repo
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir, msg string) {
	t.Helper()
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", msg)
}

// numbered returns n lines reading "line 1" to "line n".
func numbered(n int) []string {
	var ls []string
	for i := 1; i <= n; i++ {
		ls = append(ls, "line "+itoa(i))
	}
	return ls
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

func join(ls []string) string { return strings.Join(ls, "\n") + "\n" }

func changed(t *testing.T, root, ref string) *Changes {
	t.Helper()
	c, err := Changed(context.Background(), root, ref)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// touched lists the lines of file, from 1 to n, that c says changed.
func touched(c *Changes, file string, n int) []int {
	var out []int
	for i := 1; i <= n; i++ {
		if c.Touches(file, i) {
			out = append(out, i)
		}
	}
	return out
}

// scenario builds the same history whatever config is in force: a file with lines edited, added
// and deleted, a rename with one edit, a deletion, an untracked file, an ignored one, awkward names,
// and an added line that reads like a diff header.
func scenario(t *testing.T, repo string) {
	t.Helper()
	write(t, repo, "pkg/a.go", join(numbered(20)))
	write(t, repo, "pkg/old.go", join(numbered(12)))
	write(t, repo, "pkg/gone.go", join(numbered(3)))
	write(t, repo, "pkg/with space.go", join(numbered(4)))
	write(t, repo, "pkg/ünï.go", join(numbered(4)))
	write(t, repo, ".gitignore", "*.log\n")
	commit(t, repo, "base")
	run(t, repo, "branch", "base")

	a := numbered(20)
	a[2] = "line 3 edited"                                       // line 3
	a[6] = "line 7 edited"                                       // line 7
	a = append(a[:9], append([]string{"inserted"}, a[9:]...)...) // new line 10
	a = append(a[:14], a[15:]...)                                // old line 14 deleted
	a = append(a, "++ looks like a header")                      // new line 21
	write(t, repo, "pkg/a.go", join(a))
	commit(t, repo, "branch work")

	run(t, repo, "mv", "pkg/old.go", "pkg/new.go")
	o := numbered(12)
	o[4] = "line 5 edited"
	write(t, repo, "pkg/new.go", join(o))
	run(t, repo, "rm", "-q", "pkg/gone.go")
	s := numbered(4)
	s[1] = "line 2 edited"
	write(t, repo, "pkg/with space.go", join(s))
	u := numbered(4)
	u[3] = "line 4 edited"
	write(t, repo, "pkg/ünï.go", join(u))
	// Its own text, so no deleted file can pass for its old side.
	write(t, repo, "pkg/fresh.go", join([]string{"fresh 1", "fresh 2", "fresh 3"}))
	write(t, repo, "pkg/debug.log", "ignored\n")
}

func TestChangedLinesPathsAndUntracked(t *testing.T) {
	repo := setup(t, plainConfig, false)
	scenario(t, repo)
	c := changed(t, repo, "base")

	if got, want := touched(c, "pkg/a.go", 25), []int{3, 7, 10, 21}; !reflect.DeepEqual(got, want) {
		t.Errorf("pkg/a.go changed lines = %v, want %v", got, want)
	}
	if got, want := touched(c, "pkg/new.go", 12), []int{5}; !reflect.DeepEqual(got, want) {
		t.Errorf("renamed file changed lines = %v, want %v", got, want)
	}
	if got := touched(c, "pkg/old.go", 12); got != nil {
		t.Errorf("a rename's old side has no new-side lines, got %v", got)
	}
	if got, want := touched(c, "pkg/with space.go", 4), []int{2}; !reflect.DeepEqual(got, want) {
		t.Errorf("file with a space changed lines = %v, want %v", got, want)
	}
	if got, want := touched(c, "pkg/ünï.go", 4), []int{4}; !reflect.DeepEqual(got, want) {
		t.Errorf("non-ASCII file changed lines = %v, want %v", got, want)
	}
	if got, want := touched(c, "pkg/fresh.go", 3), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("untracked file changed lines = %v, want %v", got, want)
	}
	if c.Touches("pkg/debug.log", 1) {
		t.Error("an ignored untracked file counted as changed")
	}

	want := []string{"pkg/a.go", "pkg/fresh.go", "pkg/gone.go", "pkg/new.go", "pkg/old.go", "pkg/with space.go", "pkg/ünï.go"}
	if got := c.Paths(); !reflect.DeepEqual(got, want) {
		t.Errorf("Paths() = %q, want %q", got, want)
	}
	if !c.TouchesDir("pkg") || !c.TouchesDir(".") || c.TouchesDir("pk") || c.TouchesDir("other") {
		t.Error("TouchesDir answered wrongly for pkg, ., pk or other")
	}
}

func TestPureDeletionTouchesNothing(t *testing.T) {
	repo := setup(t, plainConfig, false)
	write(t, repo, "a.go", join(numbered(5)))
	commit(t, repo, "base")
	run(t, repo, "branch", "base")
	ls := numbered(5)
	write(t, repo, "a.go", join(append(ls[:2], ls[3:]...)))
	c := changed(t, repo, "base")
	if got := touched(c, "a.go", 5); got != nil {
		t.Errorf("a pure deletion touched lines %v", got)
	}
	if !c.TouchesDir(".") {
		t.Error("the file a deletion changed should still count as a changed path")
	}
}

func TestPureRenameTouchesNoLinesButBothPaths(t *testing.T) {
	repo := setup(t, plainConfig, false)
	write(t, repo, "contract/skip/README.md", join(numbered(30)))
	commit(t, repo, "base")
	run(t, repo, "branch", "base")
	if err := os.MkdirAll(filepath.Join(repo, "contract", "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "mv", "contract/skip/README.md", "contract/keep/README.md")
	c := changed(t, repo, "base")
	if got := touched(c, "contract/keep/README.md", 30); got != nil {
		t.Errorf("a pure rename touched lines %v", got)
	}
	if !c.TouchesDir("contract/skip") || !c.TouchesDir("contract/keep") {
		t.Errorf("both sides of a rename should be changed paths, got %q", c.Paths())
	}
}

func TestMergeBaseLeavesOutTheRefsOwnLaterWork(t *testing.T) {
	repo := setup(t, plainConfig, false)
	write(t, repo, "a.go", join(numbered(5)))
	commit(t, repo, "base")
	run(t, repo, "checkout", "-q", "-b", "feature")
	ls := numbered(5)
	ls[0] = "feature edit"
	write(t, repo, "a.go", join(ls))
	commit(t, repo, "feature")
	run(t, repo, "checkout", "-q", "main")
	write(t, repo, "b.go", join(numbered(5)))
	commit(t, repo, "main moves on")
	run(t, repo, "checkout", "-q", "feature")

	c := changed(t, repo, "main")
	if got, want := touched(c, "a.go", 5), []int{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("a.go changed lines = %v, want %v", got, want)
	}
	if got := c.Paths(); !reflect.DeepEqual(got, []string{"a.go"}) {
		t.Errorf("work on main after the merge base leaked in: %q", got)
	}
}

func TestModuleBelowTheRepositoryTop(t *testing.T) {
	repo := setup(t, plainConfig, false)
	write(t, repo, "outside.go", join(numbered(3)))
	write(t, repo, "mod/go.mod", "module m\n")
	write(t, repo, "mod/pkg/a.go", join(numbered(3)))
	commit(t, repo, "base")
	run(t, repo, "branch", "base")
	ls := numbered(3)
	ls[1] = "edit"
	write(t, repo, "outside.go", join(ls))
	write(t, repo, "mod/pkg/a.go", join(ls))
	write(t, repo, "mod/pkg/new.go", "x\n")
	write(t, repo, "stray.go", "x\n")

	c := changed(t, filepath.Join(repo, "mod"), "base")
	if got, want := c.Paths(), []string{"pkg/a.go", "pkg/new.go"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Paths() = %q, want %q", got, want)
	}
	if got, want := touched(c, "pkg/a.go", 3), []int{2}; !reflect.DeepEqual(got, want) {
		t.Errorf("pkg/a.go changed lines = %v, want %v", got, want)
	}
}

// TestUserConfigAndEnvironmentChangeNothing builds the same history twice, once under a plain
// config and once under one that sets every diff option a user might, and wants the same answer.
func TestUserConfigAndEnvironmentChangeNothing(t *testing.T) {
	plainRepo := setup(t, plainConfig, false)
	scenario(t, plainRepo)
	plain := changed(t, plainRepo, "base")

	hostileRepo := setup(t, hostileConfig, true)
	scenario(t, hostileRepo)
	hostile := changed(t, hostileRepo, "base")

	if !reflect.DeepEqual(plain.lines, hostile.lines) {
		t.Errorf("changed lines differ under a user's config:\nplain   %v\nhostile %v", plain.lines, hostile.lines)
	}
	if !reflect.DeepEqual(plain.Paths(), hostile.Paths()) {
		t.Errorf("changed paths differ under a user's config:\nplain   %q\nhostile %q", plain.Paths(), hostile.Paths())
	}
}

func TestUnknownRefIsAnError(t *testing.T) {
	repo := setup(t, plainConfig, false)
	write(t, repo, "a.go", "x\n")
	commit(t, repo, "base")
	if _, err := Changed(context.Background(), repo, "no-such-ref"); err == nil {
		t.Error("an unknown ref should be an error")
	}
	if _, err := Changed(context.Background(), repo, ""); err == nil {
		t.Error("an empty ref should be an error")
	}
}

func TestHunkSpan(t *testing.T) {
	cases := []struct {
		line string
		want span
		ok   bool
	}{
		{"@@ -3 +3 @@ func f() {", span{3, 3}, true},
		{"@@ -3,0 +4,2 @@", span{4, 5}, true},
		{"@@ -7,2 +6,0 @@", span{}, false},
		{"@@ -0,0 +1,10 @@", span{1, 10}, true},
	}
	for _, c := range cases {
		got, ok, err := hunkSpan(c.line)
		if err != nil || ok != c.ok || got != c.want {
			t.Errorf("hunkSpan(%q) = %v, %v, %v; want %v, %v", c.line, got, ok, err, c.want, c.ok)
		}
	}
}
