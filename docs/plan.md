# flinch v0.1.0 build plan

This is the plan v0.1.0 was built from. The obligations below were the starting point; the Contract
in `contract/*/README.md` is what flinch is held to now, and it grew as the suites were written.

## v0.1.0 scope

Ships: the Contract loader and `flinch contract`; covers; the ten default operators with erase first;
mutant ids and hashes; `equivalent` and `unpromised` declarations with the unpromised wildcard; the
runner (overlay builds, test2json, per-test coverage, crash and timeout recovery, batch clean runs);
the matrix (holds, sole holds, hollow, blind, kill kinds, held only from outside); the gate; `--since`,
`--only`, `--dry-run`, `--tags`, `--jobs`, `--operators`, `--timeout-coefficient`, `--contract`,
`--format`, `--output`; text and JSON reports.

Deferred: the result cache, `--witness`, `flinch explain`, the eight opt-in operators, redundant-test
reporting, both GOFLAGS hand-offs (tests that build binaries themselves don't see mutants or report
coverage), type-checking from source, the `flinch` settings block in mutants.md, a marketplace Action.

Decided: unreached code fails the build; no advisory mode; a Contract test names only its own
primitive's obligations; tags sit on top-level functions; drop-call skips calls into `log` and
`log/slog`; the summary leads with sole holds and splits declarations into equivalent, unpromised and
wildcard.

## Conventions

- Standard library only. Go 1.25 is the oldest supported release.
- Comments read like qualm's: full sentences, plain words, saying why. No em dashes anywhere.
- Paths inside flinch are slash-separated and relative to the module root unless a name says `abs`.
- Every package has unit tests next to it (white-box is fine). Contract tests come later, in `contract/`.
- `gofmt` clean and `go vet` clean.
- Errors that mean "flinch couldn't decide" (exit 2) are returned as errors. Problems in the Contract's
  files are `problem.Problem` values (exit 1).

## Shared packages (written, don't change without saying so)

- `internal/problem`: `Problem{Path, Line, Message}`, `Sort`.
- `internal/mutantid`: `ID`, `Hash`, `Prefix`, `Collapse`, `Parse` for declaration lines.
- `internal/units`: unit naming (`Of`, `Innermost`, `Find`, `ReceiverType`).
- `internal/model`: `Mutant`, `Test`, `TestRef`, `Kill`, `KillKind`, `Row`, `Status`.

## Packages and their APIs

### Static (`flinch contract`)

`internal/source`: the module and its packages without a Go toolchain.

```go
type Module struct{ Root, Path string }            // Root is absolute
func FindModule(dir string) (Module, error)         // walks up to go.mod
type Config struct{ Tags []string }                 // GOOS and GOARCH come from go/build.Default
type Package struct {
	Dir, ImportPath, Name string
	GoFiles   []string // non-test files that build under the config, sorted
	TestFiles []string // _test.go files that build under the config, sorted
	Mutable   []string // GoFiles minus generated files and files that import "C"
}
func Packages(m Module, cfg Config) ([]Package, error) // skips testdata, vendor, names starting with . or _, nested modules
type Parsed struct {
	Fset  *token.FileSet
	Files []*ast.File
	Names []string    // module-relative, same order as Files
	Units []units.Unit
	Types []string    // package-level type names
}
func Parse(m Module, p Package) (*Parsed, error) // GoFiles only
```

`internal/contract`: the loader.

```go
type Contract struct{ Dir string; Primitives []*Primitive } // sorted by name
type Primitive struct {
	Name, Dir    string
	Readme       string // path of README.md
	Mutants      string // path of mutants.md, "" when absent
	Obligations  []Obligation
	Covers       []Ref
	Declarations []Declaration
}
type Obligation struct{ ID string; Line int } // ID like "K1"; full id is Name + "/" + ID
type Ref struct{ Text string; Line int }
type Declaration struct {
	Kind    string // "equivalent" or "unpromised"
	Text    string // the line, trimmed
	Path    string
	Line    int
	Reason  string // nearest heading above the block plus the prose between; "" when no heading
}
func Load(root, dir string) (*Contract, []problem.Problem, error)
```

`internal/tags`: `func Scan(m source.Module, c *contract.Contract, pkgs []source.Package) ([]model.Test, []problem.Problem)`

`internal/covers`:

