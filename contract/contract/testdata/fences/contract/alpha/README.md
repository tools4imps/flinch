# alpha

- **F1** before any fence

```
- **F2** inside a fence with no info string
```

~~~~ covers
internal/a
~~~
`````
internal/b
~~~~~

   ```covers extra words
internal/c
   ```

    ```covers
- **F3** four spaces of indent open no fence
    ```

``` covers`inside
- **F4** a backtick in a backtick fence's info string opens no fence

~~~ go`inside
- **F5** a backtick in a tilde fence's info string is fine
~~~

<!--
- **F6** inside an HTML comment
```covers
internal/d
```
-->
```covers
internal/m
```

<!---->
```covers
internal/k
```

- **F9** holds, and a comment opens after it <!-- a note
that runs on -->

```text
<!--
```
- **F7** a comment opener inside a fence is only text

````covers
internal/e
```
```` with words
internal/f
````

```Covers
internal/g
```

```coverslike
internal/h
```

```  covers
internal/i
```

```covers
internal/j
- **F8** inside a fence that never closes
