# flinch

flinch fails a pull request when code the Contract covers can break and no Contract test notices. It is the mutation gate for [Impatient Programming](https://impatientprogramming.org), and the sibling of [exhale](https://github.com/tools4imps/exhale-ruby) and [qualm](https://github.com/tools4imps/qualm).

A Contract, in Impatient Programming, is what a component promises: numbered obligations in `contract/<primitive>/README.md`, and a black-box test suite beside them that proves each one. Pact's consumer-driven contracts between services are a separate idea. flinch breaks the code each primitive covers, one small change at a time, runs the Contract suite against every broken copy, and records which obligation's tests noticed. A test that doesn't flinch when the code breaks proves nothing.

flinch is one Go binary built from the standard library alone. It works on Go code.

## Install

```bash
go install github.com/tools4imps/flinch/cmd/flinch@latest
```

Or download a binary for macOS, Linux or Windows from the [releases page](https://github.com/tools4imps/flinch/releases). flinch reads the export data your Go toolchain writes, so the binary has to be built with a Go at least as new as yours. `go install` takes care of that.

## Use

```bash
flinch contract   # check the Contract's own files; needs no Go toolchain
flinch            # mutate everything the Contract covers
flinch --since origin/main   # mutate what a branch changed
```

Run it anywhere inside a Go module. A real run on a small module with three obligations:

```text
flinch: 9 mutants in covered code
  6 killed, 3 unheld (2 lived, 1 unreached, 0 erased), 0 skipped, 0 no verdict
  0 declared: 0 equivalent, 0 unpromised, 0 by wildcard
  score 66.6%, killed / (killed + unheld) = 6 / 9, for information only
  3 obligations: 3 hold code, 0 hollow, 0 held only by crashes, 0 hold no mutant alone
  3 Contract tests: 0 blind

Unheld (3)
  calc  calc/calc.go:11  400b689eea3e
    calc.Clamp: n < lo -> n <= lo
    lived: run by calc/A2 (TestClampInside), which passed
    to clear: strengthen those tests, write the obligation this change breaks, or declare it in contract/calc/mutants.md
  calc  calc/calc.go:25  a641db354147
    calc.Div: a / b -> a * b
    unreached: no Contract test runs this line
    to clear: write the promise that needs this code, delete the code, or declare it unpromised in contract/calc/mutants.md
  ...
exit 1
```

flinch exits 0 when the Contract holds, 1 when it doesn't, and 2 when it couldn't decide, such as when the Contract suite fails on clean code.

## The Contract

```text
contract/
  calc/
    README.md       obligations and a covers block
    calc_test.go    the calc Contract suite
    mutants.md      declared survivors, optional
```

A primitive's README lists its obligations and names the code it covers:

````markdown
# calc

Small sums.

## Obligations

- **A1** Add returns the sum of its arguments.
- **A2** Clamp returns n when it sits between lo and hi, and the nearer bound otherwise.

```covers
calc
```
````

A `covers` line is a package directory relative to the module root (`internal/skip`), a tree (`internal/...`), a function, type or variable (`internal/check.Keep`), or one method (`internal/check.Runner.Settle`). The most specific reference wins.

The Contract suite lives in the primitive's own directory, so Go's visibility rules hold it to exported API. Every top-level test there names the obligations it proves:

```go
package calc_test

import (
	"testing"

	"example.com/demo/calc"
)

// Contract: calc/A1
func TestAddSums(t *testing.T) {
	if calc.Add(2, 3) != 5 {
		t.Fatal("2+3")
	}
}
```

`flinch contract` checks all of this without building anything: every obligation is named by a Contract test, every Contract test names an obligation of its own primitive, no unit test elsewhere carries a tag, and every covers line names real code.

## What fails the build

| Finding | Means |
| --- | --- |
| Lived | Contract tests ran the changed line and every one passed |
| Unreached | No Contract test runs the line |
| Erased | The function returned zero values from its first line and nobody noticed |
| Hollow obligation | In a full run, its tests killed nothing |
| Blind test | In a full run, a Contract test that killed nothing |
| Broken declaration | A declared mutant a Contract test kills, or a declaration that matches nothing |
| Contract error | A problem in the Contract's own files |

There's no score threshold. The score is printed for information. Every mutant in covered code is either killed or declared.

## Declaring survivors

Some mutants can't be killed, and some changes the Contract leaves open on purpose. Declare them in the primitive's `mutants.md`, under a heading that gives the reason:

````markdown
# Mutants the diff Contract leaves alone

## Hunk text always starts with "@@ "

`rest` begins with `@@ ` wherever Hunks cuts it, so the index of
"\n@@ " is never 0, and `>=` and `>` agree.

```equivalent
internal/gitdiff.Hunks: i >= 0 -> i > 0
```
````

`equivalent` claims no program could tell the mutant from the original. `unpromised` says the behavior does change and the Contract leaves it open; it also accepts a wildcard over one unit, `internal/jev.trim: *`. flinch still runs every declared mutant a Contract test reaches. A declaration a test contradicts fails the build, and so does one that matches nothing.

## Operators

`flinch operators` lists them. `erase` runs first in each function; when it lives, the function is reported once and its finer mutants are skipped.

| Operator | Change |
| --- | --- |
| erase | the function returns zero values from its first line |
| arithmetic | `+` and `-` swap, `*` and `/` swap, `%` becomes `*`, numeric operands only |
| boundary | `<` and `<=` swap, `>` and `>=` swap |
| equality | `==` and `!=` swap |
| logical | `&&` and `\|\|` swap |
| negation | a `!` is dropped |
| bool | `true` and `false` swap |
| step | `++` and `--` swap, `+=` and `-=` swap |
| drop-error | in a return, an expression of type `error` becomes `nil` |
| drop-call | a call whose results are unused is removed; calls into `log` and `log/slog` are left alone |

Mutants are compiled through `go test -overlay`, so your working tree is never touched.

## Flags

| Flag | Effect |
| --- | --- |
| `--since REF` | mutate the covered lines changed since the merge base with REF, plus everything covered by a primitive whose Contract directory changed |
| `--only PRIMITIVE` | mutate one primitive's covered code; repeatable |
| `--operators LIST` | run only these operators |
| `--tags LIST` | build tags for every go command flinch runs |
| `--jobs N` | parallel workers, the CPU count by default |
| `--timeout-coefficient N` | multiplies the clean run time in each time budget, 10 by default |
| `--contract DIR` | where the Contract lives, `contract` by default |
| `--format text\|json` | report format |
| `--output FILE` | write the report to a file |
| `--dry-run` | list the mutants a run would build, and build none |

## In CI

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/tools4imps/flinch/cmd/flinch@latest
- run: flinch contract
- run: flinch --since "origin/${{ github.base_ref }}"
  if: github.event_name == 'pull_request'
- run: flinch
  if: github.event_name == 'push'
```

On an existing codebase, `--since` holds new code from the first day while a full run on main lists the backlog. Make that job non-required until the backlog is gone.

## Not in this release

The design in [`docs/design.md`](docs/design.md) goes further than v0.1.0. Still to come: a result cache, unit tests as witnesses for missing obligations (`--witness`), `flinch explain`, the opt-in operators, redundant-test reporting, and coverage for binaries that tests build themselves.

## How flinch is held to account

flinch has its own Contract in [`contract/`](contract): 74 obligations across eleven primitives, each proved by black-box Contract tests. CI runs `flinch contract` on every push, mutates what each pull request changes, and mutates everything on main.

## License

MIT
