package runner

import (
	"bufio"
	"bytes"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/tools4imps/flinch/internal/model"
)

// A span is a stretch of source between two positions, both 1-based and both inclusive.
type span struct{ sl, sc, el, ec int }

func (s span) contains(line, col int) bool {
	return !before(line, col, s.sl, s.sc) && !before(s.el, s.ec, line, col)
}

func (s span) overlaps(o span) bool {
	return !before(o.el, o.ec, s.sl, s.sc) && !before(s.el, s.ec, o.sl, o.sc)
}

func before(l1, c1, l2, c2 int) bool { return l1 < l2 || l1 == l2 && c1 < c2 }

// A block is one coverage block and the Contract tests whose lone run reached it.
type block struct {
	span
	tests []model.TestRef
}

// readProfile returns the blocks a coverage profile counts as run, by module-relative file. A profile
// names files by import path, which dirs maps to module-relative directories; files outside the module
// are left out, since flinch never mutates them.
func readProfile(data []byte, dirs map[string]string) (map[string][]span, error) {
	out := map[string][]span{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		// A line reads "<importpath>/<file>.go:sl.sc,el.ec stmts count".
		f := strings.Fields(line)
		if len(f) < 3 {
			return nil, fmt.Errorf("unreadable coverage line %q", line)
		}
		if f[len(f)-1] == "0" {
			continue
		}
		loc := strings.Join(f[:len(f)-2], " ")
		colon := strings.LastIndexByte(loc, ':')
		if colon < 0 {
			return nil, fmt.Errorf("unreadable coverage line %q", line)
		}
		s, err := parseSpan(loc[colon+1:])
		if err != nil {
			return nil, fmt.Errorf("unreadable coverage line %q", line)
		}
		file := loc[:colon]
		dir, ok := dirs[path.Dir(file)]
		if !ok {
			continue
		}
		rel := path.Join(dir, path.Base(file))
		out[rel] = append(out[rel], s)
	}
	return out, sc.Err()
}

func parseSpan(s string) (span, error) {
	from, to, ok := strings.Cut(s, ",")
	if !ok {
		return span{}, fmt.Errorf("no comma")
	}
	sl, sc, err1 := lineCol(from)
	el, ec, err2 := lineCol(to)
	if err1 != nil || err2 != nil {
		return span{}, fmt.Errorf("bad position")
	}
	return span{sl, sc, el, ec}, nil
}

func lineCol(s string) (int, int, error) {
	l, c, ok := strings.Cut(s, ".")
	if !ok {
		return 0, 0, fmt.Errorf("no dot")
	}
	line, err1 := strconv.Atoi(l)
	col, err2 := strconv.Atoi(c)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("bad number")
	}
	return line, col, nil
}

// coverage gathers the blocks each lone run reached.
type coverage map[string]map[span]map[model.TestRef]bool

func (c coverage) add(t model.TestRef, blocks map[string][]span) {
	for file, spans := range blocks {
		byspan := c[file]
		if byspan == nil {
			byspan = map[span]map[model.TestRef]bool{}
			c[file] = byspan
		}
		for _, s := range spans {
			if byspan[s] == nil {
				byspan[s] = map[model.TestRef]bool{}
			}
			byspan[s][t] = true
		}
	}
}

// blocks turns the gathered coverage into sorted blocks per file, so lookups come out the same way on
// every run.
func (c coverage) blocks() map[string][]block {
	out := map[string][]block{}
	for file, byspan := range c {
		bs := make([]block, 0, len(byspan))
		for s, tests := range byspan {
			bs = append(bs, block{span: s, tests: sortedRefs(tests)})
		}
		sort.Slice(bs, func(i, j int) bool {
			a, b := bs[i].span, bs[j].span
			if a.sl != b.sl || a.sc != b.sc {
				return before(a.sl, a.sc, b.sl, b.sc)
			}
			return before(a.el, a.ec, b.el, b.ec)
		})
		out[file] = bs
	}
	return out
}

func sortedRefs(set map[model.TestRef]bool) []model.TestRef {
	out := make([]model.TestRef, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sortRefs(out)
	return out
}

func sortRefs(ts []model.TestRef) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Primitive != ts[j].Primitive {
			return ts[i].Primitive < ts[j].Primitive
		}
		return ts[i].Name < ts[j].Name
	})
}
