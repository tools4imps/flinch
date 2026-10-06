package runner

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
)

// fixture copies testdata/fx, a module with a package to mutate and a few Contract suites, into a
// temp directory, so no run can touch the repository's own copy.
func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fx")
	err := filepath.WalkDir("testdata/fx", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/fx", p)
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
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// suiteOf lists a fixture primitive's top-level tests, the way the engine will.
func suiteOf(t *testing.T, root, prim string) Suite {
	t.Helper()
	dir := "contract/" + prim
	files, err := filepath.Glob(filepath.Join(root, dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	var tests []string
	for _, f := range files {
		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" {
				continue
			}
			for _, p := range []string{"Test", "Example", "Fuzz"} {
				if strings.HasPrefix(fn.Name.Name, p) {
					tests = append(tests, fn.Name.Name)
				}
			}
		}
	}
	return Suite{Primitive: prim, Dir: dir, ImportPath: "example.com/fx/" + dir, Tests: tests}
}

// at finds the first occurrence of text in a fixture file and returns its byte offset and position.
func at(t *testing.T, root, file, text string) (offset, line, col int) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(src), text)
	if i < 0 {
		t.Fatalf("%s has no %q", file, text)
	}
	line = 1 + strings.Count(string(src[:i]), "\n")
	col = i - strings.LastIndex(string(src[:i]), "\n")
	return i, line, col
}

// mutant replaces the first occurrence of orig in a fixture file. tag tells apart two jobs that use
// the same change.
func mutant(t *testing.T, root, file, orig, repl, tag string) model.Mutant {
	t.Helper()
	i, line, col := at(t, root, file, orig)
	id := path.Dir(file) + ".X: " + orig + " -> " + repl + tag
	return model.Mutant{
		ID: id, Hash: mutantid.Hash(id), Dir: path.Dir(file), ImportPath: "example.com/fx/" + path.Dir(file),
		File: file, Line: line, Col: col, Start: i, End: i + len(orig), Text: repl,
	}
}

func refs(names ...string) []model.TestRef {
	var out []model.TestRef
	for _, n := range names {
		p, name, _ := strings.Cut(n, "/")
		out = append(out, model.TestRef{Primitive: p, Name: name})
	}
	return out
}

func kill(test string, kind model.KillKind, subs ...string) model.Kill {
	return model.Kill{Test: refs(test)[0], Kind: kind, Subtests: subs}
}

// snapshot reads every file under root, to show that a run leaves the module as it found it.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		files[p] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

var coverpkg = []string{"example.com/fx/calc", "example.com/fx/words"}

