package report

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/tools4imps/flinch/internal/matrix"
	"github.com/tools4imps/flinch/internal/model"
)

// counts is the summary both formats print.
type counts struct {
	Mutants, Killed, Lived, Unreached, Erased, Skipped, Declared, NoVerdict, Unheld int
	Equivalent, Unpromised, Wildcard                                                int

	Obligations, Holding, Hollow, Unjudged, CrashOnly, NoSole int

	Tests, Blind int
}

func count(m *matrix.Result) counts {
	var c counts
	if m == nil {
		return c
	}
	c.Mutants = len(m.Mutants)
	for _, mu := range m.Mutants {
		switch mu.Status {
		case model.Killed:
			c.Killed++
		case model.Lived:
			c.Lived++
		case model.Unreached:
			c.Unreached++
		case model.Erased:
			c.Erased++
		case model.Skipped:
			c.Skipped++
		case model.NoVerdict:
			c.NoVerdict++
		case model.Declared:
			c.Declared++
			switch {
			case mu.Declaration != nil && mu.Declaration.Wildcard:
				c.Wildcard++
			case mu.Declaration != nil && mu.Declaration.Kind == "equivalent":
				c.Equivalent++
			default:
				c.Unpromised++
			}
		}
	}
	c.Unheld = c.Lived + c.Unreached + c.Erased

	c.Obligations = len(m.Obligations)
	for _, o := range m.Obligations {
		switch {
		case len(o.Holds) > 0:
			c.Holding++
		case o.Hollow:
			c.Hollow++
		default:
			c.Unjudged++
		}
		if o.CrashOnly {
			c.CrashOnly++
		}
		if len(o.SoleHolds) == 0 {
			c.NoSole++
		}
	}
	c.Tests = len(m.Tests)
	for _, t := range m.Tests {
		if t.Blind {
			c.Blind++
		}
	}
	return c
}

// score is killed over killed plus unheld, as a percentage cut, not rounded, to one decimal place,
// so a run with an unheld mutant never prints 100%. It's false when both counts are zero.
func (c counts) score() (float64, bool) {
	d := c.Killed + c.Unheld
	if d == 0 {
		return 0, false
	}
	return math.Floor(1000*float64(c.Killed)/float64(d)) / 10, true
}

// lessObligation orders full obligation ids by primitive, then by id with runs of digits compared
// as numbers, so skip/K2 comes before skip/K10.
func lessObligation(a, b string) bool {
	pa, ia := splitID(a)
	pb, ib := splitID(b)
	if pa != pb {
		return pa < pb
	}
	return natural(ia, ib)
}

func splitID(id string) (primitive, local string) {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

func natural(a, b string) bool {
	for a != "" && b != "" {
		da, db := isDigit(a[0]), isDigit(b[0])
		if da && db {
			na, ra := leadingDigits(a)
			nb, rb := leadingDigits(b)
			if na != nb {
				// Compare by length first, then text, so the numbers can be of any size.
				ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
				if len(ta) != len(tb) {
					return len(ta) < len(tb)
				}
				if ta != tb {
					return ta < tb
				}
				return na < nb
			}
			a, b = ra, rb
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func leadingDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// sortedObligations returns the obligations in id order.
func sortedObligations(os []matrix.Obligation) []matrix.Obligation {
	out := append([]matrix.Obligation(nil), os...)
	sort.SliceStable(out, func(i, j int) bool { return lessObligation(out[i].ID, out[j].ID) })
	return out
}

// sortedMutants returns the mutants ordered by file, line, column and id.
func sortedMutants(ms []matrix.Mutant) []matrix.Mutant {
	out := append([]matrix.Mutant(nil), ms...)
	sort.SliceStable(out, func(i, j int) bool { return lessMutant(out[i], out[j]) })
	return out
}

func lessMutant(a, b matrix.Mutant) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Col != b.Col {
		return a.Col < b.Col
	}
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Hash < b.Hash
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// sortedStrs returns a sorted copy of s that is never nil, so JSON prints [] and never null.
func sortedStrs(s []string) []string {
	out := append([]string{}, s...)
	sort.Strings(out)
	return out
}
