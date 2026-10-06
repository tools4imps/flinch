package mutate_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
)

// Contract: mutate/M2
func TestEveryOtherLineKeepsItsNumberAndText(t *testing.T) {
	var ms []model.Mutant
	for _, ip := range fixturePaths {
		got, _ := generate(t, ip, nil)
		ms = append(ms, got...)
	}
	ops := map[string]bool{}
	for _, m := range ms {
		ops[m.Operator] = true
		src := source(t, m.File)
		out := m.Apply(src)
		if bytes.Equal(out, src) {
			t.Errorf("%s: the mutant changes nothing", m.ID)
			continue
		}
		before := strings.Split(string(src), "\n")
		after := strings.Split(string(out), "\n")
		if len(before) != len(after) {
			t.Errorf("%s: %d lines became %d", m.ID, len(before), len(after))
			continue
		}
		// The change starts on the mutant's line and ends on the line where the replaced bytes end.
		last := m.Line + bytes.Count(src[m.Start:m.End], []byte("\n"))
		for i := range before {
			if (i+1 < m.Line || i+1 > last) && before[i] != after[i] {
				t.Errorf("%s: line %d changed from %q to %q", m.ID, i+1, before[i], after[i])
			}
		}
		line := 1 + bytes.Count(src[:m.Start], []byte("\n"))
		col := m.Start - bytes.LastIndexByte(src[:m.Start], '\n')
		if m.Line != line || m.Col != col {
			t.Errorf("%s: at %d:%d, but its change starts at %d:%d", m.ID, m.Line, m.Col, line, col)
		}
	}
	for _, op := range []string{"erase", "arithmetic", "boundary", "equality", "logical", "negation", "bool", "step", "drop-error", "drop-call"} {
		if !ops[op] {
			t.Errorf("the fixture has no %s mutant to check", op)
		}
	}
}

// Contract: mutate/M2
func TestEachMutantChangesOneExpressionOrStatement(t *testing.T) {
	ms, _ := generate(t, opsPath, nil)
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, m := range ms {
		src := source(t, m.File)
		f := files[m.File]
		if f == nil {
			var err error
			f, err = parser.ParseFile(fset, m.File, src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			files[m.File] = f
		}
		tf := fset.File(f.Pos())
		off := func(p token.Pos) int { return tf.Offset(p) }

		// The replaced bytes lie inside one expression or statement, and the id's original is that
		// node's source. An erase mutant's node is the function's body, which it fills from the
		// start; its id shows the body as "{ ... }".
		var holder ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			// A node that doesn't hold the change has no child that does.
			if n == nil || off(n.Pos()) > m.Start || off(n.End()) < m.End {
				return false
			}
			switch n.(type) {
			case ast.Expr, ast.Stmt:
				text := mutantid.Collapse(string(src[off(n.Pos()):off(n.End())]))
				if text == m.Original || (m.Erase && m.Original == "{ ... }" && off(n.Pos())+1 == m.Start) {
					if holder == nil || off(n.End())-off(n.Pos()) < off(holder.End())-off(holder.Pos()) {
						holder = n
					}
				}
			}
			return true
		})
		if holder == nil {
			t.Errorf("%s: no expression or statement %q holds the change at [%d:%d]", m.ID, m.Original, m.Start, m.End)
			continue
		}
		if m.Erase && m.End != m.Start {
			t.Errorf("%s: an erase mutant replaces %q instead of inserting", m.ID, src[m.Start:m.End])
		}
	}

	// One mutant in full, so every field is pinned to the change it describes.
	m := find(t, ms, "ops.Compare: a < b -> a <= b")
	src := source(t, "ops/compare.go")
	start := bytes.Index(src, []byte("a < b ||")) + 2
	want := model.Mutant{
		ID: "ops.Compare: a < b -> a <= b", Hash: mutantid.Hash("ops.Compare: a < b -> a <= b"),
		Primitive: "mutate", Dir: "ops", ImportPath: opsPath, File: "ops/compare.go",
		Line: 4, Col: 7, Start: start, End: start + 1, Text: "<=", Operator: "boundary",
		Unit: "Compare", Top: "Compare", Original: "a < b", Replace: "a <= b",
	}
	if m != want {
		t.Errorf("got  %+v\nwant %+v", m, want)
	}
}

// Each mutant is a change of its own, so no two share an id. The same change made twice in one unit
// is told apart by #2 on the later one.
//
// Contract: mutate/M2
func TestEachMutantHasAnIDOfItsOwn(t *testing.T) {
	seen := map[string]string{}
	for _, ip := range fixturePaths {
		ms, _ := generate(t, ip, nil)
		for _, m := range ms {
			if prev, ok := seen[m.Hash]; ok {
				t.Errorf("%s and %s share the hash %s", prev, m.ID, m.Hash)
			}
			seen[m.Hash] = m.ID
			if m.Hash != mutantid.Hash(m.ID) {
				t.Errorf("%s: hash %s, want %s", m.ID, m.Hash, mutantid.Hash(m.ID))
			}
		}
	}
	ms, _ := generate(t, opsPath, nil)
	first := find(t, ms, "ops.Twice: x > 0 -> x >= 0")
	second := find(t, ms, "ops.Twice: x > 0 -> x >= 0 #2")
	if first.Line >= second.Line {
		t.Errorf("the first x > 0 is on line %d and the second on %d", first.Line, second.Line)
	}
}
