// Package units names the pieces of Go code that covers references, mutant ids and declarations talk
// about. A unit is one of these:
//
//	Count           a function
//	Rules.Reason    a method, named by its receiver's type without * or type parameters
//	Count.func2     the second function literal inside Count, counting nested ones, in source order
//	httpClient      a package-level variable's initializer, named by its first name ("_" if it has none)
//	init.0          the first init function, numbered across the package's files as the compiler does
//
// A package-level variable without an initializer has nothing to mutate and isn't a unit.
package units

import (
	"go/ast"
	"go/token"
	"strconv"
)

// Kind says what sort of declaration a unit is.
type Kind int

const (
	Func Kind = iota
	Method
	Closure
	Var
	Init
)

// A Unit is one named piece of a package.
type Unit struct {
	Name string
	Kind Kind
	Top  string // the top-level unit it sits in: itself, unless it's a closure
	Type string // the receiver's type name, for a method
	File string // the file name as given to Of
	Pos  token.Pos
	End  token.Pos
	// Sig and Body are set for functions, methods, init functions and closures.
	Sig  *ast.FuncType
	Body *ast.BlockStmt
}

// Of names every unit in a package. The files must be sorted by name. The compiler numbers init
// functions across the plain files in that order and then across the cgo files, because the go
// command hands it the files cgo generates after the rest, so Of numbers them the same way.
func Of(files []*ast.File, names []string) []Unit {
	var us []Unit
	first := initStarts(files)
	for i, f := range files {
		name := names[i]
		inits := first[i]
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				u := Unit{Name: d.Name.Name, Kind: Func, File: name, Pos: d.Pos(), End: d.End(), Sig: d.Type, Body: d.Body}
				switch {
				case d.Recv != nil && len(d.Recv.List) > 0:
					u.Kind = Method
					u.Type = ReceiverType(d.Recv.List[0].Type)
					u.Name = u.Type + "." + d.Name.Name
				case d.Name.Name == "init":
					u.Kind = Init
					u.Name = "init." + strconv.Itoa(inits)
					inits++
				}
				u.Top = u.Name
				us = append(us, u)
				if d.Body != nil {
					us = append(us, closures([]ast.Node{d.Body}, u.Name, name)...)
				}
			case *ast.GenDecl:
				if d.Tok != token.VAR {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					if len(vs.Values) == 0 {
						continue
					}
					n := "_"
					for _, id := range vs.Names {
						if id.Name != "_" {
							n = id.Name
							break
						}
					}
					us = append(us, Unit{Name: n, Kind: Var, Top: n, File: name, Pos: vs.Pos(), End: vs.End()})
					values := make([]ast.Node, len(vs.Values))
					for i, v := range vs.Values {
						values[i] = v
					}
					us = append(us, closures(values, n, name)...)
				}
			}
		}
	}
	return us
}

// initStarts gives the number of each file's first init function: the plain files in order, then
// the files that import "C".
func initStarts(files []*ast.File) []int {
	starts := make([]int, len(files))
	n := 0
	for _, cgo := range []bool{false, true} {
		for i, f := range files {
			if importsC(f) != cgo {
				continue
			}
			starts[i] = n
			for _, decl := range f.Decls {
				if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Name.Name == "init" {
					n++
				}
			}
		}
	}
	return starts
}

func importsC(f *ast.File) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return true
		}
	}
	return false
}

// closures names the function literals under the nodes of one top-level unit, in source order,
// nested ones included.
func closures(nodes []ast.Node, top, file string) []Unit {
	var us []Unit
	for _, n := range nodes {
		ast.Inspect(n, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok {
				us = append(us, Unit{
					Name: top + ".func" + strconv.Itoa(len(us)+1), Kind: Closure, Top: top, File: file,
					Pos: lit.Pos(), End: lit.End(), Sig: lit.Type, Body: lit.Body,
				})
			}
			return true
		})
	}
	return us
}

// ReceiverType is a method receiver's type name, without a pointer or type parameters.
func ReceiverType(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

// Innermost is the smallest unit that contains pos, or nil when pos sits in none, such as in a type
// or constant declaration.
func Innermost(us []Unit, pos token.Pos) *Unit {
	var best *Unit
	for i := range us {
		u := &us[i]
		if u.Pos <= pos && pos < u.End && (best == nil || u.End-u.Pos < best.End-best.Pos) {
			best = u
		}
	}
	return best
}

// Find returns the unit with the given name, or nil.
func Find(us []Unit, name string) *Unit {
	for i := range us {
		if us[i].Name == name {
			return &us[i]
		}
	}
	return nil
}