func TestProveAndRun(t *testing.T) {
	root := fixture(t)
	before := snapshot(t, root)
	t.Setenv("FX_SLOW", filepath.Join(t.TempDir(), "slow"))
	var progress strings.Builder
	o := Options{Root: root, Jobs: 4, Work: t.TempDir(), Progress: &progress}
	suites := []Suite{suiteOf(t, root, "calc"), suiteOf(t, root, "other"), suiteOf(t, root, "par")}

	start := time.Now()
	b, err := Prove(t.Context(), o, suites, coverpkg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Prove took %v", time.Since(start))

	const calc = "calc/calc.go"
	t.Run("Reach", func(t *testing.T) {
		_, l, c := at(t, root, calc, "-x")
		if got, want := b.Reach(calc, l, c), refs("calc/TestAbs"); !reflect.DeepEqual(got, want) {
			t.Errorf("Reach(-x) = %v, want %v", got, want)
		}
		_, l, c = at(t, root, calc, "a + b")
		if got, want := b.Reach(calc, l, c), refs("calc/TestAdd", "other/TestOtherAdd", "par/TestParFast"); !reflect.DeepEqual(got, want) {
			t.Errorf("Reach(a + b) = %v, want %v", got, want)
		}
		_, l, c = at(t, root, "words/words.go", "strings.ToUpper")
		if got, want := b.Reach("words/words.go", l, c), refs("other/ExampleShout", "other/FuzzShout", "other/TestShout"); !reflect.DeepEqual(got, want) {
			t.Errorf("Reach(ToUpper) = %v, want %v", got, want)
		}
		_, l, c = at(t, root, calc, "os.WriteFile")
		if got := b.Reach(calc, l, c); len(got) != 0 {
			t.Errorf("nothing calls pause, but Reach = %v", got)
		}
	})
	t.Run("ReachSpan", func(t *testing.T) {
		_, fl, fc := at(t, root, calc, "{\n\ts := 0")
		_, tl, _ := at(t, root, calc, "return s\n}")
		if got, want := b.ReachSpan(calc, fl, fc, tl+1, 1), refs("calc/TestSum", "par/TestParSum"); !reflect.DeepEqual(got, want) {
			t.Errorf("ReachSpan(Sum) = %v, want %v", got, want)
		}
	})
	t.Run("Linking", func(t *testing.T) {
		if got, want := b.Linking("example.com/fx/words"), refs("other/ExampleShout", "other/FuzzShout", "other/TestOtherAdd", "other/TestShout"); !reflect.DeepEqual(got, want) {
			t.Errorf("Linking(words) = %v, want %v", got, want)
		}
		if got := b.Linking("example.com/fx/calc"); len(got) != 15 {
			t.Errorf("Linking(calc) = %v, want all 15 tests", got)
		}
	})

	sum := mutant(t, root, calc, "i++", "i--", "")
	sumPar := mutant(t, root, calc, "i++", "i--", " #par")
	cases := []struct {
		name string
		job  Job
		want model.Row
	}{
		{"assertion kills in two suites",
			Job{mutant(t, root, calc, "a + b", "a - b", ""), refs("calc/TestAdd", "other/TestOtherAdd")},
			model.Row{Ran: refs("calc/TestAdd", "other/TestOtherAdd"), Complete: true,
				Kills: []model.Kill{kill("calc/TestAdd", model.Assertion), kill("other/TestOtherAdd", model.Assertion)}}},
		{"survivor, with a test that reads testdata from its own directory",
			Job{mutant(t, root, calc, "x < 0", "x <= 0", ""), refs("calc/TestReadsTestdata", "calc/TestAbs")},
			model.Row{Ran: refs("calc/TestAbs", "calc/TestReadsTestdata"), Complete: true}},
		{"subtest kill",
			Job{mutant(t, root, calc, "return -x", "return x", ""), refs("calc/TestAbs")},
			model.Row{Ran: refs("calc/TestAbs"), Complete: true, Kills: []model.Kill{kill("calc/TestAbs", model.Assertion, "TestAbs/neg")}}},
		{"panic, with the later test restarted",
			Job{mutant(t, root, calc, "a / b", "a / (b - b)", ""), refs("calc/TestAdd", "calc/TestDiv", "calc/TestLater")},
			model.Row{Ran: refs("calc/TestAdd", "calc/TestDiv", "calc/TestLater"), Complete: true, Kills: []model.Kill{kill("calc/TestDiv", model.Panic)}}},
		{"panic in init kills every test in the batch",
			Job{mutant(t, root, calc, "ready = true", `panic("init")`, ""), refs("calc/TestAdd", "calc/TestTable", "other/TestShout")},
			model.Row{Ran: refs("calc/TestAdd", "calc/TestTable", "other/TestShout"), Complete: true,
				Kills: []model.Kill{kill("calc/TestAdd", model.Panic), kill("calc/TestTable", model.Panic), kill("other/TestShout", model.Panic)}}},
		{"serial timeout, named and confirmed",
			Job{sum, refs("calc/TestAdd", "calc/TestSum", "calc/TestLater")},
			model.Row{Ran: refs("calc/TestAdd", "calc/TestLater", "calc/TestSum"), Complete: true, Kills: []model.Kill{kill("calc/TestSum", model.Timeout)}}},
		{"parallel timeout names the stuck test",
			Job{sumPar, refs("par/TestParFast", "par/TestParSum")},
			model.Row{Ran: refs("par/TestParFast", "par/TestParSum"), Complete: true, Kills: []model.Kill{kill("par/TestParSum", model.Timeout)}}},
		{"a timeout that doesn't repeat is no kill",
			Job{mutant(t, root, calc, "return 1", `pause(os.Getenv("FX_SLOW"), 6*time.Second); return 1`, ""), refs("calc/TestSlow")},
			model.Row{Ran: refs("calc/TestSlow"), Complete: true}},
		{"a crash that names no test reruns one test per process",
			Job{mutant(t, root, calc, "return strings.TrimSpace(s)", `os.Exit(3); return strings.TrimSpace(s)`, ""), refs("calc/TestAdd", "calc/TestReadsTestdata", "calc/TestLater")},
			model.Row{Ran: refs("calc/TestAdd", "calc/TestLater", "calc/TestReadsTestdata"), Complete: true, Kills: []model.Kill{kill("calc/TestReadsTestdata", model.Panic)}}},
		{"no tests, nothing built",
			Job{mutant(t, root, calc, "s += i", "s -= i", ""), nil},
			model.Row{Complete: true}},
	}
	broken := mutant(t, root, calc, "a + b", `a + "x"`, "")
	jobs := []Job{{broken, refs("calc/TestAdd")}}
	for _, c := range cases {
		jobs = append(jobs, c.job)
	}

	start = time.Now()
	rows, err := b.Run(t.Context(), jobs)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Run took %v for %d jobs", time.Since(start), len(jobs))
	if len(rows) != len(jobs) {
		t.Errorf("got %d rows for %d jobs", len(rows), len(jobs))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rows[c.job.Mutant.Hash]; !reflect.DeepEqual(got, c.want) {
				t.Errorf("row = %+v\nwant  %+v", got, c.want)
			}
		})
	}
	t.Run("a mutant that doesn't build", func(t *testing.T) {
		r := rows[broken.Hash]
		t.Logf("verdict: %s", r.Verdict)
		if !strings.HasPrefix(r.Verdict, "doesn't build: calc/calc.go:") || r.Complete || len(r.Kills) > 0 || len(r.Ran) > 0 {
			t.Errorf("row = %+v", r)
		}
	})
	t.Run("progress", func(t *testing.T) {
		if !strings.Contains(progress.String(), "mutants") {
			t.Errorf("progress = %q", progress.String())
		}
	})
	t.Run("the module is unchanged", func(t *testing.T) {
		if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
			t.Error("the run changed files in the module")
		}
	})
}

