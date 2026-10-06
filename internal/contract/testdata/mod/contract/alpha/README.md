# alpha

Alpha does alpha things.

- **A1** The first promise.
- **A2** The second promise.
  - **A9** An indented item, which isn't an obligation.
* **A8** A star bullet, which isn't one either.
- **a7** Lowercase, which isn't one either.

```covers
# the whole package
internal/alpha

internal/beta.F
```

~~~~ text
- **A6** Inside a tilde fence.
~~~
Still inside, because that closing fence is shorter.
~~~~

<!--
```covers
dead/inside/comment
```
- **A5** Inside a comment.
-->

- **A3** After the comment. <!-- ```covers -->

```equivalent
internal/alpha.F: a -> b
```
