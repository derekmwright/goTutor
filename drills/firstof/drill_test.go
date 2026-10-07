package firstof

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func sleepy(name string, d time.Duration) Backend {
	return func(context.Context, string) string { time.Sleep(d); return name }
}

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
		{"fastest wins, concurrently", time.Second,
			[]Backend{sleepy("slow", 300*time.Millisecond), sleepy("fast", 20*time.Millisecond)},
			"fast", nil, 20 * time.Millisecond},
		{"timeout, backends ignore ctx (leak check)", 100 * time.Millisecond,
			[]Backend{sleepy("a", 2*time.Second), sleepy("b", 3*time.Second)},
			"", context.DeadlineExceeded, 100 * time.Millisecond},
		{"no backends", time.Second, nil, "", ErrNoBackends, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				got, err := FirstOf(t.Context(), tt.timeout, "q", tt.backends...)
				if took := time.Since(start); took != tt.wantTook {
					t.Errorf("took %v; want %v", took, tt.wantTook)
				}
				if got != tt.want || !errors.Is(err, tt.wantErr) {
					t.Errorf("FirstOf = %q, %v; want %q, %v", got, err, tt.want, tt.wantErr)
				}
				time.Sleep(time.Hour) // let stragglers finish; a stuck one panics the bubble
			})
		})
	}
}

func TestParentCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(50*time.Millisecond, cancel)
		_, err := FirstOf(ctx, time.Second, "q", sleepy("a", 2*time.Second))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v; want context.Canceled", err)
		}
		time.Sleep(time.Hour)
	})
}

func TestCancelsLosers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cancelled atomic.Int32
		got, err := FirstOf(t.Context(), time.Second, "q",
			polite("fast", 10*time.Millisecond, &cancelled),
			polite("slow", 500*time.Millisecond, &cancelled),
			polite("slower", 800*time.Millisecond, &cancelled))
		if got != "fast" || err != nil {
			t.Fatalf("FirstOf = %q, %v; want \"fast\", nil", got, err)
		}
		synctest.Wait()
		if n := cancelled.Load(); n != 2 {
			t.Errorf("%d of 2 losers saw ctx cancelled on return; want 2", n)
		}
	})
}
