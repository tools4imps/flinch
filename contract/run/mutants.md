# run: declared mutants

## Room that's never needed

Apply reserves room for the mutated file before it copies the bytes in, and reserving more than it needs leaves the same bytes. readProfile reads a coverage profile line by line. A profile line holds an import path, a file name and four numbers, far below the 64 KB a Scanner reads without a buffer of its own, and reading from memory never fails, so the buffer sizes and the scanner's error never change what's read.

```equivalent
internal/model.Mutant.Apply: len(src)-(m.End-m.Start) -> len(src)+(m.End-m.Start)
internal/runner.readProfile: sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) -> (removed)
internal/runner.readProfile: 64*1024 -> 64/1024
internal/runner.readProfile: 1024*1024 -> 1024/1024
internal/runner.readProfile: return out, sc.Err() -> return out, nil
```

## Sets read only by their keys

These maps are sets. Everything that reads them asks only which keys are present, or how many there are, so storing false in place of true changes nothing. deps keeps a set too, to list each package once; when it lists one twice, link folds the copies into the same sets.

```equivalent
internal/runner.coverage.add: true -> false
internal/runner.Baseline.link: true -> false
internal/runner.Baseline.reach: true -> false
internal/runner.Baseline.ReachSpan: true -> false
internal/runner.Baseline.groups: true -> false
internal/runner.stream.frame: true -> false #2
internal/runner.runningTests: true -> false #2
internal/runner.Baseline.deps: true -> false
```

## Orders that never show

Reach and ReachSpan gather the tests of every block that matches into a set, so the order of a file's blocks never shows. Suite numbers are distinct, so `<` and `<=` sort them the same way.

```equivalent
internal/runner.coverage.blocks: sort.Slice(bs, func(i, j int) bool { a, b := bs[i].span, bs[j].span if a.sl != b.sl || a.sc != b.sc { return before(a.sl, a.sc, b.sl, b.sc) } return before(a.el, a.ec, b.el, b.ec) }) -> (removed)
internal/runner.coverage.blocks.func1: a.sl != b.sl -> a.sl == b.sl
internal/runner.coverage.blocks.func1: a.sl != b.sl || a.sc != b.sc -> a.sl != b.sl && a.sc != b.sc
internal/runner.coverage.blocks.func1: a.sc != b.sc -> a.sc == b.sc
internal/runner.Baseline.groups.func1: gs[i].s.n < gs[j].s.n -> gs[i].s.n <= gs[j].s.n
```

## Inputs that can't occur

An import path never starts with a space, and a test name is never empty and never starts with a slash. Every `---` line the testing package writes ends with the test's time in parentheses. Go puts the list of running tests on the lines right after its timeout panic. An output line only matters when it starts with `panic: `, and an empty one never does, so keeping or dropping empty strings changes nothing.

```equivalent
internal/runner.Baseline.deps: i >= 0 -> i > 0
internal/runner.stream.frame: i >= 0 -> i > 0
internal/runner.stream.frame: i >= 0 && strings.HasSuffix(name, "s)") -> i >= 0 || strings.HasSuffix(name, "s)")
internal/runner.runningTests: i >= 0 -> i > 0
internal/runner.runningTests: strings.TrimSpace(line) != "" -> strings.TrimSpace(line) == ""
internal/runner.top: i >= 0 -> i > 0
internal/runner.parseStream: len(parts) == 1 -> len(parts) != 1
```

## Restart bookkeeping

batch is never called without tests, and its loop goes round again only while tests are pending. A process never settles a negative number of tests, so their sum is zero exactly when each count is. When a process settles every test it was asked to run, nothing is pending, so whether its end named a test is never read. Go's timeout panic lists every test still running, the budget runs out only while one is, and each of them is a test the process was asked to run with no outcome yet. A row with no failing test gives the timeout rerun nothing to run.

```equivalent
internal/runner.Baseline.batch: len(pending) > 0 -> len(pending) >= 0
internal/runner.Baseline.batch: settled += n -> settled -= n
internal/runner.settle: true -> false
internal/runner.settle: true -> false #2
internal/runner.settle: true -> false #4
internal/runner.settle: false -> true #2
internal/runner.settle: out[t] == nil && contains(set, t) -> out[t] == nil || contains(set, t)
internal/runner.contains: false -> true
internal/runner.onlyTimeouts: false -> true
```

## Errors from an ended context

The errors these lines return come only from the run's context ending. undecidedRun's line returns only the context's own errors, and the go command fails with anything but a go command error only when its context has ended. Every caller then reaches parallel, which returns the context's error whatever these lines return.

```equivalent
internal/runner.Baseline.undecidedRun: return err -> return nil
internal/runner.Baseline.job: return model.Row{}, b.undecidedRun(err) -> return model.Row{}, nil
internal/runner.undecidedGo: return err -> return nil
```

