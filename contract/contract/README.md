# contract

The loader reads the Contract's files and reports every problem in them.

## Obligations

- **C1** The Contract is the `contract/` directory at the module root, or the one `--contract` names. Each direct subdirectory holding a README.md is a primitive named after the directory. flinch skips `testdata`, `vendor` and names starting with `.` or `_`, and a subdirectory without a README.md holds helpers.
- **C2** An obligation is a README.md line that opens with `- **`, uppercase letters, digits and `**`. Its full id is `<primitive>/<id>`, and two obligations with the same full id are an error.
- **C3** Fences follow CommonMark: three or more backticks or tildes, closed by the same character at least as many times. A fence inside an HTML comment is dead. The info string is the first word after the fence.
- **C4** `covers` blocks count only in README.md, and `equivalent` and `unpromised` blocks count only in mutants.md. One of those blocks anywhere else is an error that names the file it belongs in.
- **C5** A symlink inside the Contract, a file that isn't UTF-8, and a Go file in a primitive's directory whose name doesn't end in `_test.go` are errors.
- **C6** Every Contract error prints as `path:line: message`. A run reports all of them at once, sorted by path and line.

```covers
internal/contract
internal/problem
```
