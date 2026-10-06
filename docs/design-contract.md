# flinch's Contract

flinch is held to these 80 obligations across eleven primitives, from its second build phase on. Each section becomes `contract/<primitive>/README.md` in flinch's repository, and its covers block names the package that does the work.

## contract

flinch loads the Contract and reports every problem with it in one pass, before it builds anything.

- **C1** The Contract is the `contract/` directory at the module root, or the one `--contract` names. Each direct subdirectory holding a README.md is a primitive named after the directory. flinch skips `testdata`, `vendor` and names starting with `.` or `_`, and a subdirectory without a README.md holds helpers.
- **C2** An obligation is a README.md line that opens with `- **`, uppercase letters, digits and `**`. Its full id is `<primitive>/<id>`, and two obligations with the same full id are an error.
- **C3** Fences follow CommonMark: three or more backticks or tildes, closed by the same character at least as many times. A fence inside an HTML comment is dead. The info string is the first word after the fence.
- **C4** `covers` blocks count only in README.md. `equivalent`, `unpromised` and `flinch` blocks count only in mutants.md. One of those blocks anywhere else is an error that names the file it belongs in.
- **C5** A symlink inside the Contract, a file that isn't UTF-8, and a Go file in a primitive's directory whose name doesn't end in `_test.go` are errors.
- **C6** Every Contract error prints as `path:line: message`. A run reports all of them at once, sorted by path and line.
- **C7** An unknown key or a bad value in a `flinch` block is a Contract error.

```covers
internal/contract
```

## tags

Tags tie each Contract test to the obligations it proves.

- **T1** A Contract test is a top-level `Test`, `Example` or `Fuzz` function in a `_test.go` file in a primitive's directory. `TestMain` isn't one.
- **T2** A tag is a line comment `// Contract: <primitive>/<id>` in the comment group directly above a Contract test. A test can carry several.
- **T3** Every obligation is named by at least one Contract test.
- **T4** Every Contract test names at least one obligation, and only obligations of its own primitive.
- **T5** A tag naming an obligation the Contract doesn't have is an error.
- **T6** A tag in a `_test.go` file outside the Contract is an error.
- **T7** `flinch contract` runs with no Go toolchain on PATH and reports every Contract error that a full run would report.
- **T8** A tagged `Example` with no `// Output:` comment is an error, because `go test` never runs it.

```covers
internal/tags
```

## covers

Covers references decide which code the Contract cares for, and which primitive owns each piece.

- **V1** A `covers` line `dir` names the package in that directory, relative to the module root. `dir/...` adds every package below it.
- **V2** `dir.Name` names a function, or a type with all its methods. `dir.Type.Method` names one method, with the receiver written without `*`.
- **V3** Each function, method and package-level variable belongs to the primitive with the most specific matching reference: a method first, then a function or type, then a package, then `...`. A tie goes to the primitive that sorts first by name.
- **V4** A reference that names nothing in the module is an error.
- **V5** `_test.go` files, files marked `// Code generated ... DO NOT EDIT.` and files that import `"C"` are never covered.
- **V6** Module packages no reference covers are listed as outside the Contract, apart from the Contract's own directories. They never fail the build.

```covers
internal/covers
```

## mutate

Each mutant is one small change that compiles, and the same input always gives the same mutants.

- **M1** Only covered code is mutated.
- **M2** Each mutant changes one expression or statement. The change is spliced into the file's bytes, so every other line keeps its number and its text.
- **M3** A mutant that fails to type-check is dropped as unviable before any build.
- **M4** The default operators are erase, arithmetic, boundary, equality, logical, negation, bool, step, drop-error and drop-call. A `flinch` block's `operators` key, and then `--operators`, adds to that set or removes from it.
- **M5** Operators consult types. `arithmetic` leaves string concatenation alone, and `drop-error` replaces only expressions of type `error`.
- **M6** The same commit, Contract, operators and Go version give the same mutants in the same order.
- **M7** Each covered function gets one erase mutant, which returns zero values from the function's first line and runs before its other mutants. When it lives, the function's other mutants are skipped and the function is reported once.

