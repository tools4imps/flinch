# mutate's declared mutants

## Ties in the order that never happen

A strict comparison between two file names or two starts runs only when they differ. Two changes start at the same place only when an erase mutant meets a call at the very start of a one-line body, and the erase mark tells those apart, so the comparisons after it never run.

```equivalent
internal/mutate.Generate.func1: p.Names[a.file] < p.Names[b.file] -> p.Names[a.file] <= p.Names[b.file]
internal/mutate.Generate.func1: a.start < b.start -> a.start <= b.start
internal/mutate.Generate.func1: a.op != b.op -> a.op == b.op
internal/mutate.Generate.func1: a.op < b.op -> a.op <= b.op
internal/mutate.Generate.func1: a.end != b.end -> a.end == b.end
internal/mutate.Generate.func1: a.end < b.end -> a.end <= b.end
internal/mutate.Generate.func1: a.text < b.text -> a.text <= b.text
```

## Both operands of an arithmetic operator have one type

Go needs both operands of `+`, `-`, `*`, `/` and `%` to have the same type once untyped constants are converted, so one operand is numeric exactly when the other is.

```equivalent
internal/mutate.gen.binary: g.numeric(x.X) && g.numeric(x.Y) -> g.numeric(x.X) || g.numeric(x.Y)
```

## nil is never of type error

go/types gives a returned `nil` the type `untyped nil`, which is never identical to `error`, so the type check skips it whether or not the nil check runs first.

```equivalent
internal/mutate.gen.dropError: !ok || tv.IsNil() -> !ok && tv.IsNil()
```

## Named results stop the loop before any zero value

A result list names all of its results or none. The loop stops at the first named one before it collects a zero value, so the erase mutant is a bare return with or without the flag.

```equivalent
internal/mutate.gen.erase: true -> false
```

## Conditions the type checker never leaves open

go/types records a `*types.Func` for every function declaration's name, blank ones included. Every operand and union term `numeric` sees has a type. Every constraint, and every interface embedded in one, has an interface underneath. Every union in a constraint has at least one term.

```equivalent
internal/mutate.gen.zeroBody: false -> true #3
internal/mutate.numeric: false -> true
internal/mutate.numericConstraint: false -> true
internal/mutate.numericConstraint: e.Len() > 0 -> e.Len() >= 0
```

## A type's source starts with a token

`prevEnd` is -1 before the first token and at least 1 after it, so it is never 0.

```equivalent
internal/mutate.oneLine: prevEnd >= 0 -> prevEnd > 0
```

## What go list reports

go list writes export data for every package a loaded package imports, apart from `unsafe` and `C`, which the importer never looks up. Without `-e` it leaves erroneous packages out of its output and exits with an error, so a package it prints never carries one, and output from a run that succeeded always decodes. It treats `-tags` with an empty list like no `-tags` at all. Every package in a module comes with a Go version, 1.16 when go.mod has no `go` line. go list ran in the root and listed each directory, so both exist when Load resolves their symlinks.

```equivalent
internal/typecheck.Load.func1: return nil, fmt.Errorf("go list reported no export data for %s", path) -> return nil, nil
internal/typecheck.list: return nil, fmt.Errorf("reading go list output: %w", err) -> return nil, nil
internal/typecheck.list: return nil, fmt.Errorf("go list: %s: %s", p.ImportPath, p.Error.Err) -> return nil, nil
internal/typecheck.list: len(tags) > 0 -> len(tags) >= 0
internal/typecheck.Loader.load: lp.Module != nil && lp.Module.GoVersion != "" -> lp.Module != nil || lp.Module.GoVersion != ""
internal/typecheck.relDir: err1 == nil && err2 == nil -> err1 == nil || err2 == nil
```

## Source that changes while flinch reads it

go list has just built every file Load reads, parses and type-checks. A file that vanishes or changes in between is a race the Contract leaves open.

```unpromised
internal/typecheck.Loader.load: return nil, err -> return nil, nil
internal/typecheck.Loader.load: return nil, err -> return nil, nil #2
internal/typecheck.Loader.load: return nil, fmt.Errorf("type-checking %s: %w", lp.ImportPath, err) -> return nil, nil
```

## Which error Load reports for an argument that isn't an import path

Load fails on a pattern or a relative path either way. Whether it type-checks the packages such an argument matches before it fails, and so which error it reports first, is open.

```unpromised
internal/typecheck.Load: lp.DepOnly || !requested[lp.ImportPath] -> lp.DepOnly && !requested[lp.ImportPath]
```

## Memory

Viable drops each mutated file from the file set once it's checked, so a long run's memory stays flat. The Contract promises nothing about memory, and nothing reads the dropped entries.

```unpromised
internal/typecheck.Loader.Viable: f != nil -> f == nil
internal/typecheck.Loader.Viable: tf != nil -> tf == nil
```

## Two units with one span

units.Of never names two units with the same span, so which of two such units Innermost returns is open.

```unpromised
internal/units.Innermost: u.End-u.Pos < best.End-best.Pos -> u.End-u.Pos <= best.End-best.Pos
```
