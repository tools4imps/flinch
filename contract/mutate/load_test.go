package mutate_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/mutate"
	"github.com/tools4imps/flinch/internal/typecheck"
)

// Contract: mutate/M9
func TestAPackageThatCantBeLoadedStopsTheRun(t *testing.T) {
	t.Setenv("GOWORK", "off")
	for _, c := range []struct {
		name, path, want string
	}{
		{"a package that doesn't exist", "example.com/fix/nothere", "example.com/fix/nothere"},
		{"a pattern rather than a package", "example.com/fix/...", "example.com/fix/..."},
	} {
		_, pkgs, err := typecheck.Load(context.Background(), fixture, nil, []string{opsPath, c.path})
		if err == nil {
			t.Errorf("%s: Load succeeded with %d packages", c.name, len(pkgs))
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the error %q doesn't name %s", c.name, err, c.want)
		}
	}

	// A relative root needs the working directory, which can't be found from inside a directory
	// that may not be searched.
	shut := t.TempDir()
	t.Chdir(shut)
	if err := os.Chmod(shut, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(shut, 0o755) })
	if _, pkgs, err := typecheck.Load(context.Background(), ".", nil, []string{opsPath}); err == nil {
		t.Errorf("Load succeeded from a working directory it can't search, with %d packages", len(pkgs))
	}
}

// Contract: mutate/M9
func TestPackagesOutsideTheRootAreRefused(t *testing.T) {
	t.Setenv("GOWORK", "off")
	sub := filepath.Join(fixture, "ops")
	for _, path := range []string{rootPath, "example.com/fix/side"} {
		_, pkgs, err := typecheck.Load(context.Background(), sub, nil, []string{path})
		if err == nil {
			t.Errorf("loading %s from ops succeeded: %+v", path, pkgs[path])
			continue
		}
		if !strings.Contains(err.Error(), "outside the module") {
			t.Errorf("loading %s from ops: %v", path, err)
		}
	}
	// The same root still loads its own package, and names its directory ".".
	_, pkgs, err := typecheck.Load(context.Background(), sub, nil, []string{opsPath})
	if err != nil {
		t.Fatal(err)
	}
	if p := pkgs[opsPath]; p.Dir != "." || p.Names[0] != "arith.go" {
		t.Errorf("ops loaded from its own directory has dir %q and names %q", p.Dir, p.Names)
	}
}

// A workspace can name the module by its real path while Load is given a symlink to it.
//
// Contract: mutate/M9
func TestPackagesAreNamedUnderARootSpelledThroughASymlink(t *testing.T) {
	want, _ := generate(t, opsPath, nil)
	real := copyFixture(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "go.work")
	if err := os.WriteFile(work, []byte("go 1.25\n\nuse "+strconv.Quote(real)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", work)
	l, pkgs, err := typecheck.Load(context.Background(), link, nil, []string{opsPath})
	if err != nil {
		t.Fatal(err)
	}
	p := pkgs[opsPath]
	if p.Dir != "ops" || p.Names[0] != "ops/arith.go" {
		t.Fatalf("ops loaded through the link has dir %q and names %q", p.Dir, p.Names)
	}
	got, _ := mutate.Generate(l, p, mutableAll(pkgs), ownAll, nil)
	sameList(t, "through the link", ids(got), ids(want))
}

// native's cgo files are never mutated, but they have to type-check for its plain files to be, and a
// plain file that sorts after them still finds its place. Loading native needs cgo, so the test asks
// for it rather than letting a missing C compiler turn it off.
//
// Contract: mutate/M9
func TestPackagesWithCgoFilesAreLoaded(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	l, pkgs := load(t, fixture, nativePath)
	p := pkgs[nativePath]
	ms, unviable := mutate.Generate(l, p, mutableAll(pkgs), ownAll, nil)
	sameSet(t, "native", ids(ms), []string{
		"native.Get: { ... } -> { return *new(int) }",
		"native.Get: A + 1 -> A - 1",
		"native.Value: { ... } -> { return *new(int) }",
		"native.Value: A - one() -> A + one()",
		"native.Value: A - one() - two() -> A - one() + two()",
	})
	if unviable != 0 {
		t.Errorf("%d of native's mutants were unviable", unviable)
	}
}
