# declare mutants

## Whitespace inside a unit's name is open

Exact parses an id and prints it again before it looks it up, and falls back to collapsing the id's whitespace. The two keys differ only when the directory or the unit holds a run of whitespace, and no Go package directory or unit can.

```unpromised
internal/declare.Index.Exact: !pat.Wildcard -> pat.Wildcard
```
