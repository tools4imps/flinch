# tags

Tags tie each Contract test to the obligations it checks.

## Obligations

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
