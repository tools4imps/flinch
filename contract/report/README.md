# report

The report puts what has to change first, and every finding says how to clear it.

## Obligations

- **P1** The text report lists Contract errors first, then unheld mutants by primitive, file and line, each with its id, its hash and the obligations whose tests ran it.
- **P2** Broken declarations, hollow obligations, obligations held only by crashes, mutants held only from outside, blind tests and packages outside the Contract follow, each list sorted by path and line.
- **P3** Every finding in the text report says what would clear it.
- **P4** JSON carries `schema_version`. Additive changes bump the minor version, and consumers ignore unknown keys.
- **P5** JSON lists every mutant with its status, killers, kill kinds, covering tests and matching declaration. The same input gives the same bytes apart from the `timing` object.
- **P6** Progress goes to stderr, and the report goes to stdout or the `--output` file.
- **P7** The summary counts sole holds per obligation, splits declared mutants into equivalent, unpromised and wildcard, and prints the mutation score: killed mutants over killed plus unheld ones.

```covers
internal/report
```
