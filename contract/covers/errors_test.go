package covers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tools4imps/flinch/internal/source"
)

// Contract: covers/V7
func TestThereIsNoModuleRootWithoutAGoModThatNamesAModule(t *testing.T) {
	if m, err := source.FindModule(t.TempDir()); err == nil {
		t.Errorf("FindModule found %+v with no go.mod anywhere above", m)
	}

	unnamed := t.TempDir()
	writeFile(t, unnamed, "go.mod", "// A go.mod with no module line.\n\ngo 1.25\n")
	if m, err := source.FindModule(unnamed); err == nil {
		t.Errorf("FindModule found %+v in a go.mod with no module line", m)
	}

	unreadable := t.TempDir()
	if err := os.Mkdir(filepath.Join(unreadable, "go.mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if m, err := source.FindModule(unreadable); err == nil {
		t.Errorf("FindModule found %+v where go.mod is a directory", m)
	}

	if code, out, errs := run(t, unnamed, "contract"); code != 2 {
		t.Errorf("flinch contract exited %d without a module, want 2\n%s%s", code, out, errs)
	}
}

// Every module package is either covered or listed outside the Contract. A package flinch can't
// read could be neither, so it stops the check instead of dropping out of both lists.
//
// Contract: covers/V7
func TestAPackageFlinchCantReadStopsTheCheck(t *testing.T) {
	gone := source.Module{Root: filepath.Join(t.TempDir(), "gone"), Path: "example.com/gone"}
	if dirs, err := source.Dirs(gone); err == nil {
		t.Errorf("Dirs listed %q in a module whose root is gone", dirs)
	}
	if pkgs, err := source.Packages(gone, source.Config{}); err == nil {
		t.Errorf("Packages listed %d packages in a module whose root is gone", len(pkgs))
	}

	for name, text := range map[string]string{
		"a build constraint that doesn't parse": "//go:build (linux\n\npackage y\n",
		"a package clause that doesn't parse":   "package y.z\n",
	} {
		root := copyFixture(t)
		writeFile(t, root, "x/y/bad.go", text)
		m, err := source.FindModule(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := source.Packages(m, source.Config{}); err == nil {
			t.Errorf("Packages read a module holding %s", name)
		}
	}

	root := copyFixture(t)
	writeFile(t, root, "x/y/bad.go", "package y\n\nfunc Broken( {\n")
	m, err := source.FindModule(root)
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := source.Packages(m, source.Config{})
	if err != nil {
		t.Fatalf("Packages failed on a syntax error below the imports, which only Parse reads: %v", err)
	}
	var y, z source.Package
	for _, p := range pkgs {
		switch p.Dir {
		case "x/y":
			y = p
		case "x/y/z":
			z = p
		}
	}
	if parsed, err := source.Parse(m, y); err == nil {
		t.Errorf("Parse read %d files from a package with a syntax error", len(parsed.Files))
	}
	writeFile(t, root, "contract/p/README.md", readme("p", []string{"x/y/z"}))
	writeFile(t, root, "contract/p/p_test.go", contractTest("p"))
	if code, out, errs := run(t, root, "contract"); code != 2 {
		t.Errorf("flinch contract exited %d on a package with a syntax error, want 2\n%s%s", code, out, errs)
	}

	// A file that vanishes between listing the package and parsing it is an error too.
	if err := os.Remove(filepath.Join(root, "x", "y", "z", "z.go")); err != nil {
		t.Fatal(err)
	}
	if parsed, err := source.Parse(m, z); err == nil {
		t.Errorf("Parse read %d files from a package whose file is gone", len(parsed.Files))
	}
}
