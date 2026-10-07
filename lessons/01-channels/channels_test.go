package channels

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// Every test runs inside a synctest bubble. Time in the bubble is fake: it
// only advances when every goroutine in the bubble is blocked, so a backend
// that "sleeps for 2s" finishes instantly, and time.Since measures exact fake
// durations.
//
// If a goroutine started in the bubble is still blocked when the test
// function returns, synctest.Test panics with
//
//	deadlock: main bubble goroutine has exited but blocked goroutines remain
//
// and prints the stuck goroutine's stack. That means a goroutine leak: read
// the stack to see which send or receive it's stuck on.

// recv receives one value from ch, giving up after d of fake time.
func recv[T any](ch <-chan T, d time.Duration) (v T, ok, timedOut bool) {
	select {
	case v, ok = <-ch:
		return v, ok, false
	case <-time.After(d):
		return v, false, true
	}
}

// drain reads ch until it is closed, failing if that takes longer than d.
func drain[T any](t *testing.T, ch <-chan T, d time.Duration) []T {
	t.Helper()
	var got []T
	for {
		v, ok, timedOut := recv(ch, d)
		switch {
		case timedOut:
			t.Fatalf("channel not closed after %v (got %v so far)", d, got)
		case !ok:
			return got
		}
		got = append(got, v)
	}
}

// ---------------------------------------------------------------------------
// Part 1: Generate
// ---------------------------------------------------------------------------

func TestGenerate_Values(t *testing.T) {
	for _, start := range []int{0, 10, -3} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			ch := Generate(ctx, start)
			for i := range 5 {
				v, ok, timedOut := recv(ch, time.Second)
				if timedOut || !ok {
					t.Fatalf("start=%d: receive #%d: closed=%v timedOut=%v; want value %d",
						start, i, !ok && !timedOut, timedOut, start+i)
				}
				if v != start+i {
					t.Fatalf("start=%d: receive #%d = %d; want %d", start, i, v, start+i)
				}
			}
			cancel()
			drain(t, ch, time.Second) // avoid tripping the leak check in this test
		})
	}
}

func TestGenerate_ClosesAfterCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ch := Generate(ctx, 0)
		if _, ok, _ := recv(ch, time.Second); !ok {
			t.Fatal("channel closed before cancel")
		}
		cancel()
		// A consumer ranging over ch must terminate. A few values may still
		// arrive after cancel (why? how many?), but the channel has to close.
		drain(t, ch, time.Second)
	})
}

func TestGenerate_NoLeakWhenConsumerWalksAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ch := Generate(ctx, 0)
		if _, ok, _ := recv(ch, time.Second); !ok {
			t.Fatal("channel closed before cancel")
		}

		// Cancel and walk away: no further receives. Let the producer react.
		cancel()
		synctest.Wait()

		// If the producer exited, ch is closed and this receive is !ok
		// immediately. If it is still parked on a plain `ch <- v`, this
		// receive takes that value, which proves it was stuck.
		select {
		case v, ok := <-ch:
			if ok {
				t.Errorf("producer was still blocked sending %d after cancel with no reader: "+
					"that send has no escape hatch, so it leaks if nobody reads", v)
				drain(t, ch, time.Second)
			}
		default:
			t.Error("producer neither exited nor offered a value after cancel; is it closing the channel?")
		}
	})
}

// ---------------------------------------------------------------------------
// Part 2: FirstOf
// ---------------------------------------------------------------------------

// sleepy is a backend that ignores ctx entirely: it always takes d, then
// returns name. FirstOf has to cope with backends like this.
func sleepy(name string, d time.Duration) Backend {
	return func(_ context.Context, _ string) string {
		time.Sleep(d)
		return name
	}
}

