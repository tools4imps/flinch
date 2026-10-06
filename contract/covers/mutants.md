# covers mutants

## The ownership map never holds an empty directory

Resolve adds a directory to the ownership map only while it gives that directory a unit, so every entry has at least one.

```equivalent
internal/covers.Ownership.Covered: len(tops) > 0 -> len(tops) >= 0
```

## A reference claims its init functions one at a time

The init functions a reference names are claimed one by one for the same primitive at the same level, so their order changes nothing.

```equivalent
internal/covers.resolve: sort.Strings(inits) -> (removed)
```

## A dot right after the last slash names no package either way

When a reference has a dot right after its last slash, or starts with one, split gives a directory that ends in a slash or is empty, and the mutant keeps the whole reference as the directory instead. No package has any of those directories, since flinch skips names that start with a dot, and the error quotes the reference itself, so both read the same.

```equivalent
internal/covers.split: dot < 0 -> dot <= 0
```

## A variable without an initializer has nothing to own

Giving a spec with no initializer a unit name changes nothing, because no unit has that name in a package that compiles, and the only place ownership shows is a run, which needs a package that compiles.

```equivalent
internal/covers.index: len(vs.Values) > 0 -> len(vs.Values) >= 0
```

## A line that starts with a comment never names the module

A go.mod line that starts with `//` is all comment. Left whole, its first field starts with `//` and never reads `module`.

```equivalent
internal/source.modulePath: i >= 0 -> i > 0
```

## Rel can't fail under the root

filepath.Rel can't fail for a path that WalkDir found under the root it was given.

```equivalent
internal/source.Dirs.func1: return err -> return nil #2
```

## ReadDir already sorts

os.ReadDir returns a directory's entries sorted by name, and every file name gets the same directory in front, so these lists are sorted before the sort runs.

```equivalent
internal/source.readPackage: sort.Strings(p.GoFiles) -> (removed)
internal/source.readPackage: sort.Strings(p.TestFiles) -> (removed)
internal/source.readPackage: sort.Strings(p.Mutable) -> (removed)
```

## Parse reads the type names back as keys

Parse collects the package's type names as the keys of a map, so the value it stores under each key never matters.

```equivalent
internal/source.Parse: true -> false
```

## The order of these lists is open

flinch reads the covered directories and a package's type names as sets, and sorts the outside list itself, so the Contract leaves the order of Ownership.Covered, Parsed.Types and the module's packages open.

```unpromised
internal/covers.Ownership.Covered: sort.Strings(out) -> (removed)
internal/source.Parse: sort.Strings(out.Types) -> (removed)
internal/source.Dirs: sort.Strings(dirs) -> (removed)
```

## A package's name is open

Package.Name records the package clause. No part of flinch reads it, and the Contract promises nothing about it.

```unpromised
internal/source.readPackage: testName == "" -> testName != ""
internal/source.readPackage: p.Name == "" -> p.Name != ""
internal/source.readPackage: p.Name == "" -> p.Name != "" #2
```

## A missing working directory is open

FindModule fails on filepath.Abs only when the working directory can't be found, which even a deleted one doesn't cause on macOS. Without that error the next step fails to read a module with no root, so flinch still exits 2, with another message.

```unpromised
internal/source.FindModule: return Module{}, err -> return Module{}, nil
```

## A module that changes while flinch reads it is open

readPackage reads a directory that Dirs walked a moment before, so it fails only when the directory changes in between. The Contract promises nothing about a module that changes while flinch reads it.

```unpromised
internal/source.readPackage: return p, err -> return p, nil
```