## Small things that don't change the outcome

Job directories stay distinct whichever way the counter moves. A map of strings always encodes as JSON. With no packages to cover, a coverage build records no blocks, the same as the plain binary it replaces. parallel never has more calls than n, so a worker past the nth never gets one.

```equivalent
internal/runner.Baseline.jobDir: b.seq++ -> b.seq--
internal/runner.Baseline.overlay: return "", "", err -> return "", "", nil
internal/runner.Baseline.alone: len(coverpkg) > 0 -> len(coverpkg) >= 0
internal/runner.parallel: w < n -> w <= n
```

## Progress

Progress lines are for the person watching. The Contract leaves their wording, their pacing and whether they appear at all open.

```unpromised
internal/runner.Baseline.progress: *
internal/runner.Baseline.tick: *
internal/runner.Baseline.Run.func1: b.tick(t) -> (removed)
internal/runner.Prove: b.progress("flinch: running %d Contract suites whole", len(b.suites)) -> (removed)
internal/runner.Baseline.alone: b.progress("flinch: running %d Contract tests alone", len(runs)) -> (removed)
```

## Which step reports a failure, and in what words

When a go command fails while Prove runs, or a suite's binary can't run, a later step that needs the same thing fails too, and the run stops with exit 2 either way. R1 promises the report names a test that fails. Which step's reason the report shows, and how a reason words a suite's failure, are left open.

```unpromised
internal/runner.fails: *
internal/runner.firstLine: *
internal/runner.Baseline.deps: return nil, err -> return nil, nil
internal/runner.Baseline.link: return nil, undecidedGo(err, "go list can't load the Contract suite for "+s.Primitive) -> return nil, nil
internal/runner.Baseline.whole: return undecidedGo(err, "the Contract suite for "+s.Primitive+" doesn't build") -> return nil
internal/runner.Baseline.whole: return b.undecidedRun(err) -> return nil
internal/runner.Baseline.alone.func1: return undecidedGo(err, "the Contract suite for "+s.Primitive+" doesn't build with coverage") -> return nil
internal/runner.Baseline.alone: return err -> return nil
internal/runner.Baseline.alone.func2: return b.undecidedRun(err) -> return nil
internal/runner.judgeClean: !p.anyStarted() -> p.anyStarted()
internal/runner.judgeClean: !p.anyStarted() && !p.ended -> !p.anyStarted() || !p.ended
internal/runner.judgeClean: !p.ended -> p.ended
```

## The order of tests and suites

Reach, ReachSpan, Linking and a row list each test once, and a kill lists each failing subtest once. The Contract promises which they list and leaves the order open. The same goes for the order of the lone runs, and for the order a job's suites run in, which shows only in which suite's reason a row without a verdict gives.

```unpromised
internal/runner.sortRefs: *
internal/runner.sortedRefs: sortRefs(out) -> (removed)
internal/runner.row: sortRefs(r.Ran) -> (removed)
internal/runner.row: sort.Slice(r.Kills, func(i, j int) bool { a, b := r.Kills[i].Test, r.Kills[j].Test if a.Primitive != b.Primitive { return a.Primitive < b.Primitive } return a.Name < b.Name }) -> (removed)
internal/runner.row.func1: a.Primitive != b.Primitive -> a.Primitive == b.Primitive
internal/runner.row.func1: a.Primitive < b.Primitive -> a.Primitive <= b.Primitive
internal/runner.row.func1: a.Name < b.Name -> a.Name <= b.Name
internal/runner.stream.failedSubtests: sort.Strings(out) -> (removed)
internal/runner.Prove: sort.Strings(s.Tests) -> (removed)
internal/runner.Baseline.groups: sort.Slice(gs, func(i, j int) bool { return gs[i].s.n < gs[j].s.n }) -> (removed)
```

## Coverage profiles that don't parse

Go's testing package writes every coverage profile the runner reads, and it never writes a line these checks reject. A profile goes bad only when a test rewrites it after the run, and the Contract leaves open what flinch does then.

```unpromised
internal/runner.readProfile: return nil, fmt.Errorf("unreadable coverage line %q", line) -> return nil, nil
internal/runner.readProfile: return nil, fmt.Errorf("unreadable coverage line %q", line) -> return nil, nil #2
internal/runner.readProfile: return nil, fmt.Errorf("unreadable coverage line %q", line) -> return nil, nil #3
internal/runner.readProfile: colon < 0 -> colon <= 0
internal/runner.parseSpan: return span{}, fmt.Errorf("no comma") -> return span{}, nil
internal/runner.parseSpan: err1 != nil || err2 != nil -> err1 != nil && err2 != nil
internal/runner.parseSpan: return span{}, fmt.Errorf("bad position") -> return span{}, nil
internal/runner.lineCol: return 0, 0, fmt.Errorf("no dot") -> return 0, 0, nil
internal/runner.lineCol: err1 != nil || err2 != nil -> err1 != nil && err2 != nil
internal/runner.lineCol: return 0, 0, fmt.Errorf("bad number") -> return 0, 0, nil
```