// polite is a backend that stops early when ctx is done and counts how many
// times that happened.
func polite(name string, d time.Duration, cancelled *atomic.Int32) Backend {
	return func(ctx context.Context, _ string) string {
		select {
		case <-time.After(d):
			return name
		case <-ctx.Done():
			cancelled.Add(1)
			return ""
		}
	}
}

func TestFirstOf(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		backends []Backend
		want     string
		wantErr  error
		wantTook time.Duration
	}{
		{
			name:     "single backend",
			timeout:  time.Second,
			backends: []Backend{sleepy("a", 10*time.Millisecond)},
			want:     "a",
			wantTook: 10 * time.Millisecond,
		},
		{
			name:    "fastest wins and backends run concurrently",
			timeout: time.Second,
			backends: []Backend{
				sleepy("slow", 300*time.Millisecond),
				sleepy("fast", 20*time.Millisecond),
				sleepy("medium", 100*time.Millisecond),
			},
			want:     "fast",
			wantTook: 20 * time.Millisecond,
		},
		{
			name:    "timeout, and the slow backends ignore ctx (leak check)",
			timeout: 100 * time.Millisecond,
			backends: []Backend{
				sleepy("a", 2*time.Second),
				sleepy("b", 3*time.Second),
			},
			wantErr:  context.DeadlineExceeded,
			wantTook: 100 * time.Millisecond,
		},
		{
			name:     "no backends",
			timeout:  time.Second,
			wantErr:  ErrNoBackends,
			wantTook: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				got, err := FirstOf(t.Context(), tt.timeout, "q", tt.backends...)
				took := time.Since(start)

				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v; want %v", err, tt.wantErr)
				}
				if got != tt.want {
					t.Errorf("got %q; want %q", got, tt.want)
				}
				if took != tt.wantTook {
					t.Errorf("took %v of fake time; want %v", took, tt.wantTook)
				}

				// Let every ctx-ignoring backend finish and try to report back.
				// Any goroutine stuck on that hand-off is a leak, and
				// synctest.Test will panic when this function returns.
				time.Sleep(time.Hour)
			})
		})
	}
}

func TestFirstOf_ParentCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(50*time.Millisecond, cancel)

		start := time.Now()
		_, err := FirstOf(ctx, time.Second, "q", sleepy("a", 2*time.Second))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v; want context.Canceled", err)
		}
		if took := time.Since(start); took != 50*time.Millisecond {
			t.Errorf("took %v; want 50ms (return as soon as the parent is cancelled)", took)
		}
		time.Sleep(time.Hour)
	})
}

func TestFirstOf_CancelsLosers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cancelled atomic.Int32
		got, err := FirstOf(t.Context(), time.Second, "q",
			polite("fast", 10*time.Millisecond, &cancelled),
			polite("slow1", 500*time.Millisecond, &cancelled),
			polite("slow2", 800*time.Millisecond, &cancelled),
		)
		if err != nil || got != "fast" {
			t.Fatalf("FirstOf = %q, %v; want \"fast\", nil", got, err)
		}

		// Without advancing time, let the losers react to cancellation.
		synctest.Wait()
		if n := cancelled.Load(); n != 2 {
			t.Errorf("%d of 2 losing backends saw ctx cancelled when FirstOf returned; "+
				"want 2 (who cancels the ctx you gave them?)", n)
		}
		time.Sleep(time.Hour)
	})
}

// ---------------------------------------------------------------------------
// Part 3: Merge
// ---------------------------------------------------------------------------

