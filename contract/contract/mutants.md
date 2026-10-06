# Declared mutants

## Sorting what os.ReadDir already sorted

Load builds its list of primitives from os.ReadDir, which returns a directory's entries sorted by name, and no two primitives share a name. Sorting the list again, or sorting it with `<=`, gives the same list.

```equivalent
internal/contract.Load: sort.Slice(c.Primitives, func(i, j int) bool { return c.Primitives[i].Name < c.Primitives[j].Name }) -> (removed)
internal/contract.Load.func1: c.Primitives[i].Name < c.Primitives[j].Name -> c.Primitives[i].Name <= c.Primitives[j].Name
```

## Comparisons that run only once the two sides differ

Sort compares two paths with `<` only after it has found them unequal, and two lines the same way, so `<` and `<=` give the same answer there. Two problems that tie on their message as well are equal in every field, so no program can see which of them comes first.

```equivalent
internal/problem.Sort.func1: a.Path < b.Path -> a.Path <= b.Path
internal/problem.Sort.func1: a.Line < b.Line -> a.Line <= b.Line
internal/problem.Sort.func1: a.Message < b.Message -> a.Message <= b.Message
```

## Guards that can't change the answer

strings.Split always returns at least one string, so the length of the lines it returns is never 0. badLine runs only on bytes that aren't valid UTF-8, so it returns at the first bad byte, and at the end of its input utf8.DecodeRune reports an error that makes it return the same line anyway.

```equivalent
internal/contract.scan: len(lines) > 0 -> len(lines) >= 0
internal/contract.badLine: len(data) > 0 -> len(data) >= 0
```

## An empty line inside a comment

When a whole line sits inside an HTML comment, outsideComments returns an empty string with it. Treating that empty string as live text changes nothing, because an empty line holds no obligation, no fence and no heading.

```equivalent
internal/contract.outsideComments: false -> true
```

## A Contract that changes while flinch reads it

Load walks the whole Contract and reads every file in it before it reads each primitive's directory a second time. The second reads fail only when the Contract changes on disk in between, and no obligation says what flinch does then.

```unpromised
internal/contract.Load: return nil, nil, err -> return nil, nil, nil #3
internal/contract.Load: return nil, nil, err -> return nil, nil, nil #4
internal/contract.Load: return nil, nil, err -> return nil, nil, nil #5
internal/contract.loader.primitive: return err -> return nil
internal/contract.loader.primitive: return err -> return nil #2
```

## Files that are neither directories nor regular files

C5 makes a symlink, a file that isn't UTF-8 and a stray Go file errors. A named pipe, a socket or a device inside the Contract is none of those, and the Contract leaves open what flinch does with one.

```unpromised
internal/contract.loader.walk.func1: d.IsDir() || !d.Type().IsRegular() -> d.IsDir() && !d.Type().IsRegular()
```

## Which files flinch searches for misplaced blocks

C4 makes a `covers`, `equivalent` or `unpromised` block outside its own file an error. flinch looks for one in the Markdown files that sit directly in a primitive's directory. The Contract leaves open whether it also looks in other kinds of file, or reads through a symlink, which C5 already makes an error.

```unpromised
internal/contract.loader.primitive: !e.Type().IsRegular() || !strings.HasSuffix(name, ".md") -> !e.Type().IsRegular() && !strings.HasSuffix(name, ".md")
```

## What is left of a line after a comment closes on it

C3 says that fences follow CommonMark and that a fence inside an HTML comment is dead. The two rules disagree about a fence written on the same line after a comment closes, which CommonMark counts as part of the comment, so the Contract leaves the rest of such a line open.

```unpromised
internal/contract.outsideComments: start+4+end -> start+4-end
internal/contract.outsideComments: start+4+end+3 -> start+4+end-3
```

## How an error about a whole file prints

C6 gives every error a line, but an error about a whole file or directory, such as a symlink, has none. flinch prints it as `path: message`, and the Contract leaves open whether it prints a line of 0 instead.

```unpromised
internal/problem.Problem.String: p.Line > 0 -> p.Line >= 0
```