## Output the go command never writes

go list writes JSON. What the runner does when it can't decode what go list wrote is left open.

```unpromised
internal/runner.Baseline.deps: return nil, fmt.Errorf("reading go list output: %v", err) -> return nil, nil
```

## How an interrupted run fails

An interrupted run returns an error, and flinch exits 2. Whether that error is the context's own, the cause the context was given, or a reason saying a go command failed is left open.

```unpromised
internal/runner.Baseline.goCmd: return nil, ctx.Err() -> return nil, nil
internal/runner.parallel: ctxErr == nil -> ctxErr != nil
```

## Calls the engine never makes

The runner's API is flinch's own. The engine gives it an absolute module root, a work directory the runner can write, at least one job, only packages inside the module to cover, jobs whose tests belong to suites Prove ran, and mutants whose start comes neither before the file nor after their end. What the runner does with anything else is left open.

```unpromised
internal/runner.Prove: o.Jobs <= 0 -> o.Jobs < 0
internal/runner.Prove: return nil, err -> return nil, nil
internal/runner.Prove: return nil, err -> return nil, nil #2
internal/runner.Baseline.job: return model.Row{}, err -> return model.Row{}, nil
internal/runner.Baseline.link: err != nil || rel == ".." -> err != nil && rel == ".."
internal/runner.Baseline.link: err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) -> err != nil || rel == ".." && strings.HasPrefix(rel, ".."+string(filepath.Separator))
internal/runner.Baseline.overlay: m.Start < 0 || m.Start > m.End -> m.Start < 0 && m.Start > m.End
```

## What a row without a verdict lists

A row without a verdict makes the run exit 2, and the Contract leaves open which tests it lists as run or as killers.

```unpromised
internal/runner.Baseline.confirm: delete(res, ref) -> (removed)
```

## Text before a framing line

A test that prints without a final newline leaves its text at the start of the line where the testing package's next report begins. Whether that text counts toward the test's output, and so whether it can mark a panic, is left open. Only a test that prints the words `panic: ` itself can tell.

```unpromised
internal/runner.parseStream: parts[0] != "" -> parts[0] == ""
internal/runner.parseStream: parts[0] != "" || len(parts) == 1 -> parts[0] != "" && len(parts) == 1
```

## Limits met exactly

The memory guard and the cache limit compare a size with a limit. A test process that holds exactly the memory limit, or a cache that holds exactly the cache limit, can't be arranged, so what flinch does at the limit itself is left open. The cache's size is what its files hold, and whether the few kilobytes of each directory count too is left open.

```unpromised
internal/runner.watchMemory.func2: n > limit -> n >= limit
internal/runner.Baseline.trimCache: size > limit -> size >= limit
internal/runner.Baseline.trimCache.func1: err == nil && !d.IsDir() -> err == nil || !d.IsDir()
```

## A build that worked, built again

R15 promises a mutant whose build fails one more try. Building a mutant that built fine a second time gives the same binary, and once the run has been stopped a second build fails at once. How often flinch builds a mutant beyond that one more try is left open.

```unpromised
internal/runner.Baseline.buildMutant: err != nil && ctx.Err() == nil -> err != nil || ctx.Err() == nil
```

## How the jobs are chunked

R14 promises the run's own cache is emptied between chunks of mutants once it's past the limit. How many mutants make a chunk, and whether a pass with no jobs left checks the cache once more, only change when the cache is emptied. The cache saves time and nothing else, so both are left open.

```unpromised
internal/runner.Baseline.Run: start < len(jobs) -> start <= len(jobs)
internal/runner.Baseline.Run: b.o.Jobs*8 -> b.o.Jobs/8
```

## A go command that fails once the run is stopped

When the run has been stopped, a go command fails because of it. Every later step checks for the stop as well, so the run ends the same way whether this step names the stop or passes on an empty result. Which one it does is left open.

```unpromised
internal/runner.Baseline.goCmdEnv: return nil, ctx.Err() -> return nil, nil
```

## A suite directory that can't be listed

The engine hands the runner suites from the Contract's own directories, which it has just read. What Prove does when one of them can't be listed is left open.

```unpromised
internal/runner.Prove: return nil, err -> return nil, nil #3
internal/runner.listTree.func1: return err -> return nil
internal/runner.listTree.func1: return err -> return nil #2
```

## A new directory, removed whole

When the sweep finds a new directory in a suite's directory, it removes the directory whole and tells the walk to skip it. A walk that tries to go in anyway finds it gone, and the sweep ignores that error, so the same files go either way.

```equivalent
internal/runner.suite.leave.func1: return filepath.SkipDir -> return nil
```
