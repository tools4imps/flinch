# flinch design

Oct 5, 2026 · @Obie

flinch is a Go tool that holds code to its Contract by mutation. It breaks the code each primitive covers, one small change at a time, runs the black-box Contract suite against every broken copy, and records which obligation's tests noticed. A pull request can't merge while broken code passes the whole Contract suite, unless the Contract says why that's fine.

## What's new

flinch would be the first Go tool to tie a mutant to the obligation it breaks. The Go mutation testers report survivors and a score, and only mutrim says which test killed what. reqtrace links tests to written requirements and runs no mutants at all.

| Tool | What it answers | Kills per test | Tests tied to requirements |
| --- | --- | --- | --- |
| Gremlins | Which mutants survive in a package | No, it stops at the first failure | No |
| go-mutesting forks | The same, with PR scoping and quality gates in the jonbaldie fork | No | No |
| gomutant | The same, with overlay builds, per-test coverage routing and a cache | Not confirmed | No |
| mutrim | Which test kills each mutant, and which tests are blind or redundant. Bazel-based, pre-alpha | Yes | No |
| reqtrace | Which requirements have no test | Runs no mutants | Yes |
| flinch | Which obligation holds each line of covered code, and why the rest may break | Yes, grouped by obligation | Yes, and checks that the tagged tests catch something |

What flinch adds:

- **Kills count toward obligations.** Each kill is credited to the obligations whose tests failed. An obligation whose tests catch nothing shows up by name.
- **A surviving mutant is a Contract gap.** It points at a missing promise or a weak test. Sometimes it points at code nobody needs.
- **Whole functions get erased first.** One mutant per function returns zero values from its first line. When no Contract test notices, flinch reports the function once and skips its finer mutants.
- **Kills have a kind.** flinch records whether a test failed on an assertion, a panic or a timeout. An obligation held only by crashes holds its code by accident, and the report says so.
- **The gate has no percentage.** Every mutant in covered code is either killed or declared, the way exhale treats duplication.
- **Exceptions live in the Contract, with reasons, and get checked.** A declared mutant still runs when a Contract test reaches it. If a test kills it, the declaration is wrong and the build fails.
- **Unpromised behavior gets written down.** The Contract records where a regenerated implementation is free to differ.
- **Unit tests act as witnesses.** When a unit test catches a mutant the Contract suite misses, flinch names it as evidence of the missing obligation.
- **Agents probe the Contract, in a later phase.** An agent rewrites a function from its primitive's README while keeping every promise. A rewrite that passes the Contract suite and fails a unit test exposes a missing obligation with a realistic counterexample.

## At a glance

&#91;embedded content: one flinch run, from the Contract to the verdict\]

A killed mutant feeds the matrix down the left side. An unreached or lived mutant goes right, and fails the build unless mutants.md declares it with a reason.

## The Contract in Go

flinch reads the Contract layout exhale uses and puts each primitive's Contract suite in the same folder as its prose.

```text
contract/
  skip/
    README.md       obligations and a covers block
    skip_test.go    the skip Contract suite
    mutants.md      declared survivors, optional
  diff/
    README.md
    diff_test.go
    testdata/
  internal/         helpers for Contract tests; no README, so not a primitive
```

**Black-box by layout.** The suite lives in its own directory, so Go's visibility rules limit it to exported API, and no `export_test.go` can reach it. An agent can delete `internal/skip`, regenerate it from `contract/skip/`, and run the same suite against the new code. Go accepts a directory that holds only `_test.go` files, and `contract/` can import the module's `internal/` packages because it sits inside the module.

**Obligations** keep exhale's syntax: `- **K1** text` in README.md, with the full id `skip/K1`.

**covers references** name Go code. exhale's references name Ruby constants, and flinch's take four forms:

| Reference | Covers |
| --- | --- |
| `internal/skip` | The package in that directory, relative to the module root |
| `internal/check/...` | That package and every package below it |
| `internal/check.Keep` | One function, or a type with all its methods |
| `internal/check.judge.settled` | One method, with the receiver written without `*` |

As in exhale, the most specific reference wins and a tie goes to the primitive that sorts first by name. qualm's gate and judge primitives can then split `internal/check` by function. A reference that names nothing is a Contract error.

**Primitives** are the direct subdirectories of `contract/` that hold a README.md. Like the go command, flinch skips `testdata`, `vendor` and names starting with `.` or `_`, so the fixture Contracts in flinch's own tests never count as its Contract.

**Tags** keep the comment qualm and exhale already use. Every top-level `Test`, `Example` or `Fuzz` function in a primitive's directory is a Contract test, and it must carry at least one tag directly above it naming obligations of its own primitive. A tag in any `_test.go` file outside `contract/` is an error, because unit tests sit outside the Contract. So is a tagged `Example` with no `// Output:` comment, since `go test` compiles it and never runs it.

