// Package contract loads the Contract: one directory per primitive, each with a README.md of
// numbered obligations and a covers block, and an optional mutants.md of declarations.
package contract

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tools4imps/flinch/internal/problem"
)

// DefaultDir is where the Contract lives when --contract doesn't say otherwise.
const DefaultDir = "contract"

// A Contract is every primitive in the Contract directory.
type Contract struct {
	Dir        string       // slash-separated, relative to the module root
	Primitives []*Primitive // sorted by name
}

// Primitive returns the primitive with the given name, or nil.
func (c *Contract) Primitive(name string) *Primitive {
	for _, p := range c.Primitives {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// A Primitive is one directory of the Contract.
type Primitive struct {
	Name, Dir    string
	Readme       string // path of README.md
	Mutants      string // path of mutants.md, "" when absent
	Obligations  []Obligation
	Covers       []Ref
	Declarations []Declaration
}

// An Obligation is one numbered promise in a README.md. Its full id is the primitive's name, a slash
// and ID.
type Obligation struct {
	ID   string // such as "K1"
	Line int
}

// A Ref is one line of a covers block.
type Ref struct {
	Text string
	Line int
}

// A Declaration is one line of an equivalent or unpromised block.
type Declaration struct {
	Kind   string // "equivalent" or "unpromised"
	Text   string // the line, trimmed
	Path   string
	Line   int
	Reason string // nearest heading above the block plus the prose between; "" when no heading
	Block  int    // the line of the block's opening fence, so one block's lines can be told apart
}

var obligationLine = regexp.MustCompile(`^- \*\*([A-Z]+[0-9]+)\*\*`)

// Load reads the Contract in dir, a path relative to the module root at root. A missing directory is
// a problem, so flinch contract can say so in its usual form; only an unreadable file is an error.
func Load(root, dir string) (*Contract, []problem.Problem, error) {
	if dir == "" {
		dir = DefaultDir
	}
	if filepath.IsAbs(dir) {
		if rel, err := filepath.Rel(root, dir); err == nil {
			dir = rel
		}
	}
	dir = path.Clean(filepath.ToSlash(dir))
	c := &Contract{Dir: dir}
	abs := filepath.Join(root, filepath.FromSlash(dir))
	info, err := os.Lstat(abs)
	if err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return c, []problem.Problem{{Path: dir, Message: "the Contract directory is a symlink, which lets its text change without a change to the Contract; make it a real directory"}}, nil
	}
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return c, []problem.Problem{{Path: dir, Message: "there's no Contract here; flinch looks for a directory with one subdirectory per primitive, each holding a README.md"}}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	l := &loader{root: root, c: c}
	if err := l.walk(abs); err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || skipped(e.Name()) {
			continue
		}
		readme, err := os.Lstat(filepath.Join(abs, e.Name(), "README.md"))
		if errors.Is(err, fs.ErrNotExist) {
			// A subdirectory without a README.md holds helpers, not a primitive.
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		p := &Primitive{Name: e.Name(), Dir: dir + "/" + e.Name()}
		p.Readme = p.Dir + "/README.md"
		c.Primitives = append(c.Primitives, p)
		if readme.Mode()&fs.ModeSymlink != 0 {
			// The walk has reported the symlink already. Reading through it would let the Contract's
			// text live outside the Contract.
			continue
		}
		if err := l.primitive(p); err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(c.Primitives, func(i, j int) bool { return c.Primitives[i].Name < c.Primitives[j].Name })
	return c, l.problems, nil
}

// skipped reports whether flinch ignores a name inside the Contract, as the go command ignores it.
func skipped(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

type loader struct {
	root     string
	c        *Contract
	problems []problem.Problem
}

func (l *loader) add(p string, line int, format string, args ...any) {
	l.problems = append(l.problems, problem.Problem{Path: p, Line: line, Message: fmt.Sprintf(format, args...)})
}

func (l *loader) rel(abs string) string {
	r, err := filepath.Rel(l.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(r)
}

// walk checks every file flinch reads in the Contract: no symlinks, nothing that isn't UTF-8, and no
// Go file in a primitive's directory that isn't a test. Skipped names are left alone, as everywhere.
func (l *loader) walk(abs string) error {
	return filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != abs && skipped(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel := l.rel(p)
		if d.Type()&fs.ModeSymlink != 0 {
			l.add(rel, 0, "a symlink in the Contract lets its text change without a change to the Contract; copy the file in instead")
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) {
			l.add(rel, badLine(data), "this file isn't valid UTF-8; the Contract is text")
		}
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") && l.inPrimitive(p) {
			l.add(rel, 0, "a primitive's directory holds only Contract tests; rename this file to end in _test.go or move it out of the Contract")
		}
		return nil
	})
}

// inPrimitive reports whether a file sits directly in a primitive's directory.
func (l *loader) inPrimitive(file string) bool {
	dir := filepath.Dir(file)
	if l.rel(filepath.Dir(dir)) != l.c.Dir {
		return false
	}
	info, err := os.Lstat(filepath.Join(dir, "README.md"))
	return err == nil && !info.IsDir()
}

// badLine is the line of a file's first byte that isn't part of valid UTF-8.
func badLine(data []byte) int {
	line := 1
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 {
			return line
		}
		if r == '\n' {
			line++
		}
		data = data[size:]
	}
	return line
}

// primitive reads the Markdown files directly inside a primitive's directory.
func (l *loader) primitive(p *Primitive) error {
	abs := filepath.Join(l.root, filepath.FromSlash(p.Dir))
	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "mutants.md" && e.Type().IsRegular() {
			p.Mutants = p.Dir + "/mutants.md"
		}
	}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !strings.HasSuffix(name, ".md") || skipped(name) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			return err
		}
		l.markdown(p, name, string(data))
	}
	return nil
}

// markdown takes what one Markdown file contributes to its primitive. Each kind of block counts in
// one file only, so a block anywhere else is a problem that says where it belongs.
func (l *loader) markdown(p *Primitive, name, text string) {
	file := p.Dir + "/" + name
	blocks, live := scan(text)
	if name == "README.md" {
		seen := map[string]int{}
		for _, ll := range live {
			m := obligationLine.FindStringSubmatch(ll.text)
			if m == nil {
				continue
			}
			if first, dup := seen[m[1]]; dup {
				l.add(file, ll.line, "%s/%s is already an obligation at line %d; give this one its own id", p.Name, m[1], first)
				continue
			}
			seen[m[1]] = ll.line
			p.Obligations = append(p.Obligations, Obligation{ID: m[1], Line: ll.line})
		}
	}
	for _, b := range blocks {
		switch b.info {
		case "covers":
			if name != "README.md" {
				l.add(file, b.line, "a covers block counts only in %s/README.md; move it there", p.Dir)
				continue
			}
			for _, bl := range b.body {
				p.Covers = append(p.Covers, Ref{Text: bl.text, Line: bl.line})
			}
		case "equivalent", "unpromised":
			if name != "mutants.md" {
				l.add(file, b.line, "an %s block counts only in %s/mutants.md; move it there", b.info, p.Dir)
				continue
			}
			for _, bl := range b.body {
				p.Declarations = append(p.Declarations, Declaration{
					Kind: b.info, Text: bl.text, Path: file, Line: bl.line, Reason: b.reason, Block: b.line,
				})
			}
		}
	}
}
