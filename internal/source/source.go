// Package source finds the module and its packages the way the go command would, but without running
// it, so flinch contract works on a machine with no Go toolchain on PATH.
package source

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tools4imps/flinch/internal/units"
)

// A Module is the Go module flinch checks.
type Module struct {
	Root string // absolute
	Path string // the module path from go.mod
}

// FindModule walks up from dir to the nearest go.mod and reads the module path from it.
func FindModule(dir string) (Module, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Module{}, err
	}
	for d := abs; ; {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			p := modulePath(data)
			if p == "" {
				return Module{}, fmt.Errorf("%s has no module line", filepath.Join(d, "go.mod"))
			}
			return Module{Root: d, Path: p}, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return Module{}, err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return Module{}, fmt.Errorf("no go.mod in %s or any directory above it; flinch checks a Go module", abs)
		}
		d = parent
	}
}

// modulePath reads the module directive, which may be quoted and may carry a trailing comment.
func modulePath(gomod []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(gomod))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "module" {
			continue
		}
		p := fields[1]
		if unq, err := strconv.Unquote(p); err == nil {
			p = unq
		}
		return p
	}
	return ""
}

// Config says which files build. GOOS and GOARCH come from go/build.Default.
type Config struct {
	Tags []string
}

// A Package is one directory of Go files. Every file name in it is slash-separated and relative to
// the module root, the same form Parsed.Names and mutants use.
type Package struct {
	Dir, ImportPath string
	GoFiles         []string // non-test files that build under the config, sorted
	TestFiles       []string // _test.go files that build under the config, sorted
	Mutable         []string // GoFiles minus generated files and files that import "C"
}

// Dirs lists the module's directories that can hold its packages, slash-separated and relative to
// the root, with "." for the root itself. It leaves out what the go command leaves out of "./...":
// testdata and vendor directories, names starting with "." or "_", and nested modules.
func Dirs(m Module) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(m.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if p != m.Root {
			if Skipped(d.Name()) {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		rel, err := filepath.Rel(m.Root, p)
		if err != nil {
			return err
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(dirs)
	return dirs, err
}

// Skipped reports whether the go command ignores a directory or file by its name alone.
func Skipped(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// Join makes a module-relative path from a package directory and a name inside it.
func Join(dir, name string) string {
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}

// ImportPath is the import path of the package in a module-relative directory.
func ImportPath(m Module, dir string) string {
	if dir == "." || dir == "" {
		return m.Path
	}
	return m.Path + "/" + dir
}

// Packages lists the module's packages under the config, sorted by directory. A directory holding
// only _test.go files is still a package, because a Contract primitive's directory looks like that.
func Packages(m Module, cfg Config) ([]Package, error) {
	dirs, err := Dirs(m)
	if err != nil {
		return nil, err
	}
	ctx := build.Default
	ctx.BuildTags = cfg.Tags
	var pkgs []Package
	for _, dir := range dirs {
		p, err := readPackage(m, &ctx, dir)
		if err != nil {
			return nil, err
		}
		if len(p.GoFiles) > 0 || len(p.TestFiles) > 0 {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs, nil
}

func readPackage(m Module, ctx *build.Context, dir string) (Package, error) {
	p := Package{Dir: dir, ImportPath: ImportPath(m, dir)}
	abs := filepath.Join(m.Root, filepath.FromSlash(dir))
	entries, err := os.ReadDir(abs)
	if err != nil {
		return p, err
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || !regular(filepath.Join(abs, name), e) {
			continue
		}
		ok, err := ctx.MatchFile(abs, name)
		if err != nil {
			return p, err
		}
		if !ok {
			continue
		}
		head, err := parser.ParseFile(token.NewFileSet(), filepath.Join(abs, name), nil, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return p, err
		}
		rel := Join(dir, name)
		if strings.HasSuffix(name, "_test.go") {
			p.TestFiles = append(p.TestFiles, rel)
			continue
		}
		cgo := importsC(head)
		// The go command ignores cgo files when cgo is off, so flinch does too.
		if cgo && !ctx.CgoEnabled {
			continue
		}
		p.GoFiles = append(p.GoFiles, rel)
		if !cgo && !ast.IsGenerated(head) {
			p.Mutable = append(p.Mutable, rel)
		}
	}
	sort.Strings(p.GoFiles)
	sort.Strings(p.TestFiles)
	sort.Strings(p.Mutable)
	return p, nil
}

// regular reports whether an entry is a file, following a symlink the way the go command does.
func regular(abs string, e fs.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.Type().IsRegular()
	}
	info, err := os.Stat(abs)
	return err == nil && info.Mode().IsRegular()
}

func importsC(f *ast.File) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return true
		}
	}
	return false
}

// Parsed is a package's non-test files, parsed with comments, and the units and types they declare.
type Parsed struct {
	Fset  *token.FileSet
	Files []*ast.File
	Names []string // module-relative, same order as Files
	Units []units.Unit
	Types []string // package-level type names, sorted
}

// Parse parses a package's GoFiles. Test files never hold covered code, so they're left out. The
// file set records each file by its module-relative name, so positions print the way problems do.
func Parse(m Module, p Package) (*Parsed, error) {
	out := &Parsed{Fset: token.NewFileSet()}
	types := map[string]bool{}
	for _, name := range p.GoFiles {
		src, err := os.ReadFile(filepath.Join(m.Root, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(out.Fset, name, src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		out.Files = append(out.Files, f)
		out.Names = append(out.Names, name)
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				types[spec.(*ast.TypeSpec).Name.Name] = true
			}
		}
	}
	out.Units = units.Of(out.Files, out.Names)
	for t := range types {
		out.Types = append(out.Types, t)
	}
	sort.Strings(out.Types)
	return out, nil
}
