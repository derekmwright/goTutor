package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// Every test runs in a synctest bubble: time is fake and advances only when
// every goroutine in the bubble is blocked, so a 10ms "fetch" costs nothing
// and durations are exact. If a goroutine you started is still blocked when
// the test function returns, the bubble panics with
//
//	deadlock: main bubble goroutine has exited but blocked goroutines remain
//
// and prints its stack. That is a leak: read the stack to see which send or
// receive it is parked on.

func recv[T any](ch <-chan T, d time.Duration) (v T, ok, timedOut bool) {
	select {
	case v, ok = <-ch:
		return v, ok, false
	case <-time.After(d):
		return v, false, true
	}
}

func drain[T any](t *testing.T, ch <-chan T, d time.Duration) []T {
	t.Helper()
	var got []T
	for {
		v, ok, timedOut := recv(ch, d)
		switch {
		case timedOut:
			t.Fatalf("channel not closed after %v (got %d values so far)", d, len(got))
		case !ok:
			return got
		}
		got = append(got, v)
	}
}

// expectClosedNow fails unless ch is closed right now. Call it after
// synctest.Wait, when everything that can make progress has.
func expectClosedNow[T any](t *testing.T, ch <-chan T, who string) {
	t.Helper()
	select {
	case v, ok := <-ch:
		if ok {
			t.Errorf("%s still had a sender parked on %v after cancel with no reader: that send needs an escape hatch", who, v)
			drain(t, ch, time.Second)
		}
	default:
		t.Errorf("%s neither exited nor offered a value after cancel; is it closing its channel?", who)
	}
}

func seq(n int) []int {
	vs := make([]int, n)
	for i := range vs {
		vs[i] = i
	}
	return vs
}

// double is a fn that costs 10ms of fake time per call and respects ctx.
func double(ctx context.Context, v int) (int, error) {
	select {
	case <-time.After(10 * time.Millisecond):
		return v * 2, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

var errBoom = errors.New("boom")

// failOn returns a fn like double that fails for one specific input.
func failOn(bad int) func(context.Context, int) (int, error) {
	return func(ctx context.Context, v int) (int, error) {
		if v == bad {
			return 0, fmt.Errorf("item %d: %w", v, errBoom)
		}
		return double(ctx, v)
	}
}

// ---------------------------------------------------------------------------
// Part 1: Gen
// ---------------------------------------------------------------------------

func TestGen_Values(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		got := drain(t, Gen(t.Context(), []int{3, 1, 2}), time.Second)
		if want := []int{3, 1, 2}; !slices.Equal(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
	})
}

func TestGen_Empty(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if got := drain(t, Gen(t.Context(), []int(nil)), time.Second); len(got) != 0 {
			t.Fatalf("got %v; want nothing", got)
		}
	})
}

func TestGen_CancelWithNoReader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ch := Gen(ctx, seq(100))
		if v, _, timedOut := recv(ch, time.Second); timedOut || v != 0 {
			t.Fatalf("first receive: v=%d timedOut=%v", v, timedOut)
		}
		cancel()
		synctest.Wait()
		expectClosedNow(t, ch, "Gen")
	})
}

// ---------------------------------------------------------------------------
// Part 2 and 4: Pool and PoolErrgroup share a contract, so share the tests.
// ---------------------------------------------------------------------------

type poolFunc = func(context.Context, <-chan int, int, func(context.Context, int) (int, error)) <-chan Result[int, int]

var pools = []struct {
	name string
	pool poolFunc
}{
	{"Pool", Pool[int, int]},
	{"PoolErrgroup", PoolErrgroup[int, int]},
}

func TestPool_ReturnsImmediately(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				in := make(chan int) // never fed, never closed
				done := make(chan struct{})
				go func() {
					defer close(done)
					p.pool(ctx, in, 2, double)
				}()
				if _, _, timedOut := recv(done, time.Second); timedOut {
					t.Fatal("pool did not return while its input was empty; it must not wait for workers")
				}
				cancel()
			})
		})
	}
}

func TestPool_AllResults(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				res := drain(t, p.pool(ctx, Gen(ctx, seq(20)), 4, double), time.Second)
				var outs []int
				for _, r := range res {
					if r.Err != nil {
						t.Errorf("unexpected error for %d: %v", r.In, r.Err)
					}
					if r.Out != r.In*2 {
						t.Errorf("Result{In:%d Out:%d}; want Out %d", r.In, r.Out, r.In*2)
					}
					outs = append(outs, r.Out)
				}
				slices.Sort(outs)
				want := make([]int, 20)
				for i := range want {
					want[i] = i * 2
				}
				if !slices.Equal(outs, want) {
					t.Fatalf("got %v; want %v (order may differ, values may not)", outs, want)
				}
			})
		})
	}
}

