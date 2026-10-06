# Declared mutants

## A guard that can't change the answer

Parse trims the spaces from the part of the line after the colon, so " -> " can never start at its first byte, and the arrow's index is never 0.

```equivalent
internal/mutantid.Parse: arrow < 0 -> arrow <= 0
```

## Which words a malformed line gets

A line that starts with ": " isn't an id either way. This change only swaps which of two error messages Parse gives for it, and the Contract leaves the wording of messages open.

```unpromised
internal/mutantid.Parse: colon < 0 -> colon <= 0
```
