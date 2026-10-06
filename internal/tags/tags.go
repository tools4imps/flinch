// Package tags finds the Contract tests and the obligations each one names. A tag is a line comment
// directly above a top-level Test, Example or Fuzz function in a primitive's directory, in the form
// "// Contract: <primitive>/<id>".
package tags

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
)

var (
	tagLine = regexp.MustCompile(`^// Contract: ([a-z0-9_-]+)/([A-Z]+[0-9]+)$`)
	// tagLike catches a comment meant as a tag even when it's misspelled, so it can be reported
	// instead of silently counting for nothing.
	tagLike = regexp.MustCompile(`^//\s*Contract:`)
	// outputPrefix is go/doc's rule for the comment that makes go test run an example.
	outputPrefix = regexp.MustCompile(`(?i)^[[:space:]]*(unordered )?output:`)
)

// Scan reads every primitive's test files that build under the config and returns the Contract
// tests, sorted by primitive, file and line, with the problems their tags have. It also reports a
// tag anywhere else in the module, whatever its build constraints, because a tag outside a
// primitive's tests counts for nothing and its author should hear so.
func Scan(m source.Module, c *contract.Contract, pkgs []source.Package) ([]model.Test, []problem.Problem) {
	s := &scanner{m: m, c: c}
	byDir := map[string]source.Package{}
	for _, p := range pkgs {
		byDir[p.Dir] = p
	}
	primDirs := map[string]bool{}
	named := map[string]bool{}
	for _, prim := range c.Primitives {
		primDirs[prim.Dir] = true
		pkg, ok := byDir[prim.Dir]
		if !ok {
			continue
		}
		for _, file := range pkg.TestFiles {
			s.primitiveFile(prim, pkg, file)
		}
	}
	for _, t := range s.tests {
		for _, ob := range t.Obligations {
			named[ob] = true
		}
	}
	for _, prim := range c.Primitives {
		for _, ob := range prim.Obligations {
			full := prim.Name + "/" + ob.ID
			if !named[full] {
				s.add(prim.Readme, ob.Line, "no Contract test names %s; tag the test that checks it with // Contract: %s", full, full)
			}
		}
	}
	s.elsewhere(primDirs)
	sort.SliceStable(s.tests, func(i, j int) bool {
		a, b := s.tests[i], s.tests[j]
		if a.Primitive != b.Primitive {
			return a.Primitive < b.Primitive
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return s.tests, s.problems
}

type scanner struct {
	m        source.Module
	c        *contract.Contract
	tests    []model.Test
	problems []problem.Problem
}

func (s *scanner) add(path string, line int, format string, args ...any) {
	s.problems = append(s.problems, problem.Problem{Path: path, Line: line, Message: fmt.Sprintf(format, args...)})
}

func (s *scanner) parse(file string) (*token.FileSet, *ast.File, bool) {
	src, err := os.ReadFile(filepath.Join(s.m.Root, filepath.FromSlash(file)))
	if err != nil {
		return nil, nil, false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		// A test file that doesn't parse fails go test on its own, which says more than flinch could.
		return nil, nil, false
	}
	return fset, f, true
}

// primitiveFile records the Contract tests in one of a primitive's test files.
func (s *scanner) primitiveFile(prim *contract.Primitive, pkg source.Package, file string) {
	fset, f, ok := s.parse(file)
	if !ok {
		return
	}
	known := map[string]bool{}
	for _, ob := range prim.Obligations {
		known[ob.ID] = true
	}
	used := map[*ast.Comment]bool{}
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok || d.Recv != nil {
			continue
		}
		kind := testKind(d.Name.Name)
		if kind == "" {
			continue
		}
		t := model.Test{
			TestRef:    model.TestRef{Primitive: prim.Name, Name: d.Name.Name},
			Kind:       kind,
			Dir:        prim.Dir,
			ImportPath: pkg.ImportPath,
			File:       file,
			Line:       fset.Position(d.Type.Func).Line,
		}
		tagged := false
		if d.Doc != nil {
			for _, cm := range d.Doc.List {
				if !tagLike.MatchString(cm.Text) {
					continue
				}
				used[cm] = true
				tagged = true
				line := fset.Position(cm.Slash).Line
				m := tagLine.FindStringSubmatch(cm.Text)
				// An if chain rather than a switch: coverage counts the conditions of an else if,
				// but not the expressions of a case, and flinch maps mutants to tests by coverage.
				if m == nil {
					s.add(file, line, "a tag reads // Contract: <primitive>/<id>, as in // Contract: %s/%s", prim.Name, example(prim))
				} else if m[1] != prim.Name {
					s.add(file, line, "%s names %s/%s, but a Contract test names only obligations of its own primitive, %s", t.Name, m[1], m[2], prim.Name)
				} else if !known[m[2]] {
					s.add(file, line, "%s/%s isn't an obligation in %s", m[1], m[2], prim.Readme)
				} else {
					full := m[1] + "/" + m[2]
					if !slices.Contains(t.Obligations, full) {
						t.Obligations = append(t.Obligations, full)
					}
				}
			}
		}
		if !tagged {
			s.add(file, t.Line, "%s is a Contract test with no tag; name the obligations it checks with // Contract: %s/<id> in the comment directly above it", t.Name, prim.Name)
		}
		if kind == "Example" && tagged && !hasOutput(f, d.Body) {
			s.add(file, t.Line, "%s has no // Output: comment, so go test compiles it but never runs it; end it with the output it prints", t.Name)
		}
		s.tests = append(s.tests, t)
	}
	for _, cg := range f.Comments {
		for _, cm := range cg.List {
			if !used[cm] && tagLike.MatchString(cm.Text) {
				s.add(file, fset.Position(cm.Slash).Line, "this tag sits on no Contract test; put it in the comment group directly above a top-level Test, Example or Fuzz function")
			}
		}
	}
}

// elsewhere reports tags in test files outside the primitives' own directories. It reads every
// _test.go file, whatever its build constraints, since a tag ignored under one set of tags is as
// misleading as one ignored under all of them.
func (s *scanner) elsewhere(primDirs map[string]bool) {
	dirs, _ := source.Dirs(s.m)
	for _, dir := range dirs {
		if primDirs[dir] {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(s.m.Root, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		inside := dir == s.c.Dir || strings.HasPrefix(dir, s.c.Dir+"/")
		for _, e := range entries {
			if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), "_test.go") || source.Skipped(e.Name()) {
				continue
			}
			file := source.Join(dir, e.Name())
			fset, f, ok := s.parse(file)
			if !ok {
				continue
			}
			for _, cg := range f.Comments {
				for _, cm := range cg.List {
					if !tagLike.MatchString(cm.Text) {
						continue
					}
					line := fset.Position(cm.Slash).Line
					if inside {
						s.add(file, line, "this tag sits outside any primitive's directory, so it names nothing; Contract tests live directly in %s/<primitive>", s.c.Dir)
					} else {
						s.add(file, line, "a tag outside the Contract counts for nothing; move the test into %s/<primitive> or drop the tag", s.c.Dir)
					}
				}
			}
		}
	}
}

// testKind says whether go test treats a top-level function name as a test, an example or a fuzz test,
// and returns "" when it treats it as none of them. TestMain sets up the binary and is no test.
func testKind(name string) string {
	if name == "TestMain" {
		return ""
	}
	for _, prefix := range []string{"Test", "Example", "Fuzz"} {
		if isTest(name, prefix) {
			return prefix
		}
	}
	return ""
}

// isTest is go test's rule: the name starts with the prefix and the next rune, if any, isn't
// lowercase, so Testify is no test but Test_x and TestX are.
func isTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

// hasOutput applies go/doc's rule: the last comment in the body must start with "Output:" or
// "Unordered output:".
func hasOutput(f *ast.File, body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	var last *ast.CommentGroup
	for _, cg := range f.Comments {
		if cg.Pos() < body.Pos() {
			continue
		}
		if cg.End() > body.End() {
			break
		}
		last = cg
	}
	return last != nil && outputPrefix.MatchString(last.Text())
}

func example(prim *contract.Primitive) string {
	if len(prim.Obligations) > 0 {
		return prim.Obligations[0].ID
	}
	return "K1"
}
