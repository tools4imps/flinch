# report

The report puts what has to change first, and every finding says how to clear it.

## Obligations

- **P1** The text report lists Contract errors first, then unheld mutants by primitive, file and line, each with its id, its hash, the obligations whose tests ran it and those tests.
- **P2** Mutants without a verdict, broken declarations, hollow obligations, obligations held only by crashes, mutants held only from outside, blind tests and packages outside the Contract follow, in that order. Obligations keep Contract order, by primitive and then as the README lists them. Mutants without a verdict go by primitive, file and line like unheld ones, and every other list is sorted by path and line. The report ends with why flinch couldn't decide, when it couldn't, and the exit code.
- **P3** Every finding in the text report says what would clear it, and an unheld mutant's finding names the mutants.md that would declare it.
- **P4** JSON carries `schema_version`. Additive changes bump the minor version, and consumers ignore unknown keys.
- **P5** JSON lists every mutant with its status, killers, kill kinds, covering tests and matching declaration, and lists obligations in Contract order. The same input gives the same bytes apart from the `timing` object, which gives each phase's wall time in seconds.
- **P6** Progress goes to stderr, and the report goes to stdout or the `--output` file.
- **P7** The summary counts mutants by status and the unviable ones, counts sole holds per obligation, splits declared mutants into equivalent, unpromised and wildcard, and prints the mutation score: killed mutants over killed plus unheld ones. It names the run's scope and the flinch version, Go version, GOOS, GOARCH and build tags.

```covers
internal/report
```
