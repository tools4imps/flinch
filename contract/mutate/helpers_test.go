package mutate_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutate"
	"github.com/tools4imps/flinch/internal/typecheck"
)

// fixture is a module with every default operator's targets in package ops, a root package, and a
// package with cgo files in native. Only the test for cgo loads native, so a machine without a C
// compiler fails that one test rather than all of them.
const fixture = "testdata/fix"

const (
	rootPath   = "example.com/fix"
	opsPath    = "example.com/fix/ops"
	nativePath = "example.com/fix/native"
)

var fixturePaths = []string{rootPath, opsPath}

// load type-checks the fixture packages in the module at root. A go.work above the module would pull
// it into a workspace it isn't part of, so workspaces are off.
func load(t *testing.T, root string, paths ...string) (*typecheck.Loader, map[string]*typecheck.Package) {
	t.Helper()
	t.Setenv("GOWORK", "off")
	l, pkgs, err := typecheck.Load(context.Background(), root, nil, paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range paths {
		if pkgs[ip] == nil {
			t.Fatalf("Load returned no package %s", ip)
		}
	}
	return l, pkgs
}

// mutableAll marks every file of the packages as mutable, apart from the cgo files in native, which
// the engine never marks.
func mutableAll(pkgs map[string]*typecheck.Package) map[string]bool {
	m := map[string]bool{}
	for _, p := range pkgs {
		for _, name := range p.Names {
			m[name] = true
		}
	}
	delete(m, "native/b.go")
	delete(m, "native/c.go")
	return m
}

func ownAll(top string) (string, bool) { return "mutate", true }

// A run is what Generate returned for one package.
type run struct {
	ms       []model.Mutant
	unviable int
}

var (
	shared     sync.Mutex
	sharedLoad struct {
		l    *typecheck.Loader
		pkgs map[string]*typecheck.Package
	}
	sharedRuns = map[string]run{}
)

// generate returns package ip's mutants in the checked-in fixture, with every file mutable and every
// unit owned. Tests in one process share the loaded packages and the results, because loading runs
// go list and every test reads the same fixture.
func generate(t *testing.T, ip string, ops []string) ([]model.Mutant, int) {
	t.Helper()
	shared.Lock()
	defer shared.Unlock()
	if sharedLoad.l == nil {
		sharedLoad.l, sharedLoad.pkgs = load(t, fixture, fixturePaths...)
	}
	key := ip + "|" + strings.Join(ops, ",")
	if ops == nil {
		key += "|nil"
	}
	r, ok := sharedRuns[key]
	if !ok {
		r.ms, r.unviable = mutate.Generate(sharedLoad.l, sharedLoad.pkgs[ip], mutableAll(sharedLoad.pkgs), ownAll, ops)
		sharedRuns[key] = r
	}
	return slices.Clone(r.ms), r.unviable
}

// source reads a fixture file by its module-relative name.
func source(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixture, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func ids(ms []model.Mutant) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.ID
	}
	return out
}

func find(t *testing.T, ms []model.Mutant, id string) model.Mutant {
	t.Helper()
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no mutant %q", id)
	return model.Mutant{}
}

// sameList reports a difference between two lists of lines, naming what's missing and what's extra.
func sameList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if slices.Equal(got, want) {
		return
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s: missing %s", what, w)
		}
	}
	for _, g := range got {
		if !slices.Contains(want, g) {
			t.Errorf("%s: unexpected %s", what, g)
		}
	}
	if !t.Failed() {
		t.Errorf("%s: the right lines in the wrong order\ngot:  %q\nwant: %q", what, got, want)
	}
}

// sameSet is sameList for lists whose order isn't promised.
func sameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	got, want = slices.Clone(got), slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	sameList(t, what, got, want)
}

// copyFixture copies the fixture module to a fresh directory, for tests that edit it or need it at a
// second path.
func copyFixture(t *testing.T) string {
	t.Helper()
	to := t.TempDir()
	err := filepath.WalkDir(fixture, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return to
}