```go
type Ownership struct{ /* unexported */ }
func Resolve(c *contract.Contract, pkgs []source.Package, parsed map[string]*source.Parsed) (*Ownership, []problem.Problem)
func (o *Ownership) Owner(dir, top string) (primitive string, ok bool) // top is a top-level unit name
func (o *Ownership) Covered() []string                                  // package dirs with any covered unit
func (o *Ownership) Outside() []string                                  // package dirs with none, the Contract's own dirs left out
```

`internal/declare`:

```go
func Check(c *contract.Contract, own *covers.Ownership, parsed map[string]*source.Parsed) []problem.Problem
type Index struct{ /* unexported */ }
func NewIndex(c *contract.Contract) *Index
func (ix *Index) Exact(id string) *contract.Declaration
func (ix *Index) Wildcard(dir, unit string) *contract.Declaration // a wildcard on unit, or on the top-level unit holding it
func (ix *Index) All() []*contract.Declaration
```

`internal/engine` (static half): `func Check(o Options) (*Static, error)` with
`Options{Dir, Contract string; Tags []string}` and `Static{Module, Packages, Parsed, Contract, Tests,
Ownership, Declarations, Problems}`. Problems are sorted.

### Mutation

`internal/typecheck`:

```go
type Package struct {
	Dir, ImportPath string
	Fset  *token.FileSet
	Files []*ast.File
	Names []string          // module-relative, sorted
	Src   map[string][]byte // by name
	Types *types.Package
	Info  *types.Info
}
type Loader struct{ /* unexported */ }
func Load(ctx context.Context, root string, tags []string, importPaths []string) (*Loader, map[string]*Package, error)
func (l *Loader) Viable(p *Package, name string, src []byte) bool
```

`internal/mutate`:

```go
var Defaults = []string{"erase", "arithmetic", "boundary", "equality", "logical", "negation", "bool", "step", "drop-error", "drop-call"}
func Generate(l *typecheck.Loader, p *typecheck.Package, mutable map[string]bool,
	owner func(top string) (primitive string, ok bool), ops []string) (ms []model.Mutant, unviable int)
```

### Running

`internal/runner`:

```go
type Options struct {
	Root        string // module root, absolute
	Tags        []string
	Jobs        int
	Coefficient float64 // default 10
	Progress    io.Writer
	Work        string // a temp directory the runner owns
}
type Suite struct {
	Primitive, Dir, ImportPath string
	Tests []string // top-level Test, Example and Fuzz names
}
type Undecided struct{ Reason string } // an error: exit 2
func Prove(ctx context.Context, o Options, suites []Suite, coverpkg []string) (*Baseline, error)
func (b *Baseline) Reach(file string, line, col int) []model.TestRef
func (b *Baseline) Linking(importPath string) []model.TestRef
type Job struct{ Mutant model.Mutant; Tests []model.TestRef }
func (b *Baseline) Run(ctx context.Context, jobs []Job) (map[string]model.Row, error) // keyed by mutant hash
```

### Verdict

`internal/since`: `Changed(ctx, root, ref string) (*Changes, error)`, `(*Changes).Touches(file string, line int) bool`,
`(*Changes).TouchesDir(dir string) bool`.

`internal/matrix`: `Compute(in Input) *Result`, where `Input` carries the Contract, tests, mutants,
rows, reach, declarations and the run's scope, and `Result` carries per-mutant, per-obligation and
per-test results plus broken declarations.

`internal/gate`: `Decide(...) Verdict` with the exit code.

`internal/report`: `Text` and `JSON` over a `Report` value.

### Wiring

`internal/engine.Run` and `internal/cli.Main` join the stages. `cmd/flinch` wraps `cli.Main`.

## The v0.1.0 Contract (74 obligations)

Each section becomes `contract/<primitive>/README.md`, with a `covers` block naming the package.

contract (covers `internal/contract`)
- C1 The Contract is the `contract/` directory at the module root, or the one `--contract` names. Each direct subdirectory holding a README.md is a primitive named after the directory. flinch skips `testdata`, `vendor` and names starting with `.` or `_`, and a subdirectory without a README.md holds helpers.
- C2 An obligation is a README.md line that opens with `- **`, uppercase letters, digits and `**`. Its full id is `<primitive>/<id>`, and two obligations with the same full id are an error.
- C3 Fences follow CommonMark: three or more backticks or tildes, closed by the same character at least as many times. A fence inside an HTML comment is dead. The info string is the first word after the fence.
- C4 `covers` blocks count only in README.md, and `equivalent` and `unpromised` blocks count only in mutants.md. One of those blocks anywhere else is an error that names the file it belongs in.
- C5 A symlink inside the Contract, a file that isn't UTF-8, and a Go file in a primitive's directory whose name doesn't end in `_test.go` are errors.
- C6 Every Contract error prints as `path:line: message`. A run reports all of them at once, sorted by path and line.

