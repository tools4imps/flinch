// Package mutate finds the mutants in a package's covered code. Every mutant is a splice into one
// file's bytes that keeps each line where it was, and every one has passed a type-check before it
// leaves this package.
package mutate

import (
	"go/ast"
	"go/constant"
	"go/scanner"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutantid"
	"github.com/tools4imps/flinch/internal/typecheck"
	"github.com/tools4imps/flinch/internal/units"
)

// Defaults are the operators a run uses unless --operators narrows them, in the order the design
// lists them.
var Defaults = []string{"erase", "arithmetic", "boundary", "equality", "logical", "negation", "bool", "step", "drop-error", "drop-call"}

// How the operators that have no replacement source show up in ids.
const (
	// erased stands for a function's body in an erase mutant's id. The id names the function already,
	// so the body itself would only make the id long.
	erased = "{ ... }"
	// removed is the replacement a drop-call id shows. An empty replacement would leave the id ending
	// in "-> ", and a declaration line loses that trailing space when it's trimmed, so the id could
	// never be typed back in.
	removed = "(removed)"
)

// A candidate is a mutant before it has a unit, an owner, a number or a type-check.
type candidate struct {
	file       int       // index into the package's files
	at         token.Pos // a position inside the unit the change belongs to
	op         string
	start, end int    // byte offsets in the file
	text       string // what replaces [start, end)
	// fallback replaces [start, end) instead when text fails to type-check. It keeps the removed
	// code's uses alive in a branch that never runs, so a removal that strands an import or a local
	// still makes a mutant with the same id and the same behavior.
	fallback string
	original string // the id's original, collapsed
	replace  string // the id's replacement, collapsed
	erase    bool
}

// Generate returns the viable mutants in files named in mutable, inside top-level units that owner
// gives a primitive, using the operators in ops (nil means Defaults; unknown names are ignored).
// unviable counts the mutants dropped because they failed to type-check, in their fallback form too
// when they have one. The order depends only on
// the source: file name, then start offset, then operator, with a function's erase mutant ahead of
// anything else at the same offset.
func Generate(l *typecheck.Loader, p *typecheck.Package, mutable map[string]bool,
	owner func(top string) (primitive string, ok bool), ops []string) (ms []model.Mutant, unviable int) {
	if ops == nil {
		ops = Defaults
	}
	on := map[string]bool{}
	for _, op := range ops {
		on[op] = true
	}

	all := units.Of(p.Files, p.Names)
	var cands []candidate
	fileUnits := map[int][]units.Unit{}
	for i, f := range p.Files {
		name := p.Names[i]
		if !mutable[name] {
			continue
		}
		for _, u := range all {
			if u.File == name {
				fileUnits[i] = append(fileUnits[i], u)
			}
		}
		g := &gen{info: p.Info, file: i, src: p.Src[name], tf: p.Fset.File(f.Pos()), on: on}
		g.walkFile(f)
		cands = append(cands, g.cands...)
	}

	type placed struct {
		candidate
		unit      *units.Unit
		primitive string
	}
	owners := map[string]string{}
	var kept []placed
	for _, c := range cands {
		u := units.Innermost(fileUnits[c.file], c.at)
		if u == nil {
			continue
		}
		prim, seen := owners[u.Top]
		if !seen {
			prim, _ = owner(u.Top)
			owners[u.Top] = prim
		}
		if prim == "" {
			continue
		}
		kept = append(kept, placed{c, u, prim})
	}

	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i], kept[j]
		if a.file != b.file {
			return p.Names[a.file] < p.Names[b.file]
		}
		if a.start != b.start {
			return a.start < b.start
		}
		if a.erase != b.erase {
			return a.erase
		}
		if a.op != b.op {
			return a.op < b.op
		}
		if a.end != b.end {
			return a.end < b.end
		}
		return a.text < b.text
	})

	// Occurrences are numbered before the type-check drops anything. Whether a mutant type-checks can
	// depend on code outside its unit, and numbering after the drop would let such an edit renumber
	// the unit's ids.
	counts := map[[3]string]int{}
	for _, k := range kept {
		key := [3]string{k.unit.Name, k.original, k.replace}
		counts[key]++
		id := mutantid.ID{Dir: p.Dir, Unit: k.unit.Name, Original: k.original, Replacement: k.replace, N: counts[key]}
		name := p.Names[k.file]
		tf := p.Fset.File(p.Files[k.file].Pos())
		pos := tf.Position(tf.Pos(k.start))
		m := model.Mutant{
			ID:         id.String(),
			Hash:       id.Hash(),
			Primitive:  k.primitive,
			Dir:        p.Dir,
			ImportPath: p.ImportPath,
			File:       name,
			Line:       pos.Line,
			Col:        pos.Column,
			Start:      k.start,
			End:        k.end,
			Text:       k.text,
			Operator:   k.op,
			Unit:       k.unit.Name,
			Top:        k.unit.Top,
			Original:   k.original,
			Replace:    k.replace,
			Erase:      k.erase,
			Linked:     k.unit.Kind == units.Var || k.unit.Kind == units.Init,
		}
		if !l.Viable(p, name, m.Apply(p.Src[name])) {
			m.Text = k.fallback
			if k.fallback == "" || !l.Viable(p, name, m.Apply(p.Src[name])) {
				unviable++
				continue
			}
		}
		ms = append(ms, m)
	}
	return ms, unviable
}

