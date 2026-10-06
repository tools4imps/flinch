// Package covers decides which primitive owns each unit of the module's code. A primitive's covers
// block names packages, trees of packages, functions, types and methods; the most specific reference
// to a unit wins it.
package covers

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
	"github.com/tools4imps/flinch/internal/units"
)

// How specific a reference is. A unit goes to the highest, and a tie to the primitive first by name.
const (
	tree = iota + 1
	pkg
	name
	method
)

// Ownership says which primitive owns each top-level unit.
type Ownership struct {
	owner   map[string]map[string]claim // package dir, then top-level unit name
	named   map[string]bool             // package dirs some reference resolved into
	outside []string
}

type claim struct {
	primitive string
	level     int
}

// Owner returns the primitive that owns a top-level unit, named as package units names it, with
// methods as "Type.Method".
func (o *Ownership) Owner(dir, top string) (primitive string, ok bool) {
	c, ok := o.owner[dir][top]
	return c.primitive, ok
}

// Covered lists, sorted, the package directories with at least one owned unit.
func (o *Ownership) Covered() []string {
	var out []string
	for dir, tops := range o.owner {
		if len(tops) > 0 {
			out = append(out, dir)
		}
	}
	sort.Strings(out)
	return out
}

// Outside lists, sorted, the module's packages that no reference names and that own no unit,
// leaving out the Contract's own directories. They never fail the build; the report lists them so
// nobody mistakes them for checked code.
func (o *Ownership) Outside() []string {
	return append([]string(nil), o.outside...)
}

// A target is one package's units, indexed for resolving references.
type target struct {
	tops    []units.Unit    // top-level units in files flinch may mutate
	all     map[string]bool // every top-level unit, generated and cgo files included
	types   map[string]bool
	vars    map[string]string // a package-level variable's name, to the unit holding it or ""
	methods map[string][]string
}

// Resolve reads every primitive's covers block. A reference that names nothing is a problem at its
// line in the README.md. Units in generated files and files that import "C" are never owned, since
// flinch never mutates them.
func Resolve(c *contract.Contract, pkgs []source.Package, parsed map[string]*source.Parsed) (*Ownership, []problem.Problem) {
	o := &Ownership{owner: map[string]map[string]claim{}, named: map[string]bool{}}
	var probs []problem.Problem
	targets := map[string]*target{}
	var dirs []string
	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}
		targets[p.Dir] = index(p, parsed[p.Dir])
		dirs = append(dirs, p.Dir)
	}
	sort.Strings(dirs)

	for _, prim := range c.Primitives {
		for _, ref := range prim.Covers {
			hits, why := resolve(ref.Text, targets, dirs)
			if why != "" {
				probs = append(probs, problem.Problem{Path: prim.Readme, Line: ref.Line, Message: why})
				continue
			}
			for _, h := range hits {
				o.named[h.dir] = true
				for _, top := range h.tops {
					if o.owner[h.dir] == nil {
						o.owner[h.dir] = map[string]claim{}
					}
					// Primitives arrive sorted by name, so a tie leaves the unit with the first.
					if cur, ok := o.owner[h.dir][top]; !ok || h.level > cur.level {
						o.owner[h.dir][top] = claim{primitive: prim.Name, level: h.level}
					}
				}
			}
		}
	}

	for _, dir := range dirs {
		inContract := dir == c.Dir || strings.HasPrefix(dir, c.Dir+"/")
		if !inContract && !o.named[dir] && len(o.owner[dir]) == 0 {
			o.outside = append(o.outside, dir)
		}
	}
	return o, probs
}

type hit struct {
	dir   string
	tops  []string
	level int
}

const forms = "a covers line names a package directory relative to the module root, as in internal/foo, internal/foo/..., internal/foo.Name or internal/foo.Type.Method"

