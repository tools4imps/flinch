# report mutants

## How JSON is laid out

JSON readers see the same values whether the report is indented or not, and whether it escapes <,
> and & or writes them plainly. The Contract promises the values and leaves the layout open.

```unpromised
internal/report.JSON: enc.SetIndent("", " ") -> (removed)
internal/report.JSON: enc.SetEscapeHTML(false) -> (removed)
internal/report.JSON: false -> true
```

## Comparisons that run only when their sides differ

Each of these runs only after an earlier check found its two sides unequal, or on two equal ids,
which no run holds twice. Less than and less than or equal agree on every input they see.

```equivalent
internal/report.lessObligation: pa < pb -> pa <= pb
internal/report.natural: len(ta) < len(tb) -> len(ta) <= len(tb)
internal/report.natural: ta < tb -> ta <= tb
internal/report.natural: na < nb -> na <= nb
internal/report.natural: a[0] < b[0] -> a[0] <= b[0]
internal/report.natural: len(a) < len(b) -> len(a) <= len(b)
internal/report.lessMutant: a.File < b.File -> a.File <= b.File
internal/report.lessMutant: a.Line < b.Line -> a.Line <= b.Line
internal/report.lessMutant: a.Col < b.Col -> a.Col <= b.Col
internal/report.lessMutant: a.ID < b.ID -> a.ID <= b.ID
internal/report.lessMutant: a.Hash < b.Hash -> a.Hash <= b.Hash
internal/report.byPrimitive.func1: ms[i].Primitive < ms[j].Primitive -> ms[i].Primitive <= ms[j].Primitive
```

## An obligation id with nothing before its slash

Every obligation id starts with its primitive, a directory name. How the report splits an id with
nothing there is left open.

```unpromised
internal/report.splitID: i >= 0 -> i > 0
```