tags (covers `internal/tags`)
- T1 A Contract test is a top-level `Test`, `Example` or `Fuzz` function in a `_test.go` file in a primitive's directory. `TestMain` isn't one.
- T2 A tag is a line comment `// Contract: <primitive>/<id>` in the comment group directly above a Contract test. A test can carry several.
- T3 Every obligation is named by at least one Contract test.
- T4 Every Contract test names at least one obligation, and only obligations of its own primitive.
- T5 A tag naming an obligation the Contract doesn't have is an error.
- T6 A tag in a `_test.go` file outside the Contract is an error.
- T7 `flinch contract` runs with no Go toolchain on PATH and reports every Contract error that a full run would report.
- T8 A tagged `Example` with no `// Output:` comment is an error, because `go test` never runs it.

covers (covers `internal/covers`)
- V1 A `covers` line `dir` names the package in that directory, relative to the module root. `dir/...` adds every package below it.
- V2 `dir.Name` names a function, or a type with all its methods. `dir.Type.Method` names one method, with the receiver written without `*`.
- V3 Each function, method and package-level variable belongs to the primitive with the most specific matching reference: a method first, then a function or type, then a package, then `...`. A tie goes to the primitive that sorts first by name.
- V4 A reference that names nothing in the module is an error.
- V5 `_test.go` files, files marked `// Code generated ... DO NOT EDIT.` and files that import `"C"` are never covered.
- V6 Module packages no reference covers are listed as outside the Contract, apart from the Contract's own directories. They never fail the build.

mutate (covers `internal/mutate`)
- M1 Only covered code is mutated.
- M2 Each mutant changes one expression or statement. The change is spliced into the file's bytes, so every other line keeps its number and its text.
- M3 A mutant that fails to type-check is dropped as unviable before any build.
- M4 The default operators are erase, arithmetic, boundary, equality, logical, negation, bool, step, drop-error and drop-call. `--operators` narrows a run to some of them.
- M5 Operators consult types. `arithmetic` leaves string concatenation alone, `drop-error` replaces only expressions of type `error`, and `drop-call` leaves calls into `log` and `log/slog` alone.
- M6 The same commit, Contract, operators and Go version give the same mutants in the same order.
- M7 Each covered function gets one erase mutant, which returns zero values from the function's first line and runs before its other mutants. When it lives, the function's other mutants are skipped and the function is reported once.

identity (covers `internal/mutantid`)
- I1 A mutant's id is `<dir>.<unit>: <original> -> <replacement>`. When the same change appears n times in a unit, the second and later ones end with a space and `#n`.
- I2 A unit is a function, `Type.Method`, `Func.funcN` for the N-th function literal inside `Func`, a package-level variable's name, or `init.N` for a package's init functions, numbered from 0 the way the compiler numbers them.
- I3 `<original>` and `<replacement>` are source text with each run of whitespace collapsed to one space.
- I4 An edit outside a unit never changes the ids inside it.
- I5 JSON and the text report carry each id's SHA-256, cut to its first 12 hex digits.

declare (covers `internal/declare`)
- D1 An `equivalent` block lists mutant ids, one per line, and claims no program can tell them from the original. A wildcard in it is an error.
- D2 An `unpromised` block lists ids or `<dir>.<unit>: *`, and says the Contract leaves that behavior open. A wildcard covers only the unheld mutants of the unit and the closures inside it.
- D3 A declaration's reason is the nearest heading above its block plus the prose between them. A block with no heading above it is an error.
- D4 A declaration belongs in the mutants.md of the primitive covering its unit. One anywhere else is an error that names the right file.
- D5 A declared mutant that a Contract test reaches still runs. If a Contract test kills it, the declaration is an error.
- D6 A declaration that matches no mutant, in a run that mutated its whole unit, is stale and an error. A wildcard is stale when its unit has no unheld mutants.
- D7 A declaration naming a unit that no longer exists is an error in every run, scoped or not.