// gen collects the candidates in one file.
type gen struct {
	info  *types.Info
	file  int
	src   []byte
	tf    *token.File
	on    map[string]bool
	cands []candidate
}

// walkFile visits function bodies and package-level variable initializers, the only places units
// hold code. Signatures and type declarations stay untouched, since a change there alters a type
// rather than a computation.
func (g *gen) walkFile(f *ast.File) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Body == nil {
				continue
			}
			g.erase(d)
			g.walk(d.Body)
		case *ast.GenDecl:
			if d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				for _, v := range spec.(*ast.ValueSpec).Values {
					g.walk(v)
				}
			}
		}
	}
}

func (g *gen) walk(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			g.binary(x)
		case *ast.UnaryExpr:
			if x.Op == token.NOT && g.on["negation"] {
				g.swap("negation", x, x.OpPos, len("!"), "")
			}
		case *ast.Ident:
			g.boolean(x)
		case *ast.IncDecStmt:
			if g.on["step"] {
				to := token.DEC
				if x.Tok == token.DEC {
					to = token.INC
				}
				g.swap("step", x, x.TokPos, len(x.Tok.String()), to.String())
			}
		case *ast.AssignStmt:
			g.assign(x)
		case *ast.ReturnStmt:
			g.dropError(x)
		case *ast.ExprStmt:
			g.dropCall(x)
		}
		return true
	})
}

// binaryOps maps each binary operator a default operator changes to that operator's name and the
// token that replaces it.
var binaryOps = map[token.Token]struct {
	op string
	to token.Token
}{
	token.ADD:  {"arithmetic", token.SUB},
	token.SUB:  {"arithmetic", token.ADD},
	token.MUL:  {"arithmetic", token.QUO},
	token.QUO:  {"arithmetic", token.MUL},
	token.REM:  {"arithmetic", token.MUL},
	token.LSS:  {"boundary", token.LEQ},
	token.LEQ:  {"boundary", token.LSS},
	token.GTR:  {"boundary", token.GEQ},
	token.GEQ:  {"boundary", token.GTR},
	token.EQL:  {"equality", token.NEQ},
	token.NEQ:  {"equality", token.EQL},
	token.LAND: {"logical", token.LOR},
	token.LOR:  {"logical", token.LAND},
}

func (g *gen) binary(x *ast.BinaryExpr) {
	b, ok := binaryOps[x.Op]
	if !ok || !g.on[b.op] {
		return
	}
	// Arithmetic needs numbers on both sides, which is what keeps string concatenation out.
	if b.op == "arithmetic" && !(g.numeric(x.X) && g.numeric(x.Y)) {
		return
	}
	g.swap(b.op, x, x.OpPos, len(x.Op.String()), b.to.String())
}

// boolean swaps the predeclared true and false. A field or variable that happens to be called true
// resolves to something else and is left alone.
func (g *gen) boolean(x *ast.Ident) {
	if !g.on["bool"] || (x.Name != "true" && x.Name != "false") {
		return
	}
	if g.info.Uses[x] != types.Universe.Lookup(x.Name) {
		return
	}
	to := "false"
	if x.Name == "false" {
		to = "true"
	}
	g.swap("bool", x, x.Pos(), len(x.Name), to)
}

