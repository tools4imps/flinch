# Changelog

## 0.1.0

First release. flinch fails a pull request when code the Contract covers can break and no Contract test notices.

- `flinch contract` checks the Contract's own files without a Go toolchain: obligations, covers blocks, tags on black-box Contract tests, and declarations.
- `flinch` mutates covered code with ten operators, erase first, builds each mutant through `go test -overlay`, and runs every Contract test that reaches it, crash and timeout recovery included.
- Kills are credited to obligations. The report names unheld mutants, hollow obligations, blind tests, obligations held only by crashes, and mutants held only from outside their primitive.
- `equivalent` and `unpromised` declarations in `mutants.md`, checked on every run.
- `--since` for pull requests, `--only`, `--operators`, `--tags`, `--dry-run`, and text and JSON reports.
- A memory guard: a test process holding more than `--memory-limit` (2048 MB by default) is stopped and counts as a crash, so a mutant that allocates without end can't take the machine down. Tests run with their temp directory inside flinch's own.
- Mutants build in a cache of the run's own, never your Go build cache. flinch empties it between chunks of mutants once it passes 2 GB, so a long run can't fill the disk.
- flinch ships with its own Contract and runs on itself in CI.