func undecided(t *testing.T, err error, want ...string) {
	t.Helper()
	var u *Undecided
	if !errors.As(err, &u) {
		t.Fatalf("err = %v, want *Undecided", err)
	}
	t.Logf("reason: %s", u.Reason)
	for _, w := range want {
		if !strings.Contains(u.Reason, w) {
			t.Errorf("reason %q doesn't mention %q", u.Reason, w)
		}
	}
}

func TestProveUndecided(t *testing.T) {
	root := fixture(t)
	t.Run("a test that fails in the whole run", func(t *testing.T) {
		_, err := Prove(t.Context(), Options{Root: root, Jobs: 2, Work: t.TempDir()}, []Suite{suiteOf(t, root, "fails")}, nil)
		undecided(t, err, "fails/TestFails", "whole")
	})
	t.Run("a test that fails only alone", func(t *testing.T) {
		_, err := Prove(t.Context(), Options{Root: root, Jobs: 2, Work: t.TempDir()}, []Suite{suiteOf(t, root, "order")}, coverpkg)
		undecided(t, err, "order/TestDepends", "alone")
	})
	t.Run("a batch that fails against the clean code", func(t *testing.T) {
		b, err := Prove(t.Context(), Options{Root: root, Jobs: 2, Work: t.TempDir()}, []Suite{suiteOf(t, root, "batchy")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		m := mutant(t, root, "calc/calc.go", "a + b", "a - b", "")
		_, err = b.Run(t.Context(), []Job{{m, refs("batchy/TestX", "batchy/TestY")}})
		undecided(t, err, "batchy/TestX")
	})
}
