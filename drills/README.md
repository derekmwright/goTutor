# Drills

Pure-recall katas, 5 to 10 minutes each. A signature, a two-line spec, and
tests: no hints, no notes. Write it from memory, make it pass under `-race`,
and do it again a few days later. Lessons build understanding; drills make the
patterns automatic.

```
task drill:list               what's there, how often, how fast
task drill -- merge           reset to the blank stub, open the editor, start the clock
task drill:check -- merge     run the tests; on pass, log the time to log.tsv
```

| drill | pattern | from |
|---|---|---|
| `generator` | producer owns and closes; send inside `select` with `ctx.Done()` | 01 |
| `firstof` | fan out, first answer wins; buffer to N senders; `ctx.Err()` | 01 |
| `merge` | two-input fan-in; `v, ok`; nil a finished input | 01 |
| `mergeselect` | the same with exactly one `select`: a nil-channel state machine | 01 |
| `ordone` | forward one channel until closed or cancelled, without leaking | 01 |
| `breakselect` | stop a `for`/`select` loop from inside a case (`break` only leaves the `select`) | 01 |

Tests run in `testing/synctest` bubbles, so a leaked goroutine fails loudly:
`deadlock: main bubble goroutine has exited but blocked goroutines remain`.

Each drill is `drill.go` (yours), `drill.go.stub` (the blank copy that
`task drill` restores), and `drill_test.go`. Shared test helpers live in
`internal/chantest`.
