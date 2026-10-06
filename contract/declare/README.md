# declare

declare checks the mutants each primitive's mutants.md declares, and finds the declaration that accounts for a mutant.

## Obligations

- **D1** An `equivalent` block lists mutant ids, one per line, and claims no program can tell them from the original. A wildcard in it is an error.
- **D2** An `unpromised` block lists ids or `<dir>.<unit>: *`, and says the Contract leaves that behavior open. A wildcard covers only the unheld mutants of the unit and the closures inside it.
- **D3** A declaration's reason is the nearest heading above its block plus the prose between them. A block with no heading above it is an error.
- **D4** A declaration belongs in the mutants.md of the primitive covering its unit. One anywhere else is an error that names the right file, and one on a unit no primitive covers is an error too.
- **D5** A declared mutant that a Contract test reaches still runs. If a Contract test kills it, the declaration is an error.
- **D6** A declaration that matches no mutant, in a run that mutated its whole unit, is stale and an error. A wildcard is stale when its unit has no unheld mutants.
- **D7** A declaration naming a unit that no longer exists is an error in every run, scoped or not.
- **D8** Each line of an `equivalent` or `unpromised` block is a mutant id or a wildcard, and declares what no other line does. A line that is neither, or that repeats another declaration, is an error.

```covers
internal/declare
```
