# 02 · Pipelines and bounded parallelism

## The model

A **stage** is a function that owns its output channel: it takes `ctx` and an
input channel, starts goroutines, and returns `<-chan T` *immediately*. The
goroutines read input until it closes, send results, and close the output on
the way out. A pipeline is stages patched together:

```
 urls := Gen(ctx, list)                 slice ──► chan
 res  := Pool(ctx, urls, 8, fetch)      chan  ──► chan, 8 workers
 for r := range res { ... }             the consumer
```

Think audio patch bays: every stage has the same shaped out jack (`<-chan T`),
so anything can be patched into anything, and the plug only goes one way.

### Bounding = N copies of a stage

Each worker reads from the **same** input channel; the runtime hands each
value to exactly one of them. The channel is the work queue and you get load
balancing for free. The problem is on the *output* side:

```
            ┌─ worker 1 ─┐
 in ──────► ├─ worker 2 ─┼──► out       who closes out?
            └─ worker 3 ─┘
```

If worker 1 closes `out`, worker 2's next send panics. So:

1. the dispatcher counts workers in a `sync.WaitGroup` (`Add` *before* `go`;
   or `wg.Go`, which does both);
2. one extra goroutine, the **closer**, does `wg.Wait(); close(out)`;
3. `Pool` returns `out` right away so the next stage can be patched in.

### Errors

Channels carry values, so a failing worker has three choices: send a result
with an `Err` field downstream, cancel a shared context so everyone stops, or
both. `golang.org/x/sync/errgroup` packages "both" for you: a WaitGroup that
records the first error and cancels a derived context, and `SetLimit(n)` gives
you the bound without a channel at all.

After a cancelled run the consumer's `range` just ends, exactly as it would on
a clean finish. To tell them apart the consumer checks `ctx.Err()` (or, with
errgroup, `g.Wait()` returns the first error).

## The task

Edit `pipeline.go`. Each `TODO(you)` has a stub that compiles.

1. **`Gen`**: slice → channel. Owns and closes its channel, bails on `ctx`.
2. **`Pool`**: `n` workers over `in`, applying `fn` to each value and sending
   the `Result`. WaitGroup-closed output. Every send and receive has an
   escape hatch. On `fn` error, send the `Result` carrying `Err` downstream
   and keep going; deciding to stop is the consumer's job.
3. **`Collect`**: drain `in` into a slice. On the first `Result` with
   `Err != nil`, return `(got so far, err)`; the caller's `cancel` is passed
   in so you can stop the upstream stages. Must not leave *anything* running.
4. **`PoolErrgroup`**: the same contract as `Pool` using `errgroup.Group`
   with `SetLimit`. Note how different the shape is: no worker loop, one
   goroutine per *item*, and the group's context does the cancelling.

## Objectives

- Write a stage that returns its channel immediately and closes it exactly once.
- Count workers with a WaitGroup, closed by a dedicated closer goroutine.
- Keep every blocking channel op cancellable; prove it with `synctest`.
- Propagate an error upstream via `cancel` and downstream via a `Result`.
- Compare the hand-rolled pool to `errgroup.SetLimit`.

## Stretch

- `BenchmarkPool` with `n` in {1, 2, 4, 8, 32} and a `fn` that sleeps 1ms:
  where does adding workers stop helping, and why?
- Preserve input order in `Pool`'s output (hint: a per-item channel of
  channels, or an index + reorder stage).
- Make `Pool` stop *early* on the first error instead of leaving it to
  `Collect`. What does that cost in coupling?

## Further reading

- Go blog, *Go Concurrency Patterns: Pipelines and cancellation*
  (https://go.dev/blog/pipelines)
- `golang.org/x/sync/errgroup` docs, especially `SetLimit` and `TryGo`
- Go 1.25 release notes: `sync.WaitGroup.Go`
- `testing/synctest` package docs
