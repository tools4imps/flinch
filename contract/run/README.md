# run

The runner proves the Contract suite green, maps what each Contract test reaches, and runs the covering tests against every reached mutant.

## Obligations

- **R1** Before mutating, flinch runs the Contract suite whole and then each Contract test alone. A test that fails either time stops the run with exit 2, and the report names it.
- **R2** The lone runs record each test's coverage blocks. A mutant's covering tests are the Contract tests whose lone run reached its block.
- **R3** A mutant in a package-level variable's initializer or an `init` function is reached by every Contract test whose binary links its package.
- **R4** A mutant no Contract test reaches is unreached and is never built.
- **R5** Each distinct batch of covering tests runs once against the clean binary before any mutant uses it, with ten times a mutant's time budget. A test that fails there stops the run with exit 2.
- **R6** A reached mutant is compiled through `-overlay`. The working tree, the git index and every source file are unchanged after a run, including an interrupted one. When a source file changes during a run so that a mutant no longer fits it, the run stops with exit 2.
- **R7** Each test binary runs in its package's directory, the way `go test` runs it. A process a test leaves running doesn't hold up the run until it ends.
- **R8** A mutant runs its whole batch without stopping at the first failure. Its row records every test and subtest that failed, and whether each failed on an assertion, a panic or a timeout. A test that calls `os.Exit(0)` fails on a panic, as it does under `go test`.
- **R9** When a panic or timeout ends the process, the tests that hadn't run start again in a fresh one. A timeout is pinned on the tests in Go's `running tests:` list, never on the event stream's last `Test` field.
- **R10** A process that dies before its first test starts counts as a kill by every test in its batch. A crash that names no test reruns the batch one test per process, and a lone test whose process crashes is a killer. After three restarts that settle nothing, the mutant has no verdict.
- **R11** The time budget is the sum of the batch's lone run times, times the coefficient (default 10), plus 2 seconds. A kill that came only from a timeout counts once a rerun with twice the budget times out as well.
- **R12** A mutant that type-checked but won't build has no verdict, and its reason is the compiler's first complaint, naming the file by its path in the module. Any mutant without a verdict makes the run exit 2.
- **R13** A test process that holds more than the memory limit is stopped, and counts as crashing: the lone test it ran is a killer, and a batch reruns one test per process.
- **R14** Test processes run with TMPDIR, TMP and TEMP inside the run's work directory, which flinch removes when the run ends. Mutant builds use a build cache inside the run's work directory, or the outer run's when a Contract test starts flinch, so the user's Go build cache never holds a mutant. Between chunks of mutants, a run empties its own cache once it has grown past the cache limit (2 GB by default), and a run that borrowed an outer run's cache never empties it.
- **R15** A stray `.go` file a test process leaves directly in its suite's directory is removed as soon as the process ends, and everything else new there goes once no process of the suite is running, so a mutant that makes a test write files can't break later builds. A mutant whose build fails gets one more try after that cleanup.

```covers
internal/runner
internal/model.Mutant.Apply
```
