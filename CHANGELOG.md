# Changelog

## 0.1.1

- The runner's Contract promises more of what it already did. A source file that changes under a run stops it with exit 2. A process a test leaves running doesn't hold the run up. A test that calls `os.Exit(0)` fails on a panic, as it does under `go test`. A lone test whose process crashes is a killer. A mutant that won't build carries the compiler's first complaint as its reason. The work directory goes when the run ends. Contract tests back each promise, and the declarations they replace are gone.
- A mutation run of flinch on its own code, split across GitHub's runners, turned up 19 mutants that no Contract test held. Two new Contract tests catch two of them. One runs two test processes of a suite side by side, so the sweep can't remove a file that a running test still needs. The other sets a cache limit that the cache's files pass and its directories don't. The memory guard now reads memory it can't read as 0, which removes five more. The other twelve are declared in `mutants.md`, each with the reason no test can catch it.
- flinch's own weekly mutation run is split across GitHub runners, one shard per primitive or per group of operators, and a last job merges the shards' reports into one verdict. One runner would take longer than the six hours GitHub allows a job.

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
