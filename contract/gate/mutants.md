# gate mutants

## The wording of the gate's reasons

Decide returns a reason for each kind of finding beside the exit code. No report prints them, and
the Contract promises only the exit code.

```unpromised
internal/gate.count: { ... } -> { return *new(string) }
```

## Sets whose values nothing reads

The changed paths are a set. TouchesDir and Paths range over its keys and never read a value, so
storing false where it stores true changes nothing.

```equivalent
internal/since.Changed: true -> false
internal/since.Changed: true -> false #2
```

## Results nobody reads beside an error

When hunkSpan returns an error, its caller returns the error and never looks at the span or the
flag beside it.

```equivalent
internal/since.hunkSpan: false -> true
internal/since.hunkSpan: false -> true #2
internal/since.hunkSpan: false -> true #4
```

## A comma at the start of a hunk's new side

With the comma first, the start is empty and fails to parse, and with the whole field read as the
start it fails to parse the same way, with the same message.

```equivalent
internal/since.hunkSpan: i >= 0 -> i > 0
```

## Adding no untracked files

git add with an empty list of paths adds nothing and succeeds, so skipping the call when there
are no untracked files changes nothing but the number of git processes.

```equivalent
internal/since.Changed: len(untracked) > 0 -> len(untracked) >= 0
```

## The wording of git's failures

When git fails, flinch stops with an error that says what it was doing. Which git subcommand the
message names, whether it quotes git's own complaint, and which of two complaints it gives for a
hunk header with nothing before its closing @@ are left open. So is which failure it reports when
the index can't be found or read: going on without a copy of the index makes git fail to note the
untracked files, and with no files at all git reads the same empty index either way.

```unpromised
internal/since.gitWith: msg != "" -> msg == ""
internal/since.subcommand: { ... } -> { return *new(string) }
internal/since.hunkSpan: end < 0 -> end <= 0
internal/since.copyIndex: return "", fmt.Errorf("finding the git index: %w", err) -> return "", nil
internal/since.copyIndex: return "", fmt.Errorf("reading the git index: %w", err) -> return "", nil
```

## Patches git never writes

git starts every patch with a diff --git line and numbers lines from 1. What flinch makes of a
header before the first diff --git line, and whether line 0 of a file counts as changed, is left
open.

```unpromised
internal/since.parsePatch: false -> true
internal/since.hunkSpan: false -> true #3
```
