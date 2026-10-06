# cli

The flinch command line, and the engine that carries a run from the Contract checks to the verdict.

## Obligations

- **L1** `flinch` and `flinch run` do the whole check. `flinch contract` stops after the Contract checks, and `flinch operators` and `flinch version` print what their names say. `flinch help` and `-h` print the usage text, and `--version` prints the version.
- **L2** An unknown command, flag or argument, or a bad flag value, exits 2 with no report and a single message on stderr that names it. A primitive `--only` can't find, an operator `--operators` doesn't know, a ref `--since` can't resolve and a file `--output` can't create are bad values.
- **L3** `--dry-run` lists every mutant a run would build, with its id, and builds none.
- **L4** `--tags` passes its build tags to every go command flinch runs.
- **L5** `--only` mutates only the named primitives' covered code, and the run counts as scoped.
- **L6** `flinch contract` prints how many primitives, obligations and Contract tests the Contract has, every Contract error sorted by path and line, and the packages outside the Contract. A run with a Contract error stops before it generates a mutant or runs a test, and exits 1.
- **L7** A mutant's covering tests are the Contract tests whose lone run reached it. An erase mutant is reached through any part of its function's body, a mutant in a case or select header through the statement holding the header, and a mutant in a package-level variable's initializer or an init function by every test whose binary links its package.
- **L8** Erase mutants run before every other mutant. When a function's erase mutant lives, the function's other mutants are skipped and never built.
- **L9** `--operators` keeps only the named operators' mutants. `--since REF` keeps the mutants on lines changed since the merge base with REF, the erase mutant of each function with a changed line, and every mutant of a primitive whose Contract directory changed.
- **L10** A run is full when no `--since`, `--only` or `--operators` narrows it. Under the default operators, a run mutates whole every primitive in its `--only` scope, or under `--since` those of them whose Contract directory changed, and every unit it drops no mutant from. Under fewer operators it mutates nothing whole.
- **L11** flinch exits 2 with no verdict and says why on stderr when it can't read the module or its Contract, can't type-check covered code, can't find the go command, can't make its work directory, or is interrupted. When a Contract test fails on clean code, flinch exits 2 with a report that names the test.
- **L12** The report goes to stdout, or to the file `--output` names, as text unless `--format json` asks for JSON, and progress goes to stderr. A run's report records the go command's version, GOOS and GOARCH, every obligation, how many mutants were unviable, and the packages outside the Contract.
- **L13** `--memory-limit MB` stops any test process that holds more than MB megabytes, 2048 by default, and the test it ran counts as crashing. `--memory-limit 0` turns the guard off.

```covers
internal/cli
internal/engine
```
