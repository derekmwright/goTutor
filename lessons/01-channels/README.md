# 01 · Channel foundations

## The model

An **unbuffered channel is a rendezvous**: it has no storage, so a send
finishes only when a receiver is taking the value at that same moment.
Blocking is how goroutines synchronize, and it's normal.

```
unbuffered                        buffered, cap 2
 sender ──► [ ] ──► receiver       sender ──► [ x | x ] ──► receiver
 send blocks until a receiver      send blocks only when full;
 is there; the two meet            receive blocks only when empty
```

What it costs you when it goes wrong:

| situation | result |
|---|---|
| every goroutine blocked, none can wake another | `fatal error: all goroutines are asleep - deadlock!` (not a panic, so `recover` can't catch it) |
| one goroutine blocked forever, others still running | **a leak**: nothing reports it; it holds memory until the process exits |

**Goroutines have no parent.** When the function that started one returns,
nothing happens to the goroutine, and you can't kill a goroutine from outside.
`context` doesn't kill anything either. It's a signal (a channel,
`ctx.Done()`, that gets closed) which the goroutine has to *choose* to watch.

### Channel operations at a glance

| operation | nil channel | open channel | closed channel |
|---|---|---|---|
| `ch <- v` | blocks forever | blocks until received (or buffer space) | **panic** |
| `<-ch` | blocks forever | blocks until a value arrives | returns zero value immediately, `ok == false` |
| `close(ch)` | **panic** | closes it | **panic** |

Two consequences you'll use today:

- A closed channel is **always ready** to receive from. Leave a closed channel
  in a `select` and that case wins over and over.
- A nil channel is **never ready**. Setting a channel variable to `nil`
  switches its `select` case off.

### Ownership rules

1. The **sender owns** the channel: it creates it, sends on it, closes it.
2. Close **exactly once**, and only to say "no more values". Receivers never
   close.
3. Hand out a `<-chan T` so the compiler stops consumers from sending or
   closing.
4. Every blocking send or receive in a goroutine needs an **escape hatch**
   (usually `case <-ctx.Done():` in a `select`).

## The task

Edit `channels.go`. Each `TODO(you)` has a stub that compiles.

1. **`Generate`**: a producer that owns its channel, closes it on exit, and
   stops when `ctx` is done, *even if the consumer has walked away*.
2. **`FirstOf`**: fan out to N backends and return the first answer, a
   timeout error, or a cancellation error. Cancel the losers, and make sure no
   goroutine you started can block forever, even when a backend ignores
   `ctx`. This is the classic "abandoned sender" bug, fixed properly.
3. **`Merge`**: fan in two channels with one goroutine running one loop
   (more than one `select` per iteration is fine). Tell closed from zero, switch off finished inputs, close `out` when
   both are done, and don't block on `out` after cancellation.

Tests re-run each time you save (`<Space>t` in the editor, or
`task test -- lessons/01-channels`). You may read `channels_test.go`; you
shouldn't need to edit it.

### About the tests

They run inside `testing/synctest` bubbles, which use **fake time**. A
"2 second" backend finishes instantly, and `time.Since` returns exact fake
durations, so the tests can check that `FirstOf` took *exactly* 20ms (which
proves the backends ran concurrently).

If a goroutine you started is still blocked when a test ends, you'll see:

```
panic: deadlock: main bubble goroutine has exited but blocked goroutines remain
```

followed by the stuck goroutine's stack. **That's your leak detector.** Read
the stack to find the send or receive it's parked on. The panic stops the test
binary, so tests after it won't run until you fix it.

If a test hangs until the 20s timeout, a goroutine is probably **spinning**
instead of blocking (look at the closed-channel row in the table above).

## Objectives

- Say who owns a channel and who closes it, and enforce that with types.
- Write a goroutine whose every blocking operation can escape on cancellation.
- Prevent the "abandoned sender" leak, and explain more than one way to fix it.
- Use `v, ok := <-ch` and nil channels to drive a `select` loop to completion.
- Read a synctest deadlock report and find the leaking line.

## Stretch goals

- `MergeSelect` (in `merge_select.go`): the same behaviour with exactly **one**
  `select` per iteration, by switching cases on and off with nil channels. The
  `Merge` tests run against it too.

- `MergeN[T any](ctx, chans ...<-chan T) <-chan T`: one goroutine per input
  plus a `sync.WaitGroup` (look at `wg.Go` in Go 1.25). Who closes `out`, and
  when? Compare the result with the single-`select` version.
- In `FirstOf`, return the first **non-empty** answer and ignore empty ones,
  which turns the "first wins" `select` into a loop. Does your leak fix still hold?
- Run `go test -race -bench . -benchmem` after adding a benchmark that pulls
  1M values from `Generate`. How much does each unbuffered hand-off cost? What
  changes with a buffer of 64?

## Further reading

- [The Go Memory Model](https://go.dev/ref/mem): the "Channel communication"
  section, including what close and buffered capacity guarantee.
- [Go spec: Select statements](https://go.dev/ref/spec#Select_statements):
  how a ready case is chosen ("uniform pseudo-random selection").
- [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [`testing/synctest`](https://pkg.go.dev/testing/synctest) and the
  [synctest blog post](https://go.dev/blog/synctest)
