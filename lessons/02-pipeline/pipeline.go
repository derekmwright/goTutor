// Package pipeline: stages that own their output channel, a bounded worker
// pool, and error propagation up and down the pipe.
package pipeline

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Result is one output of a worker: the input it came from, the output, and
// the error if fn failed. Errors ride the channel like any other value so the
// consumer decides what to do about them.
type Result[In, Out any] struct {
	In  In
	Out Out
	Err error
}

// Gen sends each element of vs on the returned channel, in order, then closes
// it. It stops early, and still closes, if ctx is done.
func Gen[T any](ctx context.Context, vs []T) <-chan T {
	out := make(chan T)
	// TODO(you): a goroutine that owns out: sends each v with a ctx.Done()
	// escape hatch and closes out on every exit path.
	go func() {
		defer close(out)

		for _, v := range vs {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

// Pool starts n workers that each read from in, call fn, and send the Result
// on the returned channel. The channel is closed once all workers have
// exited. Pool returns immediately.
//
// A failing fn does not stop the pool: the Result carrying Err is sent and
// the worker moves on. Every send and receive must be cancellable by ctx.
func Pool[In, Out any](ctx context.Context, in <-chan In, n int, fn func(context.Context, In) (Out, error)) <-chan Result[In, Out] {
	out := make(chan Result[In, Out])
	// TODO(you): n workers (for/select over in; call fn; send Result), a
	// WaitGroup counted by the dispatcher, and a closer goroutine that waits
	// and closes out. Think about who may call close, and when.
	wg := sync.WaitGroup{}
	wg.Add(n)

	for range n {

	}

	close(out)
	return out
}

// Collect drains in and returns the Outs in arrival order. On the first
// Result with a non-nil Err it calls cancel, returns what it has so far and
// that error. In both cases, by the time Collect returns, nothing upstream
// is still blocked: it must keep reading until in is closed.
func Collect[In, Out any](ctx context.Context, cancel context.CancelFunc, in <-chan Result[In, Out]) ([]Out, error) {
	// TODO(you): range over in; on first Err, cancel and remember it; keep
	// draining so upstream goroutines can exit; return.
	return nil, nil
}

// PoolErrgroup has the same contract as Pool but is built on errgroup: one
// g.Go per input item, SetLimit(n) for the bound, and the group's context for
// cancellation. Stop reading in (and return) when the group's ctx is done.
func PoolErrgroup[In, Out any](ctx context.Context, in <-chan In, n int, fn func(context.Context, In) (Out, error)) <-chan Result[In, Out] {
	out := make(chan Result[In, Out])
	g, gctx := errgroup.WithContext(ctx)
	_ = g
	_ = gctx
	// TODO(you): g.SetLimit(n); a dispatcher goroutine that ranges over in and
	// calls g.Go for each item (fn, then send the Result on out with an escape
	// hatch); after the range, g.Wait() then close(out). Return out now.
	close(out)
	return out
}
