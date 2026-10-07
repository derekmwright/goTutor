package chantest

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

// MergeFunc is the shape of a two-input fan-in.
type MergeFunc func(ctx context.Context, a, b <-chan int) <-chan int

// RunMergeTests pins down fan-in behaviour shared by the merge drills.
func RunMergeTests(t *testing.T, merge MergeFunc) {
	tests := []struct {
		name string
		a, b func(context.Context) <-chan int
		want []int
	}{
		{"interleaved",
			func(ctx context.Context) <-chan int { return Feed(ctx, 10*time.Millisecond, 1, 2, 3) },
			func(ctx context.Context) <-chan int { return Feed(ctx, 15*time.Millisecond, 10, 20) },
			[]int{1, 2, 3, 10, 20}},
		{"a closes immediately",
			func(ctx context.Context) <-chan int { return Feed[int](ctx, 0) },
			func(ctx context.Context) <-chan int { return Feed(ctx, 50*time.Millisecond, 7, 8, 9) },
			[]int{7, 8, 9}},
		{"zero values are values",
			func(ctx context.Context) <-chan int { return Feed(ctx, 10*time.Millisecond, 0, 0) },
			func(ctx context.Context) <-chan int { return Feed(ctx, 25*time.Millisecond, 0) },
			[]int{0, 0, 0}},
		{"both empty",
			func(ctx context.Context) <-chan int { return Feed[int](ctx, 0) },
			func(ctx context.Context) <-chan int { return Feed[int](ctx, 0) },
			nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				got := Drain(t, merge(ctx, tt.a(ctx), tt.b(ctx)), time.Second)
				slices.Sort(got)
				if !slices.Equal(got, tt.want) {
					t.Errorf("got %v; want %v (sorted)", got, tt.want)
				}
			})
		})
	}

	t.Run("cancel while waiting for input", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			out := merge(ctx, make(chan int), make(chan int))
			time.AfterFunc(10*time.Millisecond, cancel)
			if got := Drain(t, out, time.Second); len(got) != 0 {
				t.Errorf("got %v; want nothing", got)
			}
		})
	})

	t.Run("cancel while consumer gone", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			a := make(chan int, 3)
			a <- 1
			a <- 2
			a <- 3
			out := merge(ctx, a, make(chan int))
			if v, ok, _ := Recv(out, time.Second); !ok || v != 1 {
				t.Fatalf("first receive = %v, %v; want 1, true", v, ok)
			}
			synctest.Wait()
			cancel()
			synctest.Wait()
			ExpectClosedNow(t, out, "merge")
		})
	})
}