func (g *gen) assign(x *ast.AssignStmt) {
	if !g.on["step"] || (x.Tok != token.ADD_ASSIGN && x.Tok != token.SUB_ASSIGN) || len(x.Lhs) != 1 {
		return
	}
	// += on a string appends, so only numeric targets count as a step.
	if !g.numeric(x.Lhs[0]) {
		return
	}
	to := token.SUB_ASSIGN
	if x.Tok == token.SUB_ASSIGN {
		to = token.ADD_ASSIGN
	}
	g.swap("step", x, x.TokPos, len(x.Tok.String()), to.String())
}

// dropError makes each error result of a return statement nil, one mutant per result.
func (g *gen) dropError(x *ast.ReturnStmt) {
	if !g.on["drop-error"] {
		return
	}
	errType := types.Universe.Lookup("error").Type()
	for _, r := range x.Results {
		tv, ok := g.info.Types[r]
		if !ok || tv.IsNil() || !types.Identical(tv.Type, errType) {
			continue
		}
		start, end := g.off(r.Pos()), g.off(r.End())
		expr := string(g.src[start:end])
		// A result written across lines keeps its newlines inside parentheses, where they can't
		// trigger a semicolon, so the lines after it keep their numbers.
		text := "nil"
		if n := strings.Count(expr, "\n"); n > 0 {
			text = "(" + strings.Repeat("\n", n) + "nil)"
		}
		g.add(candidate{
			at: x.Pos(), op: "drop-error", start: start, end: end, text: text,
			fallback: "func() error { if false { _ = " + expr + " }; return nil }()",
			original: g.collapsed(x.Pos(), x.End()),
			replace:  mutantid.Collapse(g.slice(x.Pos(), r.Pos()) + "nil" + g.slice(r.End(), x.End())),
		})
	}
}

// dropCall removes a call statement. Its bytes are blanked but its newlines stay, so no line moves.
// Logging calls are left alone, because removing one changes nothing a test should promise.
func (g *gen) dropCall(x *ast.ExprStmt) {
	call, ok := ast.Unparen(x.X).(*ast.CallExpr)
	if !ok || !g.on["drop-call"] || g.logging(call) {
		return
	}
	start, end := g.off(x.Pos()), g.off(x.End())
	stmt := string(g.src[start:end])
	g.add(candidate{
		at: x.Pos(), op: "drop-call", start: start, end: end,
		text:     strings.Repeat("\n", strings.Count(stmt, "\n")),
		fallback: "if false { " + stmt + " }",
		original: g.collapsed(x.Pos(), x.End()),
		replace:  removed,
	})
}

// logging reports whether a call goes to a function or method declared in log or log/slog. A method
// belongs to the package that declares it, so this covers *log.Logger and *slog.Logger too.
func (g *gen) logging(call *ast.CallExpr) bool {
	var id *ast.Ident
	switch f := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return false
	}
	fn, ok := g.info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path := fn.Pkg().Path()
	return path == "log" || path == "log/slog"
}

// erase puts a return of zero values at the top of a function's body, on the line of its "{". A
// result list with names returns them bare, which also yields their zero values at that point. A
// body with no statements, or one that already returns only zero values, is skipped, since erasing
// it would change nothing and the mutant could only live.
func (g *gen) erase(d *ast.FuncDecl) {
	if !g.on["erase"] || len(d.Body.List) == 0 || g.zeroBody(d) {
		return
	}
	var zeros []string
	named := false
	if d.Type.Results != nil {
		for _, f := range d.Type.Results.List {
			if len(f.Names) > 0 {
				named = true
				break
			}
			zeros = append(zeros, "*new("+oneLine(g.slice(f.Type.Pos(), f.Type.End()))+")")
		}
	}
	stmt := "return"
	if !named && len(zeros) > 0 {
		stmt += " " + strings.Join(zeros, ", ")
	}
	at := g.off(d.Body.Lbrace) + 1
	g.add(candidate{
		at: d.Body.Lbrace, op: "erase", start: at, end: at, text: " " + stmt + ";",
		original: erased, replace: mutantid.Collapse("{ " + stmt + " }"), erase: true,
	})
}

// zeroBody reports whether a function's body is one return statement that yields its zero values: a
// bare return, or a return whose every result is a literal zero, nil or false.
func (g *gen) zeroBody(d *ast.FuncDecl) bool {
	if len(d.Body.List) != 1 {
		return false
	}
	ret, ok := d.Body.List[0].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	if len(ret.Results) == 0 {
		return true
	}
	fn, ok := g.info.Defs[d.Name].(*types.Func)
	if !ok {
		return false
	}
	results := fn.Signature().Results()
	if results.Len() != len(ret.Results) {
		return false
	}
	for i, r := range ret.Results {
		if !g.zeroLiteral(ast.Unparen(r), results.At(i).Type()) {
			return false
		}
	}
	return true
}