// resolve turns one covers line into the units it claims, or says why it names nothing.
func resolve(text string, targets map[string]*target, dirs []string) ([]hit, string) {
	text = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(text), "./"), "/")
	if text == "..." || strings.HasSuffix(text, "/...") {
		base := strings.TrimSuffix(strings.TrimSuffix(text, "..."), "/")
		if base == "" {
			base = "."
		}
		var hits []hit
		for _, d := range dirs {
			if base == "." || d == base || strings.HasPrefix(d, base+"/") {
				hits = append(hits, hit{dir: d, tops: names(targets[d].tops), level: tree})
			}
		}
		if len(hits) == 0 {
			return nil, fmt.Sprintf("%s names no package in the module; %s", text, forms)
		}
		return hits, ""
	}

	dir, unit := split(text)
	t := targets[dir]
	if t == nil {
		return nil, fmt.Sprintf("%s names no package in the module; %s", text, forms)
	}
	if unit == "" {
		return []hit{{dir: dir, tops: names(t.tops), level: pkg}}, ""
	}
	if typ, meth, ok := strings.Cut(unit, "."); ok {
		full := typ + "." + meth
		if !t.all[full] {
			return nil, fmt.Sprintf("%s names nothing: %s has no method %s", text, dir, full)
		}
		level := method
		if typ == "init" {
			// "dir.init.0" names one init function, which is no more specific than a function.
			level = name
		}
		return []hit{{dir: dir, tops: t.mutable([]string{full}), level: level}}, ""
	}
	switch {
	case t.all[unit]:
		return []hit{{dir: dir, tops: t.mutable([]string{unit}), level: name}}, ""
	case unit == "init":
		var inits []string
		for u := range t.all {
			if strings.HasPrefix(u, "init.") {
				inits = append(inits, u)
			}
		}
		sort.Strings(inits)
		if len(inits) == 0 {
			return nil, fmt.Sprintf("%s names nothing: %s has no init function", text, dir)
		}
		return []hit{{dir: dir, tops: t.mutable(inits), level: name}}, ""
	case t.types[unit]:
		return []hit{{dir: dir, tops: t.mutable(t.methods[unit]), level: name}}, ""
	}
	if v, ok := t.vars[unit]; ok {
		var tops []string
		if v != "" {
			tops = t.mutable([]string{v})
		}
		return []hit{{dir: dir, tops: tops, level: name}}, ""
	}
	return nil, fmt.Sprintf("%s names nothing: %s has no function, type or variable %s", text, dir, unit)
}

// split separates a reference into its package directory and its unit. The unit starts at the first
// dot after the last slash, so a directory name can't hold a dot. The root package is "." alone, or
// ".." before a unit, as mutant ids write it.
func split(text string) (dir, unit string) {
	if text == "." {
		return ".", ""
	}
	if strings.HasPrefix(text, "..") {
		return ".", text[2:]
	}
	slash := strings.LastIndex(text, "/")
	dot := strings.Index(text[slash+1:], ".")
	if dot < 0 {
		return text, ""
	}
	return text[:slash+1+dot], text[slash+1+dot+1:]
}

func index(p source.Package, parsed *source.Parsed) *target {
	t := &target{all: map[string]bool{}, types: map[string]bool{}, vars: map[string]string{}, methods: map[string][]string{}}
	if parsed == nil {
		return t
	}
	mutable := map[string]bool{}
	for _, f := range p.Mutable {
		mutable[f] = true
	}
	for _, u := range parsed.Units {
		if u.Kind == units.Closure {
			continue
		}
		t.all[u.Name] = true
		if u.Kind == units.Method {
			t.methods[u.Type] = append(t.methods[u.Type], u.Name)
		}
		if mutable[u.File] {
			t.tops = append(t.tops, u)
		}
	}
	for _, typ := range parsed.Types {
		t.types[typ] = true
	}
	for _, f := range parsed.Files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				// Package units names a spec with an initializer by its first name that isn't "_".
				unit := ""
				if len(vs.Values) > 0 {
					unit = "_"
					for _, id := range vs.Names {
						if id.Name != "_" {
							unit = id.Name
							break
						}
					}
				}
				for _, id := range vs.Names {
					if id.Name != "_" {
						t.vars[id.Name] = unit
					}
				}
			}
		}
	}
	return t
}

// mutable keeps the names of units that sit in files flinch may mutate.
func (t *target) mutable(names []string) []string {
	var out []string
	for _, u := range t.tops {
		for _, n := range names {
			if u.Name == n {
				out = append(out, n)
			}
		}
	}
	return out
}

func names(us []units.Unit) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.Name
	}
	return out
}
