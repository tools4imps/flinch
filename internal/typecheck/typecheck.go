// Package typecheck loads a module's packages with full type information, using the export data the
// go command builds, and checks whether a mutated file still type-checks. flinch asks the toolchain
// for export data rather than type-checking dependencies from source, because the go command already
// knows how to resolve build tags, replacements and vendoring.
package typecheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// A Package is one of the module's packages, parsed and type-checked.
type Package struct {
	Dir, ImportPath string // Dir is slash-separated, relative to the module root; "." for the root
	Fset            *token.FileSet
	Files           []*ast.File
	Names           []string          // module-relative, sorted
	Src             map[string][]byte // by name
	Types           *types.Package
	Info            *types.Info

	// conf is the configuration the package was checked with. Viable checks mutants with the same one,
	// so a mutant is judged by the same rules as the code it came from.
	conf *types.Config
}

// A Loader holds what type-checking needs across calls: the file set and an importer whose cache of
// imported packages fills once and is reused for every mutant.
type Loader struct {
	fset *token.FileSet
	imp  types.Importer

	// mu serializes checks, because the importer's cache isn't safe for concurrent use.
	mu sync.Mutex
}

// listed is the part of go list's JSON that loading needs.
type listed struct {
	ImportPath string
	Dir        string
	Name       string
	Export     string
	GoFiles    []string
	CgoFiles   []string
	ImportMap  map[string]string
	DepOnly    bool
	Module     *struct{ GoVersion string }
	Error      *struct{ Err string }
}

// mode is how every file is parsed. Comments stay, because a //go:build line can set a file's Go
// version, and go/types never needs the parser's own object resolution.
const mode = parser.ParseComments | parser.SkipObjectResolution

// Load runs go list in root for the given import paths and type-checks each of them. The map is keyed
// by import path. Dependencies come from the export data go list builds, so the module has to build.
func Load(ctx context.Context, root string, tags []string, importPaths []string) (*Loader, map[string]*Package, error) {
	l := &Loader{fset: token.NewFileSet()}
	pkgs := map[string]*Package{}
	if len(importPaths) == 0 {
		return l, pkgs, nil
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	all, err := list(ctx, root, tags, importPaths)
	if err != nil {
		return nil, nil, err
	}

	exports := map[string]string{}
	for _, p := range all {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
	}
	l.imp = importer.ForCompiler(l.fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("go list reported no export data for %s", path)
		}
		return os.Open(file)
	})

	requested := map[string]bool{}
	for _, ip := range importPaths {
		requested[ip] = true
	}
	for _, lp := range all {
		if lp.DepOnly || !requested[lp.ImportPath] {
			continue
		}
		p, err := l.load(root, lp)
		if err != nil {
			return nil, nil, err
		}
		pkgs[p.ImportPath] = p
	}
	for _, ip := range importPaths {
		if pkgs[ip] == nil {
			return nil, nil, fmt.Errorf("go list did not report package %s", ip)
		}
	}
	return l, pkgs, nil
}

// list runs go list -deps -export -json and decodes its stream of package objects.
func list(ctx context.Context, root string, tags []string, importPaths []string) ([]listed, error) {
	args := []string{"list", "-deps", "-export", "-json"}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	args = append(args, importPaths...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("go list: %s", msg)
	}
	var all []listed
	dec := json.NewDecoder(&stdout)
	for {
		var p listed
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading go list output: %w", err)
		}
		if p.Error != nil {
			return nil, fmt.Errorf("go list: %s: %s", p.ImportPath, p.Error.Err)
		}
		all = append(all, p)
	}
	return all, nil
}

// load parses and type-checks one package that go list reported.
func (l *Loader) load(root string, lp listed) (*Package, error) {
	dir, err := relDir(root, lp.Dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", lp.ImportPath, err)
	}
	// The compiler sees a package's files sorted by name, and units.Of numbers init functions in that
	// order, so cgo files join the plain ones before sorting.
	files := append(append([]string(nil), lp.GoFiles...), lp.CgoFiles...)
	sort.Strings(files)

	p := &Package{Dir: dir, ImportPath: lp.ImportPath, Fset: l.fset, Src: map[string][]byte{}}
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(lp.Dir, f))
		if err != nil {
			return nil, err
		}
		name := path.Join(dir, f)
		af, err := parser.ParseFile(l.fset, name, src, mode)
		if err != nil {
			return nil, err
		}
		p.Files = append(p.Files, af)
		p.Names = append(p.Names, name)
		p.Src[name] = src
	}

	p.conf = &types.Config{
		Importer: mapped{imp: l.imp, m: lp.ImportMap},
		// cgo files are never mutated, but they have to type-check for the rest of the package to.
		FakeImportC: true,
		Sizes:       types.SizesFor("gc", build.Default.GOARCH),
	}
	if lp.Module != nil && lp.Module.GoVersion != "" {
		p.conf.GoVersion = "go" + lp.Module.GoVersion
	}
	p.Info = &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	p.Types, err = p.conf.Check(lp.ImportPath, l.fset, p.Files, p.Info)
	if err != nil {
		return nil, fmt.Errorf("type-checking %s: %w", lp.ImportPath, err)
	}
	return p, nil
}

// Viable reports whether package p still type-checks with the file called name replaced by src.
// go/types reports unused variables and imports as errors, as the compiler does, so a mutant that
// strands a variable is caught here rather than in a build. Viable is safe to call from several
// goroutines, though the checks run one at a time.
func (l *Loader) Viable(p *Package, name string, src []byte) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	files := make([]*ast.File, len(p.Files))
	copy(files, p.Files)
	i := sort.SearchStrings(p.Names, name)
	if i == len(p.Names) || p.Names[i] != name {
		return false
	}
	f, err := parser.ParseFile(l.fset, name, src, mode)
	// The mutated file is thrown away after this check, so its entry leaves the file set too. Without
	// that, a run with thousands of mutants would hold a line table for every one of them.
	if f != nil {
		if tf := l.fset.File(f.Pos()); tf != nil {
			defer l.fset.RemoveFile(tf)
		}
	}
	if err != nil {
		return false
	}
	files[i] = f
	// With no Error func, go/types stops at the first error, which is all a yes or no needs.
	conf := *p.conf
	conf.Error = nil
	_, err = conf.Check(p.ImportPath, l.fset, files, nil)
	return err == nil
}

// mapped resolves an import path through the importing package's ImportMap before asking the shared
// importer, because export data is keyed by the resolved path, as with vendored packages.
type mapped struct {
	imp types.Importer
	m   map[string]string
}

func (m mapped) Import(path string) (*types.Package, error) {
	if to, ok := m.m[path]; ok {
		path = to
	}
	return m.imp.Import(path)
}

// relDir turns go list's absolute directory into one relative to the module root. When the root was
// given through a symlink, go list may report the resolved path, so both get a second try resolved.
func relDir(root, dir string) (string, error) {
	if rel, ok := within(root, dir); ok {
		return rel, nil
	}
	r, err1 := filepath.EvalSymlinks(root)
	d, err2 := filepath.EvalSymlinks(dir)
	if err1 == nil && err2 == nil {
		if rel, ok := within(r, d); ok {
			return rel, nil
		}
	}
	return "", fmt.Errorf("%s is outside the module at %s", dir, root)
}

func within(root, dir string) (string, bool) {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
