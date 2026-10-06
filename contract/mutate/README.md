# mutate

mutate type-checks each covered package and finds its mutants: small changes spliced into the source, one expression or statement at a time.

## Obligations

- **M1** Only covered code is mutated.
- **M2** Each mutant changes one expression or statement. The change is spliced into the file's bytes, so every other line keeps its number and its text.
- **M3** A mutant that fails to type-check with the rest of its package, under the Go version its module names, is dropped as unviable before any build.
- **M4** The default operators are erase, arithmetic, boundary, equality, logical, negation, bool, step, drop-error and drop-call, and each makes the change the table below gives. `--operators` narrows a run to some of them.
- **M5** Operators consult types. `arithmetic` and `step` leave string concatenation alone, `bool` changes only the predeclared `true` and `false`, `drop-error` replaces only expressions of type `error`, and `drop-call` leaves calls into `log` and `log/slog` alone.
- **M6** The same commit, Contract, operators and Go version give the same mutants in the same order: by file name, then by where each change starts, with an erase mutant ahead of a change that starts at the same place.
- **M7** Each covered function gets one erase mutant, unless its body is empty or a single return of literal zero values, where only `nil` counts for a result of interface or type parameter type. A function literal gets none of its own. The erase mutant returns zero values from the function's first line and runs before the function's other mutants. When it lives, the function's other mutants are skipped and the function is reported once.
- **M8** When the plain form of a drop-call or drop-error mutant would leave an import or a variable unused, the mutant keeps the removed code in a branch that never runs, under the same id.
- **M9** Each covered package is type-checked from the export data `go list` builds, and named by its directory relative to the module root, even when a symlink spells that root another way. A covered package that can't be loaded stops the run instead of going unmutated.
- **M10** A mutant belongs to the innermost unit that holds its whole change, so removing a call to a function literal changes the function around the literal. A change in a package-level variable's initializer or an init function is marked as linked, since every test whose binary links the package runs it.

## Operators

| Operator | Change |
| --- | --- |
| erase | `return` at the top of the function's body, on the line of its `{`, with `*new(T)` for each result type, or bare when the results have names |
| arithmetic | `+` and `-` swap, `*` and `/` swap, and `%` becomes `*`, when the operands are numbers |
| boundary | `<` and `<=` swap, and `>` and `>=` swap |
| equality | `==` and `!=` swap |
| logical | `&&` and `\|\|` swap |
| negation | A `!` is dropped |
| bool | `true` and `false` swap |
| step | `++` and `--` swap, and `+=` and `-=` swap when the target is a number |
| drop-error | In a `return`, a result of type `error` other than `nil` becomes `nil` |
| drop-call | A call statement is removed |

```covers
internal/mutate
internal/typecheck
internal/units
```
