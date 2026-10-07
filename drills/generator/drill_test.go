package generator

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"gotutor/drills/internal/chantest"
)

func TestValues(t *testing.T) {
	for _, start := range []int{0, 10, -3} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			ch := Generate(ctx, start)
			for i := range 5 {
				v, ok, timedOut := chantest.Recv(ch, time.Second)
				if !ok || timedOut || v != start+i {
					t.Fatalf("start=%d receive #%d = %d (ok=%v timedOut=%v); want %d",
						start, i, v, ok, timedOut, start+i)
				}
			}
			cancel()
			chantest.Drain(t, ch, time.Second)
		})
	}
}

func TestClosesAfterCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ch := Generate(ctx, 0)
		if _, ok, _ := chantest.Recv(ch, time.Second); !ok {
			t.Fatal("no value before cancel")
		}
		cancel()
		chantest.Drain(t, ch, time.Second)
	})
}

func TestNoLeakWhenConsumerWalksAway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		ch := Generate(ctx, 0)
		if _, ok, _ := chantest.Recv(ch, time.Second); !ok {
			t.Fatal("no value before cancel")
		}
		cancel()
		synctest.Wait()
		chantest.ExpectClosedNow(t, ch, "Generate")
	})
}