func TestPool_Bounded(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				const n, items = 4, 12
				var inFlight, maxInFlight atomic.Int32
				fn := func(ctx context.Context, v int) (int, error) {
					cur := inFlight.Add(1)
					defer inFlight.Add(-1)
					for {
						m := maxInFlight.Load()
						if cur <= m || maxInFlight.CompareAndSwap(m, cur) {
							break
						}
					}
					return double(ctx, v)
				}
				ctx := t.Context()
				start := time.Now()
				drain(t, p.pool(ctx, Gen(ctx, seq(items)), n, fn), time.Second)
				elapsed := time.Since(start)

				if got := maxInFlight.Load(); got > n {
					t.Errorf("saw %d calls in flight; the bound is %d", got, n)
				} else if got < n {
					t.Errorf("saw at most %d calls in flight with %d workers and %d items; the workers aren't running in parallel", got, n, items)
				}
				// 12 items, 4 at a time, 10ms each: three rounds of fake time.
				if want := 30 * time.Millisecond; elapsed != want {
					t.Errorf("took %v of fake time; want exactly %v (%d items / %d workers x 10ms)", elapsed, want, items, n)
				}
			})
		})
	}
}

func TestPool_ErrorRidesTheChannel(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				res := drain(t, p.pool(ctx, Gen(ctx, seq(6)), 2, failOn(3)), time.Second)
				if len(res) != 6 {
					t.Fatalf("got %d results; want 6: one error must not stop the pool", len(res))
				}
				var errs int
				for _, r := range res {
					if r.In == 3 {
						if !errors.Is(r.Err, errBoom) {
							t.Errorf("Result for 3: Err=%v; want errBoom", r.Err)
						}
						errs++
					} else if r.Err != nil {
						t.Errorf("Result for %d: unexpected Err=%v", r.In, r.Err)
					}
				}
				if errs != 1 {
					t.Errorf("got %d error results; want 1", errs)
				}
			})
		})
	}
}

func TestPool_CancelWhileWorking(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				slow := func(ctx context.Context, v int) (int, error) {
					select {
					case <-time.After(time.Hour):
						return v, nil
					case <-ctx.Done():
						return 0, ctx.Err()
					}
				}
				out := p.pool(ctx, Gen(ctx, seq(50)), 3, slow)
				synctest.Wait() // every worker is now inside slow
				cancel()
				synctest.Wait()
				// Workers may legitimately emit a Result{Err: context.Canceled}
				// on their way out; what they may not do is stay blocked.
				got := drain(t, out, time.Second)
				for _, r := range got {
					if !errors.Is(r.Err, context.Canceled) {
						t.Errorf("after cancel, got Result %+v; want only context.Canceled errors or nothing", r)
					}
				}
			})
		})
	}
}

func TestPool_CancelWithNoReader(t *testing.T) {
	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				fast := func(_ context.Context, v int) (int, error) { return v, nil }
				out := p.pool(ctx, Gen(ctx, seq(50)), 3, fast)
				synctest.Wait() // workers are parked on sends nobody is reading
				cancel()
				synctest.Wait()
				expectClosedNow(t, out, p.name)
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Part 3: Collect, and the whole pipe end to end
// ---------------------------------------------------------------------------

func TestCollect_Clean(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		got, err := Collect(ctx, cancel, Pool(ctx, Gen(ctx, seq(10)), 3, double))
		if err != nil {
			t.Fatalf("Collect error: %v", err)
		}
		slices.Sort(got)
		want := make([]int, 10)
		for i := range want {
			want[i] = i * 2
		}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}
		if ctx.Err() != nil {
			t.Errorf("ctx was cancelled on a clean run: %v", ctx.Err())
		}
	})
}

func TestCollect_FirstErrorStopsThePipeline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var calls atomic.Int32
		fn := func(ctx context.Context, v int) (int, error) {
			calls.Add(1)
			return failOn(5)(ctx, v)
		}
		got, err := Collect(ctx, cancel, Pool(ctx, Gen(ctx, seq(1000)), 2, fn))
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v; want errBoom", err)
		}
		if ctx.Err() == nil {
			t.Error("Collect returned an error but did not cancel the pipeline")
		}
		if len(got) > 10 {
			t.Errorf("Collect returned %d results; the error was item 5, so it should have stopped long before 1000", len(got))
		}
		synctest.Wait()
		if c := calls.Load(); c > 20 {
			t.Errorf("fn was called %d times after an error at item 5; upstream kept running", c)
		}
		// Leaving the bubble checks that Gen, the workers and the closer have
		// all exited: Collect must drain in until it is closed.
	})
}

func TestCollect_ErrorIsNotSwallowedByCancel(t *testing.T) {
	// The first error must be the one fn returned, not the context.Canceled
	// that workers emit after Collect cancels them.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		_, err := Collect(ctx, cancel, Pool(ctx, Gen(ctx, seq(100)), 8, failOn(0)))
		if !errors.Is(err, errBoom) || errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v; want the original errBoom, not a cancellation", err)
		}
	})
}