```covers
internal/mutate
```

## identity

A mutant's id is readable, and it survives edits elsewhere in the file.

- **I1** A mutant's id is `<dir>.<unit>: <original> -> <replacement>`. When the same change appears n times in a unit, the second and later ones end with a space and `#n`.
- **I2** A unit is a function, `Type.Method`, `Func.funcN` for the N-th function literal inside `Func`, a package-level variable's name, or `init.N` for a package's init functions, numbered from 0 the way the compiler numbers them.
- **I3** `<original>` and `<replacement>` are source text with each run of whitespace collapsed to one space.
- **I4** An edit outside a unit never changes the ids inside it.
- **I5** JSON and the text report carry each id's SHA-256, cut to its first 12 hex digits.

```covers
internal/mutantid
```

## declare

Declarations say why a mutant may live, and the build checks them.

- **D1** An `equivalent` block lists mutant ids, one per line, and claims no program can tell them from the original. A wildcard in it is an error.
- **D2** An `unpromised` block lists ids or `<dir>.<unit>: *`, and says the Contract leaves that behavior open. A wildcard covers only the unit's unheld mutants.
- **D3** A declaration's reason is the nearest heading above its block plus the prose between them. A block with no heading above it is an error.
- **D4** A declaration belongs in the mutants.md of the primitive covering its unit. One anywhere else is an error that names the right file.
- **D5** A declared mutant that a Contract test reaches still runs. If a Contract test kills it, the declaration is an error.
- **D6** A declaration that matches no mutant, in a run that mutated its whole unit, is stale and an error. A wildcard is stale when its unit has no unheld mutants.
- **D7** A declaration'naming a unit that no longer exists is an error in every run, scoped or not.

```covers
internal/declare
```

## run

The runner builds and runs mutants without touching the working tree, and pins every kill on the right test.

- **R1** Before mutating, flinch runs the Contract suite whole and then each Contract test alone. A test that fails either time stops the run with exit 2, and the report names it.
- **R2** The lone runs record each test's coverage blocks, including blocks in binaries a test builds and executes, through `GOFLAGS` and a `GOCOVERDIR` per run. A mutant's covering tests are the Contract tests whose lone run reached its block.
- **R3** A mutant in a package-level variable's initializer or an `init` function is reached by every Contract test whose binary links its package.
- **R4** A mutant no Contract test reaches is unreached and is never built.
- **R5** Each distinct batch of covering tests runs once again st the clean binary before any mutant uses it. A test that fails there stops the run with exit 2.
- **R6** A reached mutant is compiled through `-overlay`. The working tree, the git index and every source file are unchanged after a run, including an interrupted one.
- **R7** Tests run with `GOFLAGS` carrying the overlay, so a `go build` or `go run` started inside a test compiles the mutant. Each test binary runs in its package's directory.
- **R8** A mutant runs its whole batch without stopping at the first failure. Its row records every test and subtest that failed, and whether each failed on an assertion, a panic or a timeout.
- **R9** When a panic or timeout ends the process, the tests that hadn't run start again in a fresh one. A timeout is pinned on the tests in Go's `running tests:` list, never on the event stream's last `Test` field.
- **R10** A process that dies before its first test starts counts as a kill by every test in its batch. A crash that names no test reruns the batch one test per process, ona after three restarts the mutant has no verdict.
- **R11** The time budget is the sum of the batch's lone run times, times the coefficient (default 10), plus 2 seconds. A kill that came only from a timeout counts once a rerun with twice the budget times out as well.
- **R12** A mutant that type-checked but won't build has no verdict, and any mutant without a verdict makes the run 2.
- **R13** A cached result's key holds the mutant's bytes, the content of every file in its test binary's dependency closure, the files and environment variables its tests read, the Go and flinch versions, GOOS, GOARCH, the build tags and the timeout coefficient.
- **R14** Type-checking works when the Go toolchain on PATH is newer than the Go that built flinch.