```go
package skip_test

import (
	"testing"

	"github.com/tools4imps/qualm/internal/skip"
)

// Contract: skip/K1
func TestBuiltDirectoriesAreSkippedAtAnyDepth(t *testing.T) {
	for _, path := range []string{"wendor/a.go", "x/node_modules/y.js", "a/b/dist/c.go"} {
		if skip.Rules{}.Reason(path) == "" {
			t.Errorf("%s was judged, want skipped", path)
		}
	}
}
```

flinch's own Contract, written in this layout, is on the next tab: flinch's Contract

## Words flinch uses

Each mutant ends with one status, and JSON's `status` field uses the same words: killed, lived, unreached, erased, skipped, declared, unviable or no verdict. A run fails on an unheld mutant, a broken declaration, a hollow obligation, a blind test or a Contract error.

| Word | Meaning | Fails the build |
| --- | --- | --- |
| Covered code | Code that some primitive's `covers` block names. flinch mutates nothing else |  |
| Killed | At least one Contract test failed, panicked or ran past its time budget against the mutant | No |
| Lived | Contract tests ran the mutated line and every one passed | Yes, unless declared |
| Unreached | No Contract test runs the mutated line | Yes, unless declared |
| Erased | A function's erase mutant lived: no Contract test noticed when the function returned zero values from its first line | Yes, unless declared |
| Skipped | A finer mutant inside an erased function. It never runs, and its function is reported instead | No |
| Unheld | Lived, unreached or erased | Yes, unless declared |
| Equivalent | Declared as impossible for any program to tell apart from the original | When a Contract test kills it, or the declaration is stale |
| Unpromised | Declared as a behavior change the Contract leaves open on purpose | The same as equivalent |
| Unviable | Fails to type-check, so it's never built or counted | No |
| No verdict | Type-checked but wouldn't build, or its row stayed incomplete past the restart cap | Exit 2 |
| Complete row | Every covering test has a pass or a fail of its own against the mutant |  |
| Holds | An obligation holds a mutant when one of its tests kills it |  |
| Sole hold | A mutant that exactly one obligation holds |  |
| Kill kind | How the killing test failed: an assertion, a panic or a timeout |  |
| Hollow obligation | Holds nothing in a full run | Yes |
| Blind test | In a full run, a Contract test that ran in complete rows of undeclared mutants and killed none | Yes |
| Redundant test | Every mutant it kills, another Contract test kills too | No, reported |
| Witness | A unit test outside the Contract that kills an unheld mutant | No, reported |
| Contract error | A problem in the Contract's own files, such as a tag that names no obligation | Yes |
| Full run | Every mutant in covered code. A run narrowed by `--since`, `--only` or `--operators` is scoped |  |

## How a run works

A run proves the Contract suite green, maps what each Contract test touches, then builds every mutant with `go test -overlay` and runs all the tests that reach it. The toolchain behavior in steps 3, 5, 8, 9 and 10 was checked on Go 1.26.1 in a scratch module, and a review run confirmed what step 4 says about initializers. The GOFLAGS hand-offs in steps 3 and 8 haven't been tried yet.

1. **Load the Contract.** Parse each README.md, mutants.md and Contract test file, check the tags, resolve the covers references, and check that every declared unit still exists. `flinch contract` stops here. It needs no Go toolchain, so it finishes in seconds.
2. **Prove the suite green.** Run the Contract suite once as a whole and then each Contract test alone. A test that fails in either run stops flinch with exit 2, and the report names it.
3. **Map coverage per test.** The lone runs use a binary built with `-cover -coverpkg` over the covered packages and write `-test.coverprofile`, which gives each test its own coverage blocks across packages. They also pass `-cover -coverpkg` through `GOFLAGs`, with a `GOCOVERDIR` per run, so a binary that a test builds and executes, such as `cmd/flinch`, reports its coverage too. `go tool covdata` merges the two.
4. **Reach initializers by linking.** `-cover` puts no counters on package-level variable initializers, and every test runs `init`. A mutant in either counts as reached by every Contract test whose binary links its package.
5. **Generate mutants.** Type-check each covered package with `go/types`, using the export data that `go list -deps -export -json` writes, so flinch needs only the standard library. When the toolchain on PATH is newer than the Go that built flinch, it falls back to type-checking from source. Each change is spliced into the file's bytes so no line number moves, and a mutant that fails to type-check is dropped as unviable.
6. **Erase first.** Each covered function's erase mutant runs before its others. It puts `return *new(T1), *new(T2)` at the top of the body, which keeps every import and variable in use. If it lives, the function is erased and its finer mutants are skipped.
7. **Prove each batch clean.** A mutant's covering tests form its batch. Each distinct batch runs once against the clean binary before any mutant uses it. A test that fails there is order-dependent, and the run stops with exit 2.
8. **Build the mutant.** Write the mutated file to a temp directory and build each covering test binary with `go test -c -overlay`. Only the changed package and what imports it recompile; a build took about 0.2 s. Tests run with `GOFLAGS= -overlay=<file>`, so a test that runs `go build` itself gets the mutant too. Each binary runs in its package's directory, the way `go test` runs it.
9. **Run the whole batch.** One process per binary, through `go tool test2json -t <binary> -test.v=test2json -test.run '[^(TestA|TestB)$]'`. There's no fail-fast, so the row names every test and subtest that fails, and how it failed.
10. **Recover from crashes.** A panic or a timeout ends the process. The panicking test is a killer. On a timeout, so is every test in Go's `running tests:` list, and the event's `Test` field is ignored because with `t.Parallel` it names whichever test printed last. The tests that hadn't run start again in a fresh process. A process that dies before its first test starts, as it does on a panic in `init`, counts as a kill by every test in the batch. When a crash names no test, the batch reruns one test per process, and after three restarts the mutant has no verdict.
11. **Find witnesses, when asked.** With `--witness`, flinch first runs and times each covered package's unit tests on clean code, and lists any that fail there. Each unheld mutant then runs against the passing unit tests of its own package.
12. **Decide.** The rows become the views by obligation, test and mutant, and then the verdict.

