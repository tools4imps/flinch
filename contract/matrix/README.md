# matrix

The matrix turns rows of kills into findings about obligations and tests.

## Obligations

- **X1** An obligation holds a mutant when one of its tests killed it. Its sole holds are the mutants no other obligation holds.
- **X2** In a full run, an obligation that holds nothing is hollow.
- **X3** In a full run, a Contract test that ran in at least one complete row of an undeclared mutant and killed nothing is blind. A test seen only in incomplete rows is never called blind.
- **X4** A scoped run judges hollow obligations and blind tests only for primitives whose covered code it mutated whole.
- **X5** An obligation whose every hold came from a panic or a timeout is reported as held only by crashes. That never fails the build.
- **X6** A mutant in one primitive's code that only other primitives' tests kill is reported as held only from outside. That never fails the build.

```covers
internal/matrix
internal/model
```
