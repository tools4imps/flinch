# cli mutants

## A flag's default text is never shown

The flag set records each flag's String as its default value, which only its own usage output would print. flinch sends that output to io.Discard and prints a usage text of its own, so no program can see what String returns.

```equivalent
internal/cli.listFlag.String: { ... } -> { return *new(string) }
```

## An erase mutant's span starts at the opening brace either way

The span ends one column before the closing brace instead of one after it, but it still starts at the opening brace. Go's cover tool starts a function body's first block at that brace, and a function that ran at all counts that block, so the span reaches the same tests.

```equivalent
internal/engine.Prepare: to.Column + 1 -> to.Column - 1
```

## Timing is wall time

The report's timing object is the one part of a report that varies between identical runs. The Contract promises nothing about the numbers in it.

```unpromised
internal/engine.Run: time.Since(started) - timing["plan"] -> time.Since(started) + timing["plan"]
internal/engine.Run: time.Since(started) - timing["plan"] -> time.Since(started) + timing["plan"] #2
internal/engine.Run: time.Since(started) - timing["plan"] - timing["prove"] -> time.Since(started) - timing["plan"] + timing["prove"]
```

## A go command that prints too few lines

`go env` prints one line for each variable it's asked for, so with any Go toolchain this guard never stops the loop. What flinch does with a go command that prints fewer lines is left open.

```unpromised
internal/engine.goEnv: i < len(lines) -> i <= len(lines)
```

## No mutant starts at a case keyword or a colon

A case header's range runs from its case or default keyword to its colon. Every operator replaces an expression, a call statement or a function body, so no mutant starts on the keyword or on the colon, and moving either end of the range by one byte matches the same mutants.

```equivalent
internal/engine.Prepare: h.from <= m.Start -> h.from < m.Start
internal/engine.Prepare: m.Start < h.to -> m.Start <= h.to
```

## ast.Inspect ignores the result of its closing call

ast.Inspect calls the function with nil after a node's children and drops what it returns, so the closing call may return true or false.

```equivalent
internal/engine.caseHeads.func1: true -> false
```

## Only a case clause has a colon, and it always sits deep

A node with no colon gives a header range that ends at offset 0, which no mutant starts before, so recording one changes nothing. A case clause always has the file, a declaration, a body, its switch or select and that statement's block above it, so the stack never holds exactly two nodes when one is found.

```equivalent
internal/engine.caseHeads.func1: colon.IsValid() && len(stack) >= 2 -> colon.IsValid() || len(stack) >= 2
internal/engine.caseHeads.func1: len(stack) >= 2 -> len(stack) > 2
```
