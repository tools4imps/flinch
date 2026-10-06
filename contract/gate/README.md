# gate

The gate turns findings into an exit code that only the commit can change.

## Obligations

- **G1** The verdict reads only the commit, its Contract, the Go toolchain, GOOS, GOARCH and the build tags. The job count, the machine and git config never change it, and `--since` reads the base only to choose mutants.
- **G2** Exit 0 means no Contract error, no unheld undeclared mutant and no broken declaration, plus no hollow obligation or blind test in a full run. Exit 1 means at least one of those. Exit 2 means flinch couldn't decide, and it wins over exit 1.
- **G3** `--since REF` mutates the covered lines changed since the merge base with REF, plus all the code covered by any primitive whose Contract directory changed. Renames and git config never change which lines count as changed.
- **G4** Contract errors are checked across the whole Contract in every run, scoped or not.
- **G5** No mutation score or threshold enters the verdict.

```covers
internal/gate
internal/since
```
