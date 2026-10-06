package mutate_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tools4imps/flinch/internal/model"
	"github.com/tools4imps/flinch/internal/mutate"
)

var defaults = []string{"erase", "arithmetic", "boundary", "equality", "logical", "negation", "bool", "step", "drop-error", "drop-call"}

// changes describes each of an operator's mutants in package ops as its id, the bytes it replaces
// and what replaces them.
func changes(t *testing.T, op string) []string {
	t.Helper()
	ms, _ := generate(t, opsPath, []string{op})
	var out []string
	for _, m := range ms {
		if m.Operator != op {
			t.Errorf("narrowed to %s, Generate returned a %s mutant: %s", op, m.Operator, m.ID)
		}
		out = append(out, fmt.Sprintf("%s | %q => %q", m.ID, source(t, m.File)[m.Start:m.End], m.Text))
	}
	return out
}

// Contract: mutate/M4
func TestTheDefaultOperators(t *testing.T) {
	if !slices.Equal(mutate.Defaults, defaults) {
		t.Errorf("Defaults = %q, want %q", mutate.Defaults, defaults)
	}
	all, unviable := generate(t, opsPath, nil)
	named, namedUnviable := generate(t, opsPath, defaults)
	if !slices.Equal(all, named) || unviable != namedUnviable {
		t.Error("no operators named gave different mutants from naming the defaults")
	}
	seen := map[string]bool{}
	for _, m := range all {
		seen[m.Operator] = true
	}
	for _, op := range defaults {
		if !seen[op] {
			t.Errorf("the defaults made no %s mutant", op)
		}
	}
}

// Contract: mutate/M4
func TestOperatorsNarrowARun(t *testing.T) {
	all, _ := generate(t, opsPath, nil)
	for _, ops := range [][]string{{"bool", "drop-call"}, {"erase"}, {"step", "boundary", "negation"}} {
		some, _ := generate(t, opsPath, ops)
		var want []model.Mutant
		for _, m := range all {
			if slices.Contains(ops, m.Operator) {
				want = append(want, m)
			}
		}
		if len(want) == 0 {
			t.Fatalf("the fixture has no %v mutants", ops)
		}
		if !slices.Equal(some, want) {
			t.Errorf("narrowing to %v gave %d mutants, want the %d of the full run in its order", ops, len(some), len(want))
		}
	}
	none, unviable := generate(t, opsPath, []string{})
	if len(none) != 0 || unviable != 0 {
		t.Errorf("an empty operator list gave %d mutants and %d unviable", len(none), unviable)
	}
}

