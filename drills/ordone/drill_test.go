package ordone

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"gotutor/drills/internal/chantest"
)

func TestForwardsUntilClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		got := chantest.Drain(t, OrDone(ctx, chantest.Feed(ctx, 10*time.Millisecond, 0, 1, 2)), time.Second)
		if want := []int{0, 1, 2}; !slices.Equal(got, want) {
			t.Errorf("got %v; want %v", got, want)
		}
	})
}

func TestStopsOnCancelWhileWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		out := OrDone(ctx, make(chan int)) // never sends, never closes
		time.AfterFunc(10*time.Millisecond, cancel)
		if got := chantest.Drain(t, out, time.Second); len(got) != 0 {
			t.Errorf("got %v; want nothing", got)
		}
	})
}

func TestNoLeakWhenConsumerWalksAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		in := make(chan int, 3)
		in <- 1
		in <- 2
		in <- 3
		out := OrDone(ctx, in)
		if v, ok, _ := chantest.Recv(out, time.Second); !ok || v != 1 {
			t.Fatalf("first receive = %v, %v; want 1, true", v, ok)
		}
		synctest.Wait()
		cancel()
		synctest.Wait()
		chantest.ExpectClosedNow(t, out, "OrDone")
	})
}
