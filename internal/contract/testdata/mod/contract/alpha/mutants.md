Prose before any heading.

```unpromised
internal/alpha.F: *
```

## Logging is open ##

The Contract leaves log text open.
It says nothing   about it.

```unpromised
internal/alpha.Log: *

# a comment line
internal/alpha.Log: "a" -> "b"
```

Setext heading
--------------

Equivalence here.

````equivalent
internal/alpha.F: i < n -> i != n
````

After the block, prose starts over.

```equivalent
internal/alpha.G: x -> y
```

```covers
internal/alpha
```