**Time budget.** A batch gets the sum of its tests' lone run times, times a coefficient (default 10), plus 2 seconds. A kill that came only from a timeout counts once a rerun with twice the budget times out as well. A mutant that loops forever then counts as killed by the test that was running.

**Parallelism.** `--jobs` workers, defaulting to the CPU count, each with its own overlay file. Go's build cache is safe under concurrent builds. The job count never changes a verdict.

**Cache.** A result is keyed the way Go's own test cache keys one. The key holds the mutant's bytes, the content of every file in the test binary's dependence closure, and the files and environment variables each test read, from `-test.testlogfile`. It also holds the Go and flinch versions, GOOS, GOARCH, the build tags and the timeout coefficient. Tests that talk to the network fall outside any key, and `--no-cache` is there for them. The cache lives under `os.UserCacheDir()/flinch`, and CI can keep it with `actions/cache`.

**Scale.** Gremlins found about 400 mutants in qualm's packages. qualm's whole suite runs in 8.8 s, and its slowest packages fork git. The spike measures a full flinch run on qualm before any other phase starts.

## Operators

Ten operators run by default and eight more are opt-in. `erase` runs first in every function. The rest of the default set covers mutineer's Tier 1, with `drop-call` as Go's form of statement removal. It also takes `negation` and `step` from mutineer's Tier 2 and adds `drop-error` for Go. Error paths are where Go suites tend to go quiet, and a test that never checks an error lets `return n, nil` live.

| Operator | Change | Example | Default |
| --- | --- | --- | --- |
| erase | The function returns zero values from its first line. It runs before any other mutant in the function | `return *new(int), *new(error)` at the top of `Count` | Yes |
| arithmetic | `+` and `-` swap, `*` and `/` swap, `%` becomes `*`, numeric operands only | `n + 1` to `n - 1` | Yes |
| boundary | `<` and `<=` swap, `>` and `>=` swap | `i < len(s)` to `i <= len(s)` | Yes |
| equality | `==` and `!=` swap | `err != nil` to `err == nil` | Yes |
| logical | `&&` and `\|\|` swap | `ok && done` to `ok \|\| done` | Yes |
| negation | A `!` is dropped | `!ok` to `ok` | Yes |
| bool | `true` and `false` swap |  | Yes |
| step | `++` and `--` swap, `+=` and `-=` swap | `i++` to `i--` | Yes |
| drop-error | In a `return`, an expression of type `error` becomes `nil` | `return n, err` to `return n, nil` | Yes |
| drop-call | A call statement whose results are unused is removed | `w.Flush()` removed | Yes |
| zero-return | A returned value becomes its type's zero value | `return total` to `return 0` | No |
| constant | An integer literal becomes 0, or itself plus 1 | `limit := 64` to `limit := 65` | No |
| empty-string | A non-empty string literal becomes `""` |  | No |
| branch | An `if` condition becomes `true`, or `false` |  | No |
| loop-exit | `break` and `continue` swap |  | No |
| drop-defer | A `defer` statement is removed | `defer f.Close()` removed | No |
| inline-go | `go f()` runs as `f()` |  | No |
| slice-bound | A slice's low bound moves up by 1 | `s[i:]` to `s[i+1:]` | No |

Hav MYHOSTAN descriptions here
Updatinginginging
The operators read types from `go/types`, so they skip most changes that couldn't compile, such as `-` on strings. The type-check after each splice catches the rest, like a removed statement that leaves a variable unused. A primitive picks operators in its mutants.md with a `flinch` block (`operators: +zero-return -drop-call`), and `--operators` does the same for one run.
$FosPEER descriptions UpDatinginging