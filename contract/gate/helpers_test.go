package gate_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/cli"
	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
)

// The helpers below build matrix input by hand. A primitive's code sits in a package directory of
// the same name, and its Contract tests sit in contract/<primitive>.

func ref(prim, name string) model.TestRef { return model.TestRef{Primitive: prim, Name: name} }

func ctest(prim, name string, line int, obligations ...string) model.Test {
	return model.Test{
		TestRef: ref(prim, name), Kind: "Test", Dir: "contract/" + prim,
		File: "contract/" + prim + "/" + prim + "_test.go", Line: line, Obligations: obligations,
	}
}

// mut makes a mutant of prim's code in unit, with a real id and hash.
func mut(prim, unit, change string, line int) model.Mutant {
	top, _, _ := strings.Cut(unit, ".")
	id := mutantid.ID{Dir: prim, Unit: unit, Original: change, Replacement: change + "'", N: 1}
	return model.Mutant{
		ID: id.String(), Hash: id.Hash(), Primitive: prim, Dir: prim, File: prim + "/" + prim + ".go",
		Line: line, Col: 2, Unit: unit, Top: top, Original: change, Replace: change + "'", Operator: "arithmetic",
	}
}

func eraseOf(prim, fn string, line int) model.Mutant {
	m := mut(prim, fn, "{ ... }", line)
	m.Erase, m.Operator = true, "erase"
	return m
}

func killedBy(tests ...model.TestRef) model.Row {
	row := model.Row{Ran: tests, Complete: true}
	for _, t := range tests {
		row.Kills = append(row.Kills, model.Kill{Test: t, Kind: model.Assertion})
	}
	return row
}

func passedBy(tests ...model.TestRef) model.Row { return model.Row{Ran: tests, Complete: true} }

func exact(kind string, m model.Mutant) matrix.Declaration {
	return matrix.Declaration{
		Kind: kind, Primitive: m.Primitive, Path: "contract/" + m.Primitive + "/mutants.md", Line: 3,
		Reason: "## Why\n\nBecause.", ID: m.ID,
	}
}

func wildcard(prim, unit string) matrix.Declaration {
	return matrix.Declaration{
		Kind: "unpromised", Primitive: prim, Path: "contract/" + prim + "/mutants.md", Line: 7,
		Reason: "## Open\n\nOpen on purpose.", Wildcard: true, Dir: prim, Unit: unit,
	}
}

// copyFixture copies a fixture module into a temp directory and returns its path.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
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

// gitIn runs git in dir with a fixed identity and clock and none of the machine's config, for
// setting up a repository. It returns the trimmed output.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=flinch", "GIT_AUTHOR_EMAIL=flinch@example.com", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=flinch", "GIT_COMMITTER_EMAIL=flinch@example.com", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo makes dir a git repository with everything in it committed on main.
func newRepo(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, dir, "init", "-q", "-b", "main")
	commitAll(t, dir, "base")
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
}

// cleanGit keeps the machine's git config and diff environment out of flinch's git commands.
func cleanGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_DIFF_OPTS", "GIT_EXTERNAL_DIFF"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

// hostileGit sets every git setting and diff variable that could move which lines a diff calls
// changed, for flinch's git commands from here on: a global config file, the repository's own
// config, and the environment.
func hostileGit(t *testing.T, repo string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	ignore := filepath.Join(t.TempDir(), "ignore")
	write(t, ignore, "*.go\n*.md\n")
	attributes := filepath.Join(t.TempDir(), "attributes")
	write(t, attributes, "*.go -diff\n*.md binary\n")
	settings := "[diff]\n\tnoprefix = true\n\tmnemonicPrefix = true\n\trelative = true\n\trenames = false\n" +
		"\talgorithm = patience\n\tcontext = 9\n\tinterHunkContext = 30\n\texternal = false\n\tindentHeuristic = false\n" +
		"[color]\n\tui = always\n\tdiff = always\n" +
		"[core]\n\tquotePath = true\n\texcludesFile = " + ignore + "\n\tattributesFile = " + attributes + "\n"
	write(t, cfg, settings)
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_DIFF_OPTS", "--unified=9")
	t.Setenv("GIT_EXTERNAL_DIFF", "false")
	f, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(settings); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// replace swaps the one occurrence of old in a file for new.
func replace(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), old) != 1 {
		t.Fatalf("%s holds %q %d times, want once", path, old, strings.Count(string(data), old))
	}
	write(t, path, strings.Replace(string(data), old, new, 1))
}

// flinch runs flinch in the current directory and returns its exit code, stdout and stderr.
func flinch(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := cli.Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// withoutTiming drops the JSON report's timing object, the one part that may differ between runs.
func withoutTiming(t *testing.T, report string) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(report), &v); err != nil {
		t.Fatalf("the report isn't JSON: %v\n%s", err, report)
	}
	delete(v, "timing")
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
