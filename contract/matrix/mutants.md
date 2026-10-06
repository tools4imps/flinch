# matrix mutants

## Sets whose values nothing reads

These maps are sets. The code ranges over their keys or counts them and never reads a value, so
storing false where it stores true changes nothing.

```equivalent
internal/matrix.fillKills: true -> false
internal/matrix.fillKills: true -> false #2
internal/matrix.fillKills: true -> false #3
internal/matrix.fillKills: true -> false #4
internal/matrix.obligations: true -> false
internal/matrix.obligations: true -> false #2
internal/matrix.testResults: true -> false
```

## Comparisons that run only when their sides differ

Each of these runs only after an earlier check found its two sides unequal, or, for test names,
when two references name the same test and sort the same either way. Less than and less than or
equal agree on every input they see.

```equivalent
internal/matrix.testResults.func1: a.File < b.File -> a.File <= b.File
internal/matrix.testResults.func1: a.Line < b.Line -> a.Line <= b.Line
internal/matrix.lessRef: a.Primitive < b.Primitive -> a.Primitive <= b.Primitive
internal/matrix.lessRef: a.Name < b.Name -> a.Name <= b.Name
```

## A killed mutant always has a kill

The matrix calls a mutant killed only when its row holds a kill, so where this check runs the
mutant has at least one.

```equivalent
internal/matrix.fillKills: len(m.Kills) > 0 -> len(m.Kills) >= 0
```

## A count compared only with zero

A wildcard's count of the mutants it covers is only compared with zero. Counting down leaves it
nonzero exactly when counting up does.

```equivalent
internal/matrix.declare: covered++ -> covered--
```

## The order of the matrix's own list of broken declarations

Both reports sort broken declarations by path and line before they print them. The Contract
promises the reports' order and leaves the order of the matrix's list open.

```unpromised
internal/matrix.Compute: problem.Sort(r.Broken) -> (removed)
```

## A declaration id that doesn't parse

The Contract loader rejects a declaration line that isn't a mutant id before any run reaches the
matrix, so what the matrix does with one is left open.

```unpromised
internal/matrix.exactWhole: false -> true
```

## An obligation id with nothing before its slash

Every obligation id starts with its primitive, a directory name. What the matrix makes of an id
with nothing there is left open.

```unpromised
internal/matrix.primitiveOf: i >= 0 -> i > 0
```

## The grammar of a broken declaration's message

The Contract promises that a broken declaration is reported and that its line says to delete it.
The grammar around the names of the tests that kill it is open.

```unpromised
internal/matrix.plural: { ... } -> { return *new(string) }
internal/matrix.plural: n == 1 -> n != 1
```
