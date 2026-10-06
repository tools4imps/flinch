# Declared mutants

## Positions that can't be equal

A comment can't start at the brace that opens a function's body, and it can't end where the closing brace ends. So `<` and `<=` give the same answer when hasOutput compares those positions, and so do `>` and `>=`.

```equivalent
internal/tags.hasOutput: cg.Pos() < body.Pos() -> cg.Pos() <= body.Pos()
internal/tags.hasOutput: cg.End() > body.End() -> cg.End() >= body.End()
```

## The order Scan returns Contract tests in

No tags obligation promises an order. Every list a person sees sorts the tests on its own: the matrix sorts them by file, line and name.

```unpromised
internal/tags.Scan: sort.SliceStable(s.tests, func(i, j int) bool { a, b := s.tests[i], s.tests[j] if a.Primitive != b.Primitive { return a.Primitive < b.Primitive } if a.File != b.File { return a.File < b.File } return a.Line < b.Line }) -> (removed)
internal/tags.Scan.func1: a.Primitive != b.Primitive -> a.Primitive == b.Primitive
internal/tags.Scan.func1: a.Primitive < b.Primitive -> a.Primitive <= b.Primitive
internal/tags.Scan.func1: a.File != b.File -> a.File == b.File
internal/tags.Scan.func1: a.File < b.File -> a.File <= b.File
internal/tags.Scan.func1: a.Line < b.Line -> a.Line <= b.Line
```

## The wording of an error

The Contract leaves the words of an error open. These change only a message: the example id in the hint for a malformed tag, and which of two messages a stray tag gets.

```unpromised
internal/tags.example: { ... } -> { return *new(string) }
internal/tags.scanner.elsewhere: dir == s.c.Dir -> dir != s.c.Dir
internal/tags.scanner.elsewhere: dir == s.c.Dir || strings.HasPrefix(dir, s.c.Dir+"/") -> dir == s.c.Dir && strings.HasPrefix(dir, s.c.Dir+"/")
```

## A test file flinch can't read

source lists a test file only after it has opened it, but the check for tags outside the Contract reads every `_test.go` file it finds, some of which the build never opens. The Contract leaves open what flinch does with one it can't read.

```unpromised
internal/tags.scanner.parse: false -> true
```