// zeroLiteral reports whether e, returned as a result of type result, is that type's zero value.
// Only nil counts for an interface result, because a 0 or false returned there is boxed in a non-nil
// interface, while erase returns a nil one.
func (g *gen) zeroLiteral(e ast.Expr, result types.Type) bool {
	switch x := e.(type) {
	case *ast.Ident:
		obj := g.info.Uses[x]
		if obj == types.Universe.Lookup("nil") {
			return true
		}
		return obj == types.Universe.Lookup("false") && !types.IsInterface(result)
	case *ast.BasicLit:
		if types.IsInterface(result) {
			return false
		}
		v := constant.MakeFromLiteral(x.Value, x.Kind, 0)
		switch v.Kind() {
		case constant.Int, constant.Float, constant.Complex:
			return constant.Sign(v) == 0
		case constant.String:
			return constant.StringVal(v) == ""
		}
	}
	return false
}

// swap replaces size bytes at pos with to. The id shows the whole expression or statement around
// the change, before and after.
func (g *gen) swap(op string, whole ast.Node, pos token.Pos, size int, to string) {
	start := g.off(pos)
	g.add(candidate{
		at: whole.Pos(), op: op, start: start, end: start + size, text: to,
		original: g.collapsed(whole.Pos(), whole.End()),
		replace:  mutantid.Collapse(g.slice(whole.Pos(), pos) + to + string(g.src[start+size:g.off(whole.End())])),
	})
}

func (g *gen) add(c candidate) {
	c.file = g.file
	g.cands = append(g.cands, c)
}

func (g *gen) off(p token.Pos) int { return g.tf.Offset(p) }

func (g *gen) slice(from, to token.Pos) string { return string(g.src[g.off(from):g.off(to)]) }

func (g *gen) collapsed(from, to token.Pos) string { return mutantid.Collapse(g.slice(from, to)) }

func (g *gen) numeric(e ast.Expr) bool {
	tv, ok := g.info.Types[e]
	return ok && numeric(tv.Type)
}

// numeric reports whether every type t can stand for is a number. A type parameter counts when its
// constraint limits it to numbers.
func numeric(t types.Type) bool {
	if t == nil {
		return false
	}
	if tp, ok := types.Unalias(t).(*types.TypeParam); ok {
		return numericConstraint(tp.Constraint())
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsNumeric != 0
}

// numericConstraint reports whether a constraint's type set holds only numbers. The set is the
// intersection of the embedded elements, so one numeric element is enough.
func numericConstraint(c types.Type) bool {
	iface, ok := c.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := 0; i < iface.NumEmbeddeds(); i++ {
		switch e := iface.EmbeddedType(i).(type) {
		case *types.Union:
			all := e.Len() > 0
			for j := 0; j < e.Len(); j++ {
				all = all && numeric(e.Term(j).Type())
			}
			if all {
				return true
			}
		default:
			if _, isIface := e.Underlying().(*types.Interface); isIface {
				if numericConstraint(e) {
					return true
				}
			} else if numeric(e) {
				return true
			}
		}
	}
	return false
}

// oneLine puts a piece of Go source on one line, so an erase mutant inserted on the line of a "{"
// adds no lines even when a result type spans several. A newline where Go would insert a semicolon
// becomes one, comments go, and a raw string spanning lines becomes an interpreted one.
func oneLine(src string) string {
	if !strings.Contains(src, "\n") {
		return src
	}
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), nil, 0)
	var b strings.Builder
	prevEnd := -1
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		off := file.Offset(pos)
		if tok == token.SEMICOLON && lit == "\n" {
			b.WriteString(";")
			prevEnd = off
			continue
		}
		text := lit
		if text == "" {
			text = tok.String()
		}
		size := len(text)
		if tok == token.STRING && strings.Contains(lit, "\n") {
			if v, err := strconv.Unquote(lit); err == nil {
				text = strconv.Quote(v)
			}
		}
		if prevEnd >= 0 && off > prevEnd {
			b.WriteString(" ")
		}
		b.WriteString(text)
		prevEnd = off + size
	}
	return strings.TrimSuffix(b.String(), ";")
}