// Contract: mutate/M4
func TestErase(t *testing.T) {
	sameSet(t, "erase", changes(t, "erase"), []string{
		`ops.Arith: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Warm: { ... } -> { return *new(Celsius) } | "" => " return *new(Celsius);"`,
		`ops.Bits: { ... } -> { return *new(uint) } | "" => " return *new(uint);"`,
		`ops.Signs: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Compound: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.init.0: { ... } -> { return } | "" => " return;"`,
		`ops.Calls: { ... } -> { return } | "" => " return;"`,
		`ops.Strand: { ... } -> { return } | "" => " return;"`,
		`ops.Now: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Check: { ... } -> { return *new(error) } | "" => " return *new(error);"`,
		`ops.Compare: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Order: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Present: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Twice: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Greet: { ... } -> { return *new(string) } | "" => " return *new(string);"`,
		`ops.Tag: { ... } -> { return *new(Label) } | "" => " return *new(Label);"`,
		`ops.Mixed: { ... } -> { return *new(string) } | "" => " return *new(string);"`,
		`ops.Boxed: { ... } -> { return *new(any) } | "" => " return *new(any);"`,
		`ops.BoxedFalse: { ... } -> { return *new(any) } | "" => " return *new(any);"`,
		`ops.BoxedEmpty: { ... } -> { return *new(any) } | "" => " return *new(any);"`,
		`ops.One: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Letter: { ... } -> { return *new(rune) } | "" => " return *new(rune);"`,
		`ops.Text: { ... } -> { return *new(string) } | "" => " return *new(string);"`,
		`ops.Yes: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Pointer: { ... } -> { return *new(*int) } | "" => " return *new(*int);"`,
		`ops.Two: { ... } -> { return *new(int), *new(error) } | "" => " return *new(int), *new(error);"`,
		`ops.Named: { ... } -> { return } | "" => " return;"`,
		`ops.Touch: { ... } -> { return } | "" => " return;"`,
		`ops.Generic: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Bound: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Pair: { ... } -> { return *new(struct { a int; b string "tag:\"two\nlines\""; c []int ` + "`" + `json:"c"` + "`" + `; }) } | "" => " return *new(struct { a int; b string \"tag:\\\"two\\nlines\\\"\"; c []int ` + "`" + `json:\"c\"` + "`" + `; });"`,
		`ops.Errors: { ... } -> { return *new(int), *new(error) } | "" => " return *new(int), *new(error);"`,
		`ops.myErr.Error: { ... } -> { return *new(string) } | "" => " return *new(string);"`,
		`ops.Concrete: { ... } -> { return *new(*myErr) } | "" => " return *new(*myErr);"`,
		`ops.Boxing: { ... } -> { return *new(error) } | "" => " return *new(error);"`,
		`ops.Pass: { ... } -> { return *new(int), *new(error) } | "" => " return *new(int), *new(error);"`,
		`ops.Wrap: { ... } -> { return *new(error) } | "" => " return *new(error);"`,
		`ops.Later: { ... } -> { return *new(func() error) } | "" => " return *new(func() error);"`,
		`ops.Flags: { ... } -> { return *new(bool), *new(bool) } | "" => " return *new(bool), *new(bool);"`,
		`ops.Not: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Odd: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Make: { ... } -> { return *new(odd) } | "" => " return *new(odd);"`,
		`ops.Shadow: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Sum: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Double: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Halve: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Join: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Plus: { ... } -> { return *new(T) } | "" => " return *new(T);"`,
		`ops.Sig: { ... } -> { return *new([1 + 1]int) } | "" => " return *new([1 + 1]int);"`,
		`ops.Quit: { ... } -> { return } | "" => " return;"`,
		`ops.Step: { ... } -> { return } | "" => " return;"`,
		`ops.init.1: { ... } -> { return } | "" => " return;"`,
		`ops.init.2: { ... } -> { return } | "" => " return;"`,
		`ops.List.Len: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Pair2.Key: { ... } -> { return *new(K) } | "" => " return *new(K);"`,
		`ops.plain.Bump: { ... } -> { return } | "" => " return;"`,
		`ops.plain.Same: { ... } -> { return *new(bool) } | "" => " return *new(bool);"`,
		`ops.Count: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Small: { ... } -> { return *new(uint8) } | "" => " return *new(uint8);"`,
		`ops.Scale: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.Must: { ... } -> { return *new(int) } | "" => " return *new(int);"`,
		`ops.init.3: { ... } -> { return } | "" => " return;"`,
	})
}

