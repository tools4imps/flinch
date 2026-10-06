# gate

The gate turns findings into an exit code that only the commit can change.

## Obligations

- **G1** The verdict reads only the commit, its Contract, the Go toolchain, GOOS, GOARCH and the build tags. The job count, the machine and git config never change it, and `--since` reads the base only to choose mutants.
- **G2** Exit 0 means no Contract error, no unheld undeclared mutant and no broken declaration, plus no hollow obligation or blind test in a full run. Exit 1 means at least one of those. Exit 2 means flinch couldn't decide, as when a mutant has no verdict or a git command fails, and it wins over exit 1.
- **G3** `--since REF` mutates the covered lines that differ between the merge base with REF and the working tree, plus all the code covered by any primitive whose Contract directory changed. Staged, unstaged and untracked files count alike, and ignored files and files a sparse checkout leaves out don't count. A renamed file counts only its edited lines, whether the rename is committed, staged or a plain move, and no git config, attributes file or git environment variable changes which lines count as changed.
- **G4** Contract errors are checked across the whole Contract in every run, scoped or not.
- **G5** No mutation score or threshold enters the verdict.

```covers
internal/gate
internal/since
```
