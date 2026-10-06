# identity

Every mutant has an id that people can read and type, and a short hash of it.

## Obligations

- **I1** A mutant's id is `<dir>.<unit>: <original> -> <replacement>`. When the same change appears n times in a unit, the second and later ones end with a space and `#n`.
- **I2** A unit is a function, `Type.Method`, `Func.funcN` for the N-th function literal inside `Func`, a package-level variable's name, or `init.N` for a package's init functions, numbered from 0 the way the compiler numbers them.
- **I3** `<original>` and `<replacement>` are source text with each run of whitespace collapsed to one space.
- **I4** An edit outside a unit never changes the ids inside it.
- **I5** JSON and the text report carry each id's SHA-256, cut to its first 12 hex digits.

```covers
internal/mutantid
```
