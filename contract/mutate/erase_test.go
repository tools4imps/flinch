package mutate_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/units"
)

// zeroBodies already return their zero values, or do nothing, so erasing them would change nothing.
var zeroBodies = []string{"Zero", "Blank", "Off", "Rate", "Hex", "Imag", "NUL", "Bare", "Done", "Nothing", "NilErr", "NilPtr"}

// Contract: mutate/M7
func TestEachFunctionGetsOneEraseMutant(t *testing.T) {
	_, pkgs := load(t, fixture, opsPath)
	p := pkgs[opsPath]
	ms, _ := generate(t, opsPath, nil)

	erases := map[string][]model.Mutant{}
	for _, m := range ms {
		if m.Erase {
			erases[m.Unit] = append(erases[m.Unit], m)
			if m.Operator != "erase" {
				t.Errorf("%s: marked Erase but made by %s", m.ID, m.Operator)
			}
			if m.Unit != m.Top {
				t.Errorf("%s: an erase mutant in a function literal", m.ID)
			}
		} else if m.Operator == "erase" {
			t.Errorf("%s: an erase mutant not marked Erase", m.ID)
		}
	}

	kinds := map[string]units.Kind{}
	for _, u := range units.Of(p.Files, p.Names) {
		kinds[u.Name] = u.Kind
		if u.Body == nil || u.Kind == units.Closure || u.Kind == units.Var {
			if len(erases[u.Name]) > 0 {
				t.Errorf("%s isn't a function but got an erase mutant", u.Name)
			}
			continue
		}
		got := len(erases[u.Name])
		switch {
		case slices.Contains(zeroBodies, u.Name):
			if got != 0 {
				t.Errorf("%s already returns its zero values but got %d erase mutants", u.Name, got)
			}
		case got != 1:
			t.Errorf("%s got %d erase mutants, want 1", u.Name, got)
		default:
			// The return sits right after the body's opening brace, on its line.
			m := erases[u.Name][0]
			lbrace := p.Fset.Position(u.Body.Lbrace)
			if m.Start != lbrace.Offset+1 || m.End != m.Start || m.Line != lbrace.Line {
				t.Errorf("%s: inserted at [%d:%d] on line %d; the body opens at offset %d on line %d",
					m.ID, m.Start, m.End, m.Line, lbrace.Offset, lbrace.Line)
			}
		}
	}

	// The run tries mutants marked Erase first. Every other mutant in a function names that function
	// as its top-level unit, which is how the run skips them once the erase mutant lives.
	for _, m := range ms {
		if m.Erase || kinds[m.Top] == units.Var || slices.Contains(zeroBodies, m.Top) {
			continue
		}
		if len(erases[m.Top]) != 1 {
			t.Errorf("%s: its function %s has no erase mutant to go first", m.ID, m.Top)
		}
	}
}

// Contract: mutate/M7
func TestEraseReturnsZeroValuesFromTheFirstLine(t *testing.T) {
	ms, _ := generate(t, opsPath, []string{"erase"})
	dir := copyFixture(t)

	// Every erase mutant at once, from the end of each file back, so earlier offsets stay put.
	byFile := map[string][]model.Mutant{}
	for _, m := range ms {
		byFile[m.File] = append(byFile[m.File], m)
	}
	for file, fms := range byFile {
		sort.Slice(fms, func(i, j int) bool { return fms[i].Start > fms[j].Start })
		src := source(t, file)
		for _, m := range fms {
			src = m.Apply(src)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(file)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	main := `package main

import (
	"fmt"

	"example.com/fix/ops"
)

func main() {
	n := 4
	ops.Touch(&n)
	fmt.Println(ops.Arith(7, 3))
	fmt.Println(ops.Two())
	fmt.Println(ops.Named())
	fmt.Println(ops.Pair())
	fmt.Printf("%q\n", ops.Generic("x"))
	fmt.Println(n)
	fmt.Println(ops.Boxed())
	fmt.Println(ops.Later() == nil)
	fmt.Println(ops.Errors(5))
	fmt.Println(ops.Pointer() == nil)
	fmt.Println(ops.Compare(1, 1))
	fmt.Println(ops.Flags())
}
`
	if err := os.MkdirAll(filepath.Join(dir, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	cmd := exec.CommandContext(context.Background(), "go", "run", "./cmd")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the erased fixture: %v\n%s", err, stderr.String())
	}
	want := strings.Join([]string{"0", "0 <nil>", "0 <nil>", "{0  []}", `""`, "4", "<nil>", "true", "0 <nil>", "true", "false", "false false"}, "\n") + "\n"
	if string(out) != want {
		t.Errorf("the erased functions returned\n%s\nwant\n%s", out, want)
	}
}
