package breakselect

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"gotutor/drills/internal/chantest"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		name string
		vs   []int
		stop int
		want []int
	}{
		{"stops at stop", []int{1, 2, 3, 99, 4}, 99, []int{1, 2, 3}},
		{"stop first", []int{99, 1}, 99, nil},
		{"closed before stop", []int{1, 2}, 99, []int{1, 2}},
		{"zero is a value", []int{0, 0, 5}, 5, []int{0, 0}},
		{"closed, stop is zero", []int{1, 2}, 0, []int{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				in := make(chan int, len(tt.vs))
				for _, v := range tt.vs {
					in <- v
				}
				if !slices.Contains(tt.vs, tt.stop) {
					close(in)
				}
				got := Collect(ctx, in, tt.stop)
				if !slices.Equal(got, tt.want) {
					t.Errorf("got %v; want %v", got, tt.want)
				}
				// Whatever followed stop must still be unread.
				if i := slices.Index(tt.vs, tt.stop); i >= 0 && len(in) != len(tt.vs)-i-1 {
					t.Errorf("Collect read past stop: %d values left in the channel; want %d",
						len(in), len(tt.vs)-i-1)
				}
			})
		})
	}
}

func TestCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		in := make(chan int, 2)
		in <- 1
		in <- 2 // never closed, no stop value
		time.AfterFunc(10*time.Millisecond, cancel)
		done := make(chan []int)
		go func() { done <- Collect(ctx, in, 99) }()
		got, ok, timedOut := chantest.Recv(done, time.Second)
		if timedOut || !ok {
			t.Fatal("Collect did not return after ctx was cancelled")
		}
		if want := []int{1, 2}; !slices.Equal(got, want) {
			t.Errorf("got %v; want %v", got, want)
		}
	})
}