// A result type written across lines goes on one line, spelled as written apart from the line
// breaks, even where gofmt would have spelled it otherwise.
//
// Contract: mutate/M4
func TestEraseKeepsAResultTypeAsWritten(t *testing.T) {
	dir := copyFixture(t)
	semi := "package ops\n\nfunc Semi() struct {\n\ta int;b []int\n} {\n\treturn struct {\n\t\ta int;b []int\n\t}{}\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "ops", "semi.go"), []byte(semi), 0o644); err != nil {
		t.Fatal(err)
	}
	l, pkgs := load(t, dir, opsPath)
	ms, _ := mutate.Generate(l, pkgs[opsPath], map[string]bool{"ops/semi.go": true}, ownAll, []string{"erase"})
	if got, want := ids(ms), []string{"ops.Semi: { ... } -> { return *new(struct { a int;b []int; }) }"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Contract: mutate/M4
func TestArithmetic(t *testing.T) {
	sameSet(t, "arithmetic", changes(t, "arithmetic"), []string{
		`ops.Arith: a + b -> a - b | "+" => "-"`,
		`ops.Arith: a - b -> a + b | "-" => "+"`,
		`ops.Arith: a * b -> a / b | "*" => "/"`,
		`ops.Arith: a / b -> a * b | "/" => "*"`,
		`ops.Arith: a % b -> a * b | "%" => "*"`,
		`ops.Arith: sum + diff -> sum - diff | "+" => "-"`,
		`ops.Arith: sum + diff + prod -> sum + diff - prod | "+" => "-"`,
		`ops.Arith: sum + diff + prod + quot -> sum + diff + prod - quot | "+" => "-"`,
		`ops.Arith: sum + diff + prod + quot + rem -> sum + diff + prod + quot - rem | "+" => "-"`,
		`ops.Warm: c + by -> c - by | "+" => "-"`,
		`ops.Signs: -a + ^a -> -a - ^a | "+" => "-"`,
		`ops.Now: func() int { return *n }() + 1 -> func() int { return *n }() - 1 | "+" => "-"`,
		`ops.Mixed: n+1 -> n-1 | "+" => "-"`,
		`ops.Double: x * 2 -> x / 2 | "*" => "/"`,
		`ops.Halve: x / 2 -> x * 2 | "/" => "*"`,
		`ops.limit: 10 * 2 -> 10 / 2 | "*" => "/"`,
		`ops.second: 2 + 3 -> 2 - 3 | "+" => "-"`,
		`ops._: 4 - 1 -> 4 + 1 | "-" => "+"`,
		`ops.grouped: 6 * 7 -> 6 / 7 | "*" => "/"`,
		`ops.Count.func1: b() + 1 -> b() - 1 | "+" => "-"`,
		`ops.Count: a() * c() -> a() / c() | "*" => "/"`,
	})
}

// Contract: mutate/M4
func TestBoundary(t *testing.T) {
	sameSet(t, "boundary", changes(t, "boundary"), []string{
		`ops.Check: n > 9 -> n >= 9 | ">" => ">="`,
		`ops.Compare: a < b -> a <= b | "<" => "<="`,
		`ops.Compare: a <= 0 -> a < 0 | "<=" => "<"`,
		`ops.Compare: a > b -> a >= b | ">" => ">="`,
		`ops.Compare: a >= 1 -> a > 1 | ">=" => ">"`,
		`ops.Order: a < b -> a <= b | "<" => "<="`,
		`ops.Twice: x > 0 -> x >= 0 | ">" => ">="`,
		`ops.Twice: x > 0 -> x >= 0 #2 | ">" => ">="`,
		`ops.Errors: n < 0 -> n <= 0 | "<" => "<="`,
		`ops.Step: i < n -> i <= n | "<" => "<="`,
		`ops.positive.func1: x > 0 -> x >= 0 | ">" => ">="`,
		`ops.init.3: limit > 5 -> limit >= 5 | ">" => ">="`,
	})
}

// Contract: mutate/M4
func TestEquality(t *testing.T) {
	sameSet(t, "equality", changes(t, "equality"), []string{
		`ops.Compare: a == b -> a != b | "==" => "!="`,
		`ops.Compare: a != 0 -> a == 0 | "!=" => "=="`,
		`ops.Order: a == b -> a != b | "==" => "!="`,
		`ops.Present: p != nil -> p == nil | "!=" => "=="`,
		`ops.Errors: n == 0 -> n != 0 | "==" => "!="`,
	})
}

// Contract: mutate/M4
func TestLogical(t *testing.T) {
	sameSet(t, "logical", changes(t, "logical"), []string{
		`ops.Compare: a < b || a <= 0 -> a < b && a <= 0 | "||" => "&&"`,
		`ops.Compare: a == b && a != 0 -> a == b || a != 0 | "&&" => "||"`,
		`ops.Compare: !(a > b) && a >= 1 -> !(a > b) || a >= 1 | "&&" => "||"`,
		`ops.Order: a == b || a < b -> a == b && a < b | "||" => "&&"`,
		`ops.Twice: a && b -> a || b | "&&" => "||"`,
		`ops.Not: !ok && !(done) -> !ok || !(done) | "&&" => "||"`,
	})
}

// Contract: mutate/M4
func TestNegation(t *testing.T) {
	sameSet(t, "negation", changes(t, "negation"), []string{
		`ops.Compare: !(a > b) -> (a > b) | "!" => ""`,
		`ops.Not: !ok -> ok | "!" => ""`,
		`ops.Not: !(done) -> (done) | "!" => ""`,
	})
}

// Only the predeclared true and false change. A field or a local that happens to be called true is
// something else.
//
// Contract: mutate/M4
func TestBool(t *testing.T) {
	sameSet(t, "bool", changes(t, "bool"), []string{
		`ops.Off: false -> true | "false" => "true"`,
		`ops.BoxedFalse: false -> true | "false" => "true"`,
		`ops.Yes: true -> false | "true" => "false"`,
		`ops.Flags: true -> false | "true" => "false"`,
		`ops.Flags: false -> true | "false" => "true"`,
		`ops.Make: false -> true | "false" => "true"`,
		`ops.nested.func2: true -> false | "true" => "false"`,
		`ops.plain.Same: true -> false | "true" => "false"`,
	})
}

// Contract: mutate/M4
func TestStep(t *testing.T) {
	sameSet(t, "step", changes(t, "step"), []string{
		`ops.Now.func1: *n++ -> *n-- | "++" => "--"`,
		`ops.Sum: s += x -> s -= x | "+=" => "-="`,
		`ops.Step: i++ -> i-- | "++" => "--"`,
		`ops.Step: total += i -> total -= i | "+=" => "-="`,
		`ops.Step: n-- -> n++ | "--" => "++"`,
		`ops.Step: total -= n -> total += n | "-=" => "+="`,
		`ops.init.1: limit++ -> limit-- | "++" => "--"`,
		`ops.init.2: count-- -> count++ | "--" => "++"`,
		`ops.plain.Bump: p.n++ -> p.n-- | "++" => "--"`,
	})
}

// A result written across lines keeps its newlines, and one whose plain removal would strand an
// import or a local keeps its uses in a branch that never runs.
//
// Contract: mutate/M4
func TestDropError(t *testing.T) {
	sameSet(t, "drop-error", changes(t, "drop-error"), []string{
		`ops.Check: return errors.New("too big") -> return nil | "errors.New(\"too big\")" => "func() error { if false { _ = errors.New(\"too big\") }; return nil }()"`,
		`ops.Two: return 0, errors.New("x") -> return 0, nil | "errors.New(\"x\")" => "func() error { if false { _ = errors.New(\"x\") }; return nil }()"`,
		`ops.Errors: return 0, errBad -> return 0, nil | "errBad" => "nil"`,
		`ops.Errors: return 0, fmt.Errorf("zero %d", n) -> return 0, nil | "fmt.Errorf(\"zero %d\",\n\t\t\tn)" => "(\nnil)"`,
		`ops.Wrap: return errors.New( msg) -> return nil | "errors.New(\n\t\tmsg)" => "func() error { if false { _ = errors.New(\n\t\tmsg) }; return nil }()"`,
		`ops.Later.func1: return errBad -> return nil | "errBad" => "nil"`,
	})
}

// A removed call keeps its newlines, and one whose plain removal would strand an import or a local
// keeps its uses in a branch that never runs.
//
// Contract: mutate/M4
func TestDropCall(t *testing.T) {
	sameSet(t, "drop-call", changes(t, "drop-call"), []string{
		`ops.Calls: fmt.Fprintln(w, "h") -> (removed) | "fmt.Fprintln(w, \"h\")" => ""`,
		`ops.Calls: fmt.Fprintf(w, "%d", 1) -> (removed) | "fmt.Fprintf(w,\n\t\t\"%d\", 1)" => "\n"`,
		`ops.Calls: err.Error() -> (removed) | "err.Error()" => ""`,
		`ops.Calls: close(ch) -> (removed) | "close(ch)" => ""`,
		`ops.Strand: fmt.Fprint(w, x) -> (removed) | "fmt.Fprint(w, x)" => "if false { fmt.Fprint(w, x) }"`,
		`ops.Now: func() { *n++ }() -> (removed) | "func() { *n++ }()" => ""`,
		`ops.Quit: os.Exit(code) -> (removed) | "os.Exit(code)" => "if false { os.Exit(code) }"`,
	})
}

// Contract: mutate/M5
func TestArithmeticLeavesStringConcatenationAlone(t *testing.T) {
	ms, unviable := generate(t, opsPath, []string{"arithmetic"})
	for _, m := range ms {
		switch m.Top {
		case "Greet", "Tag", "Join", "Plus":
			t.Errorf("%s: arithmetic changed a + that may concatenate strings", m.ID)
		}
	}
	// Mixed's numbers still change, and only they do.
	var mixed []string
	for _, m := range ms {
		if m.Top == "Mixed" {
			mixed = append(mixed, m.ID)
		}
	}
	if want := []string{"ops.Mixed: n+1 -> n-1"}; !slices.Equal(mixed, want) {
		t.Errorf("Mixed got %q, want %q", mixed, want)
	}
	// Type parameters that hold only numbers count as numbers.
	for _, id := range []string{"ops.Double: x * 2 -> x / 2", "ops.Halve: x / 2 -> x * 2"} {
		find(t, ms, id)
	}
	steps, _ := generate(t, opsPath, []string{"step"})
	find(t, steps, "ops.Sum: s += x -> s -= x")
	for _, m := range steps {
		if m.Top == "Greet" {
			t.Errorf("%s: step changed a += on a string", m.ID)
		}
	}
	// A concatenation turned into a subtraction would only fail to type-check, so none was tried:
	// the two unviable arithmetic mutants are Small's and Scale's.
	if unviable != 2 {
		t.Errorf("%d arithmetic mutants were unviable, want 2", unviable)
	}
}

// Contract: mutate/M5
func TestDropErrorReplacesOnlyErrors(t *testing.T) {
	ms, _ := generate(t, opsPath, []string{"drop-error"})
	for _, m := range ms {
		switch m.Top {
		case "Concrete", "Boxing", "Pass", "NilErr", "Blank", "myErr.Error":
			t.Errorf("%s: drop-error replaced something that isn't an expression of type error", m.ID)
		}
		if strings.HasSuffix(m.Original, ", nil") || m.Original == "return nil" {
			t.Errorf("%s: drop-error replaced a nil", m.ID)
		}
	}
	find(t, ms, "ops.Errors: return 0, errBad -> return 0, nil")
	find(t, ms, "ops.Later.func1: return errBad -> return nil")
}

// Contract: mutate/M5
func TestDropCallLeavesLoggingAlone(t *testing.T) {
	ms, _ := generate(t, opsPath, []string{"drop-call"})
	var calls []string
	for _, m := range ms {
		if m.Top == "Calls" {
			calls = append(calls, m.Original)
		}
	}
	// log.Printf, a *log.Logger, slog.Info, a *slog.Logger, a method promoted from an embedded
	// *log.Logger, a parenthesized log.Println and a chained slog call all stay.
	want := []string{`fmt.Fprintln(w, "h")`, `fmt.Fprintf(w, "%d", 1)`, "err.Error()", "close(ch)"}
	if !slices.Equal(calls, want) {
		t.Errorf("drop-call removed %q from Calls, want %q", calls, want)
	}
}

// A field or a local that happens to be called true is something else.
//
// Contract: mutate/M5
func TestBoolChangesOnlyThePredeclaredConstants(t *testing.T) {
	ms, _ := generate(t, opsPath, []string{"bool"})
	for _, m := range ms {
		if m.Top == "Odd" || m.Top == "Shadow" {
			t.Errorf("%s: bool changed something that isn't the predeclared true or false", m.ID)
		}
	}
	// Make's literal names a field called true and gives it the predeclared false. Only the false
	// changes.
	var made []string
	for _, m := range ms {
		if m.Top == "Make" {
			made = append(made, fmt.Sprintf("%s at %q", m.ID, source(t, m.File)[m.Start-len("true: "):m.End]))
		}
	}
	if want := []string{`ops.Make: false -> true at "true: false"`}; !slices.Equal(made, want) {
		t.Errorf("Make got %q, want %q", made, want)
	}
}

// Contract: mutate/M8
func TestFallbacksKeepRemovedCodeInABranchThatNeverRuns(t *testing.T) {
	ms, _ := generate(t, opsPath, nil)
	for _, c := range []struct{ id, text string }{
		// Strand's call is the only use of x, and Quit's the only use of os in its file.
		{"ops.Strand: fmt.Fprint(w, x) -> (removed)", "if false { fmt.Fprint(w, x) }"},
		{"ops.Quit: os.Exit(code) -> (removed)", "if false { os.Exit(code) }"},
		// Check's and Two's errors are their files' only use of errors, and Wrap's the only use of msg.
		{`ops.Check: return errors.New("too big") -> return nil`, `func() error { if false { _ = errors.New("too big") }; return nil }()`},
		{`ops.Two: return 0, errors.New("x") -> return 0, nil`, `func() error { if false { _ = errors.New("x") }; return nil }()`},
		{"ops.Wrap: return errors.New( msg) -> return nil", "func() error { if false { _ = errors.New(\n\t\tmsg) }; return nil }()"},
		// Nothing is stranded here, so the plain forms stay.
		{`ops.Calls: fmt.Fprintln(w, "h") -> (removed)`, ""},
		{`ops.Calls: fmt.Fprintf(w, "%d", 1) -> (removed)`, "\n"},
		{"ops.Errors: return 0, errBad -> return 0, nil", "nil"},
		{`ops.Errors: return 0, fmt.Errorf("zero %d", n) -> return 0, nil`, "(\nnil)"},
	} {
		if m := find(t, ms, c.id); m.Text != c.text {
			t.Errorf("%s: Text = %q, want %q", c.id, m.Text, c.text)
		}
	}
}