run (covers `internal/runner`)
- R1 Before mutating, flinch runs the Contract suite whole and then each Contract test alone. A test that fails either time stops the run with exit 2, and the report names it.
- R2 The lone runs record each test's coverage blocks. A mutant's covering tests are the Contract tests whose lone run reached its block.
- R3 A mutant in a package-level variable's initializer or an `init` function is reached by every Contract test whose binary links its package.
- R4 A mutant no Contract test reaches is unreached and is never built.
- R5 Each distinct batch of covering tests runs once against the clean binary before any mutant uses it. A test that fails there stops the run with exit 2.
- R6 A reached mutant is compiled through `-overlay`. The working tree, the git index and every source file are unchanged after a run, including an interrupted one.
- R7 Each test binary runs in its package's directory, the way `go test` runs it.
- R8 A mutant runs its whole batch without stopping at the first failure. Its row records every test and subtest that failed, and whether each failed on an assertion, a panic or a timeout.
- R9 When a panic or timeout ends the process, the tests that hadn't run start again in a fresh one. A timeout is pinned on the tests in Go's `running tests:` list, never on the event stream's last `Test` field.
- R10 A process that dies before its first test starts counts as a kill by every test in its batch. A crash that names no test reruns the batch one test per process, and after three restarts the mutant has no verdict.
- R11 The time budget is the sum of the batch's lone run times, times the coefficient (default 10), plus 2 seconds. A kill that came only from a timeout counts once a rerun with twice the budget times out as well.
- R12 A mutant that type-checked but won't build has no verdict, and any mutant without a verdict makes the run exit 2.

matrix (covers `internal/matrix`)
- X1 An obligation holds a mutant when one of its tests killed it. Its sole holds are the mutants no other obligation holds.
- X2 In a full run, an obligation that holds nothing is hollow.
- X3 In a full run, a Contract test that ran in at least one complete row of an undeclared mutant and killed nothing is blind. A test seen only in incomplete rows is never called blind.
- X4 A scoped run judges hollow obligations and blind tests only for primitives whose covered code it mutated whole.
- X5 An obligation whose every hold came from a panic or a timeout is reported as held only by crashes. That never fails the build.
- X6 A mutant in one primitive's code that only other primitives' tests kill is reported as held only from outside. That never fails the build.

gate (covers `internal/gate`)
- G1 The verdict reads only the commit, its Contract, the Go toolchain, GOOS, GOARCH and the build tags. The job count, the machine and git config never change it, and `--since` reads the base only to choose mutants.
- G2 Exit 0 means no Contract error, no unheld undeclared mutant and no broken declaration, plus no hollow obligation or blind test in a full run. Exit 1 means at least one of those. Exit 2 means flinch couldn't decide, and it wins over exit 1.
- G3 `--since REF` mutates the covered lines changed since the merge base with REF, plus all the code covered by any primitive whose Contract directory changed. Renames and git config never change which lines count as changed.
- G4 Contract errors are checked across the whole Contract in every run, scoped or not.
- G5 No mutation score or threshold enters the verdict.

report (covers `internal/report`)
- P1 The text report lists Contract errors first, then unheld mutants by primitive, file and line, each with its id, its hash and the obligations whose tests ran it.
- P2 Broken declarations, hollow obligations, obligations held only by crashes, mutants held only from outside, blind tests and packages outside the Contract follow, each list sorted by path and line.
- P3 Every finding in the text report says what would clear it.
- P4 JSON carries `schema_version`. Additive changes bump the minor version, and consumers ignore unknown keys.
- P5 JSON lists every mutant with its status, killers, kill kinds, covering tests and matching declaration. The same input gives the same bytes apart from the `timing` object.
- P6 Progress goes to stderr, and the report goes to stdout or the `--output` file.
- P7 The summary counts sole holds per obligation, splits declared mutants into equivalent, unpromised and wildcard, and prints the mutation score: killed mutants over killed plus unheld ones.

cli (covers `internal/cli` and `cmd/flinch`)
- L1 `flinch` and `flinch run` do the whole check. `flinch contract` stops after the Contract checks, and `flinch operators` and `flinch version` print what their names say.
- L2 An unknown flag or a bad flag value exits 2 with a usage message.
- L3 `--dry-run` lists every mutant a run would build, with its id, and builds none.
- L4 `--tags` passes its build tags to every go command flinch runs.
- L5 `--only` mutates only the named primitives' covered code, and the run counts as scoped.
