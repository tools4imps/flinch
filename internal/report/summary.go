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

// splitID splits a full obligation id such as "skip/K1" into its primitive and its own id.
func splitID(id string) (primitive, local string) {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
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
