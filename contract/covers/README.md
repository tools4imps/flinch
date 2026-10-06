# covers

covers reads the module's packages and decides which primitive owns each function, method and package-level variable.

## Obligations

- **V1** A `covers` line `dir` names the package in that directory, relative to the module root, and `.` names the root package. `dir/...` adds every package below it. Like `go list ./...`, flinch leaves out `testdata` and `vendor` directories, names starting with `.` or `_`, and nested modules.
- **V2** `dir.Name` names a function, a type with all its methods, or the package-level variable spec that declares Name. `dir.init` names the package's init functions and `dir.init.N` one of them. `dir.Type.Method` names one method, with the receiver written without `*`. A unit in the root package is written `..Name`, the way mutant ids write it.
- **V3** Each function, method and package-level variable belongs to the primitive with the most specific matching reference: a method first, then a function or type, then a package, then `...`. A tie goes to the primitive that sorts first by name.
- **V4** A reference that names nothing in the module is an error.
- **V5** `_test.go` files, files marked `// Code generated ... DO NOT EDIT.` and files that import `"C"` are never covered.
- **V6** Module packages no reference covers are listed as outside the Contract, sorted by directory, apart from the Contract's own directories. They never fail the build.
- **V7** The module root is the nearest directory at or above the working directory whose go.mod holds a module line. When there is none, or a package's directory or files can't be read or parsed, flinch stops with exit 2.

```covers
internal/covers
internal/source
```