// feed sends vs on a new channel, one every gap, then closes it. It gives up
// when ctx is done, so a Merge that stops reading can't make feed leak.
func feed(ctx context.Context, gap time.Duration, vs ...int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for _, v := range vs {
			select {
			case <-time.After(gap):
			case <-ctx.Done():
				return
			}
			select {
			case ch <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// mergeImpls runs every Merge test against both implementations.
var mergeImpls = []struct {
	name  string
	merge func(ctx context.Context, a, b <-chan int) <-chan int
}{
	{"Merge", Merge[int]},
	{"MergeSelect", MergeSelect[int]},
}

func TestMerge(t *testing.T) {
	for _, impl := range mergeImpls {
		t.Run(impl.name, func(t *testing.T) { testMerge(t, impl.merge) })
	}
}

func testMerge(t *testing.T, Merge func(ctx context.Context, a, b <-chan int) <-chan int) {
	tests := []struct {
		name string
		a, b func(context.Context) <-chan int
		want []int
	}{
		{
			name: "both inputs interleaved",
			a:    func(ctx context.Context) <-chan int { return feed(ctx, 10*time.Millisecond, 1, 2, 3) },
			b:    func(ctx context.Context) <-chan int { return feed(ctx, 15*time.Millisecond, 10, 20) },
			want: []int{1, 2, 3, 10, 20},
		},
		{
			name: "a closes immediately, b keeps going",
			a:    func(ctx context.Context) <-chan int { return feed(ctx, 0) },
			b:    func(ctx context.Context) <-chan int { return feed(ctx, 50*time.Millisecond, 7, 8, 9) },
			want: []int{7, 8, 9},
		},
		{
			name: "zero values are values, not closes",
			a:    func(ctx context.Context) <-chan int { return feed(ctx, 10*time.Millisecond, 0, 0) },
			b:    func(ctx context.Context) <-chan int { return feed(ctx, 25*time.Millisecond, 0) },
			want: []int{0, 0, 0},
		},
		{
			name: "both empty",
			a:    func(ctx context.Context) <-chan int { return feed(ctx, 0) },
			b:    func(ctx context.Context) <-chan int { return feed(ctx, 0) },
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel() // t.Context is cancelled too late for the bubble's leak check
				got := drain(t, Merge(ctx, tt.a(ctx), tt.b(ctx)), time.Second)
				slices.Sort(got)
				if !slices.Equal(got, tt.want) {
					t.Errorf("got %v; want %v (sorted)", got, tt.want)
				}
			})
		})
	}
}

func TestMerge_CancelWhileWaitingForInput(t *testing.T) {
	for _, impl := range mergeImpls {
		t.Run(impl.name, func(t *testing.T) { testMerge_CancelWhileWaitingForInput(t, impl.merge) })
	}
}

func testMerge_CancelWhileWaitingForInput(t *testing.T, Merge func(ctx context.Context, a, b <-chan int) <-chan int) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		a, b := make(chan int), make(chan int) // never sent on, never closed
		out := Merge(ctx, a, b)

		time.AfterFunc(10*time.Millisecond, cancel)
		if got := drain(t, out, time.Second); len(got) != 0 {
			t.Errorf("got %v; want nothing", got)
		}
	})
}

func TestMerge_CancelWhileConsumerGone(t *testing.T) {
	for _, impl := range mergeImpls {
		t.Run(impl.name, func(t *testing.T) { testMerge_CancelWhileConsumerGone(t, impl.merge) })
	}
}

func testMerge_CancelWhileConsumerGone(t *testing.T, Merge func(ctx context.Context, a, b <-chan int) <-chan int) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		a := make(chan int, 3) // buffered and never closed: plenty to forward
		a <- 1
		a <- 2
		a <- 3
		out := Merge(ctx, a, make(chan int))

		if v, ok, _ := recv(out, time.Second); !ok || v != 1 {
			t.Fatalf("first receive = %v, %v; want 1, true", v, ok)
		}

		// Stop reading, then cancel. Merge is now trying to forward 2.
		synctest.Wait()
		cancel()
		synctest.Wait()

		select {
		case v, ok := <-out:
			if ok {
				t.Errorf("Merge was still blocked forwarding %d after cancel with no reader: "+
					"that send needs an escape hatch", v)
				drain(t, out, time.Second)
			}
		default:
			t.Error("Merge neither exited nor offered a value after cancel; is it closing out?")
		}
	})
}
