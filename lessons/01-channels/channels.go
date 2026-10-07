// Package channels drills the foundations of Go channels: who owns a channel,
// who closes it, how a goroutine learns it should stop, and how select treats
// closed and nil channels.
//
// Three rules to keep in your head while you work:
//
//  1. The sender owns the channel. Only the sender closes it, and only once.
//  2. Every goroutine you start needs a guaranteed way to finish. Nothing
//     kills it for you: not the function that started it returning, not a
//     cancelled context. It has to notice and return.
//  3. Any send or receive that could block forever needs an escape hatch,
//     usually a select with <-ctx.Done().
package channels

import (
	"context"
	"errors"
	"time"
)

// ---------------------------------------------------------------------------
// Part 1: a producer that owns its channel
// ---------------------------------------------------------------------------

// Generate starts a goroutine that sends start, start+1, start+2, ... on the
// returned channel, one value per receive, until ctx is done. When the
// goroutine exits it closes the channel, so a consumer can `for v := range`
// over it and the loop ends after cancellation.
//
// Generate must not leak: if the consumer cancels ctx and walks away without
// reading another value, the goroutine must still exit and close the channel.
func Generate(ctx context.Context, start int) <-chan int {
	// TODO(you): make the channel, start the producer goroutine, return the
	// channel. The goroutine must:
	//   - close the channel exactly once when it exits (what's the idiom that
	//     guarantees that on every return path?)
	//   - never block on a send that can't escape when ctx is done
	// Why is the return type <-chan int and not chan int? Who is that for?
	return nil
}

// ---------------------------------------------------------------------------
// Part 2: first response wins, nobody leaks
// ---------------------------------------------------------------------------

// Backend answers query q. A well-behaved backend returns early when ctx is
// done, but FirstOf must not rely on that: some backends ignore ctx and
// return whenever they finish.
type Backend func(ctx context.Context, q string) string

// ErrNoBackends is returned by FirstOf when it is given no backends.
var ErrNoBackends = errors.New("channels: no backends")

// FirstOf queries every backend concurrently and returns the first answer.
//
//   - If timeout elapses first it returns an error that satisfies
//     errors.Is(err, context.DeadlineExceeded).
//   - If the parent ctx is cancelled first it returns an error that satisfies
//     errors.Is(err, context.Canceled).
//   - When FirstOf returns, the backends that are still running must see their
//     ctx cancelled, so the well-behaved ones can stop working.
//   - No goroutine FirstOf started may stay blocked forever, even when a
//     backend ignores ctx and finishes long after FirstOf has returned.
func FirstOf(ctx context.Context, timeout time.Duration, q string, backends ...Backend) (string, error) {
	// TODO(you): handle the no-backends case, derive a ctx with the timeout,
	// fan out one goroutine per backend, and select on the first answer vs.
	// ctx.Done(). Two questions to answer before you type:
	//   - when you return, how do the slow backends find out?
	//   - a backend that ignores ctx finishes 2s after you've returned and
	//     tries to hand you its answer. What guarantees that hand-off can't
	//     block forever? (There's more than one fix. Pick one and say why.)
	return "", ErrNoBackends
}

// ---------------------------------------------------------------------------
// Part 3: fan-in with closed and nil channels
// ---------------------------------------------------------------------------

// Merge forwards every value it receives from a and b to the returned
// channel, in whatever order they arrive. It closes the returned channel once
// both a and b are closed and drained, or once ctx is done, whichever comes
// first. Merge doesn't close a or b; it doesn't own them.
//
// Use a single goroutine running one loop; more than one select per iteration
// is fine. (A goroutine per input plus a sync.WaitGroup also works; that's a
// stretch goal, and so is doing it with exactly one select: see MergeSelect.)
func Merge[T any](ctx context.Context, a, b <-chan T) <-chan T {
	// TODO(you): one goroutine, one loop (any number of selects). Work out:
	//   - how do you tell "received a zero value" from "the channel is closed"?
	//   - once a is closed, a receive from it is always ready. What happens to
	//     your select loop then, and what can you set a to so that select
	//     ignores that case? (Hint: what does a receive from a nil channel do?)
	//   - how do you know you're done?
	//   - forwarding a value is a send. What if nobody is reading out?
	return nil
}