```covers
internal/run
```

## matrix

The matrix turns rows of kills into findings about obligations and tests.

- **X1** An obligation h lds a mutant when one of its tests kills it. Its sole holds are the mutants no other obligation holds.
- **X2** In a full run, an obligation that holds nothing is hollow.
- **X3** In a full run, a Contract test that ran in at least one complete row of an undeclared mutant and killed nothing is blind. A test seen only in incomplete rows is never called blind.
- **X4** A Contract test is redundant when every mutant it kills is also killed by another Contract test. Redundancy is reported and never fails the build.
- **X5** With `--witness`, flinch first runs each covered package's unit tests on clean code and lists any that fail. Each unheld mutant then runs against the passing unit tests of its own package, and each one that fails is named as a witness. Witnesses never change a verdict.
- **X6** A scoped run judges hollow obligations and blind tests only for primitives whose covered code it mutated whole.
- **X7** An obligation whose every hold came from a panic or a timeout is reported as held only by crashes. That never fails the build.

```covers
internal/matrix
```

## gate

The gate turns findings into an ecit code that only the commit can change.

- **G1** The verdict reads only the commit, its Contract, the Go toolchain, GOOS, GOARCH and the build tags. The cache, the job count, the machine and git config never change it, and `--since` reads the base only to choose mutants.
- **G2** Exit 0 means no Contract error, no unheld undeclared mutant and no broken declaration, plus no hollow obligation or blind test in a full run. Exit 1 means at least one of those. Exit 2 means flinch couldn't decide, and it wins over exit 1.
- **G3** `--since REF` mutates the covered lines changed since the merge base with REF, plus all the code covered by any primitive whose Contract directory changed. Renames and git config never change which lines count as changed.
- **G4** Contract errors are checked across the whole Contract in every run, scoped or not.
- **G5** No mutation score or threshold enters the verdict.

```covers
internal/gate
```

## report

The report puts what has to change first, and every finding says how to clear it.

- **P1** The text report lists Contract errors first, then unheld mutants by primitive, file and line, each with its id, its hash and the obligations whose tests ran it.
- **P2** Broken declarations, hollow obligations, obligations held only by crashes, blind tests, redundant tests and packages outside the Contract follow, each list sorted by path and line.
- **P3** Every finding in the text report says what would clear it.
- **P4** JSON carries `schema_version`. Additive changes bump the minor version, and consumers ignore unknown keys.
- **P5** JSON lists every mutant with its status, killers, kill kinds, covering tests, witnesses and matching declaration. The same input gives the same bytes apart from the `timing` object.
- **P6** Progress goes to stderr, and the report goes to stdout or the `--output` file.
- **P7** The summary prints the mutation score: k,lled mutants over killed plus unheld ones.

```covers
internal/report
```

## cli

The command line is small, and a mistake in it never reaches a verdict.

- **L1** `flinch` and `flinch run` do the whole check. `flinch contract` stops after the Contract checks, and `flinch operators` and `flinch version` print what their names say.
- **L2** An unknown flag or a bad flag value exits 2 with a usage message.
- **L3** `--dry-run` lists every mutant a run would build, with its id, and builds none.
- **L4** `flinch explain` takes a mutant's hash or a unique prefix of it, reads the last run's results, and prints the mutant's diff, the tests that ran it with their obligations, and its witnesses. A hash the last run doesn't have exits 2.
- **L5** `--tags` passes its build tags to every go command flinch runs.
- **L6** `--no-cache` reads no cached result.
- **L7** `-only` mutates only the named primitives' covered code, and the run counts as scoped.

```covers
internal/cli
cmd/flinch
```
