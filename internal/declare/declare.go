// Package declare checks the declarations in each primitive's mutants.md and indexes them so a run
// can match mutants against them. An equivalent declaration says no program can tell a mutant from
// the original; an unpromised one says the Contract leaves that behavior open.
package declare

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tools4imps/flinch/internal/contract"
	"github.com/tools4imps/flinch/internal/covers"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/problem"
	"github.com/tools4imps/flinch/internal/source"
	"github.com/tools4imps/flinch/internal/units"
)

// Check reports what's wrong with every declaration that can be judged without running anything:
// lines that don't parse, wildcards where only ids belong, blocks with no heading to give their
// reason, declarations filed under the wrong primitive, and declarations naming a unit that's gone.
// These hold in every run, scoped or not.
func Check(c *contract.Contract, own *covers.Ownership, parsed map[string]*source.Parsed) []problem.Problem {
	var ps []problem.Problem
	add := func(path string, line int, format string, args ...any) {
		ps = append(ps, problem.Problem{Path: path, Line: line, Message: fmt.Sprintf(format, args...)})
	}
	headless := map[string]bool{}
	seen := map[string]*contract.Declaration{}
	for _, prim := range c.Primitives {
		for i := range prim.Declarations {
			d := &prim.Declarations[i]
			block := fmt.Sprintf("%s:%d", d.Path, d.Block)
			if d.Reason == "" && !headless[block] {
				headless[block] = true
				add(d.Path, d.Block, "this %s block has no heading above it; put one there that says why these mutants don't count, with any explanation under it", d.Kind)
			}
			pat, err := mutantid.Parse(d.Text)
			if err != nil {
				add(d.Path, d.Line, "%v", err)
				continue
			}
			if pat.Wildcard && d.Kind == "equivalent" {
				add(d.Path, d.Line, "an equivalent block lists exact mutant ids; list the equivalent mutants, or move %s to an unpromised block", d.Text)
			}
			key := canonical(pat)
			if first := seen[key]; first != nil {
				add(d.Path, d.Line, "this repeats the declaration at %s:%d; drop one", first.Path, first.Line)
			} else {
				seen[key] = d
			}
			var u *units.Unit
			if p := parsed[pat.Dir]; p != nil {
				u = units.Find(p.Units, pat.Unit)
			}
			if u == nil {
				add(d.Path, d.Line, "%s.%s names no unit in the module; drop this declaration, or give it the unit's new name if it moved", pat.Dir, pat.Unit)
				continue
			}
			owner, ok := own.Owner(pat.Dir, u.Top)
			switch {
			case !ok:
				add(d.Path, d.Line, "no primitive covers %s.%s, so flinch never mutates it; drop this declaration", pat.Dir, u.Top)
			case owner != prim.Name:
				where := owner + "/mutants.md"
				if p := c.Primitive(owner); p != nil {
					where = p.Dir + "/mutants.md"
				}
				add(d.Path, d.Line, "this declaration belongs in %s, because %s covers %s.%s", where, owner, pat.Dir, u.Top)
			}
		}
	}
	return ps
}

// An Index finds the declaration that accounts for a mutant.
type Index struct {
	all      []*contract.Declaration
	exact    map[string]*contract.Declaration
	wildcard map[string]*contract.Declaration // keyed by mutantid.Prefix(dir, unit)
}

// NewIndex indexes every declaration that parses. Only an unpromised block may hold a wildcard, so
// a wildcard in an equivalent block, which Check reports, matches nothing. When two declarations
// say the same thing the first one, by primitive and then line, is the one a mutant matches.
func NewIndex(c *contract.Contract) *Index {
	ix := &Index{exact: map[string]*contract.Declaration{}, wildcard: map[string]*contract.Declaration{}}
	for _, prim := range c.Primitives {
		for i := range prim.Declarations {
			d := &prim.Declarations[i]
			ix.all = append(ix.all, d)
			pat, err := mutantid.Parse(d.Text)
			if err != nil {
				continue
			}
			key := canonical(pat)
			switch {
			case pat.Wildcard && d.Kind != "unpromised":
			case pat.Wildcard:
				if ix.wildcard[key] == nil {
					ix.wildcard[key] = d
				}
			default:
				if ix.exact[key] == nil {
					ix.exact[key] = d
				}
			}
		}
	}
	return ix
}

// Exact returns the declaration that names this mutant id, or nil. The id is parsed and printed
// again, so a declaration matches whatever whitespace either side was written with.
func (ix *Index) Exact(id string) *contract.Declaration {
	if pat, err := mutantid.Parse(id); err == nil && !pat.Wildcard {
		return ix.exact[canonical(pat)]
	}
	return ix.exact[mutantid.Collapse(id)]
}

// Wildcard returns the unpromised wildcard on unit, or on the top-level unit holding it, or nil.
func (ix *Index) Wildcard(dir, unit string) *contract.Declaration {
	if d := ix.wildcard[mutantid.Prefix(dir, unit)]; d != nil {
		return d
	}
	if top := topOf(unit); top != unit {
		return ix.wildcard[mutantid.Prefix(dir, top)]
	}
	return nil
}

// All returns every declaration, by primitive and then by line, whether it parses or not.
func (ix *Index) All() []*contract.Declaration {
	return append([]*contract.Declaration(nil), ix.all...)
}

var closureSuffix = regexp.MustCompile(`(\.func[0-9]+)+$`)

// topOf is the top-level unit holding a unit: the unit itself unless it names a function literal.
func topOf(unit string) string {
	return closureSuffix.ReplaceAllString(unit, "")
}

// canonical is the form two declarations share when they say the same thing.
func canonical(p mutantid.Pattern) string {
	if p.Wildcard {
		return mutantid.Prefix(p.Dir, p.Unit)
	}
	return strings.TrimSpace(p.ID.String())
}
