// Drill: write it from memory, no notes. Reset with `task drill -- firstof`.
package firstof

import (
	"context"
	"errors"
	"time"
)

// Backend answers q. Some backends ignore ctx.
type Backend func(ctx context.Context, q string) string

// ErrNoBackends is returned when FirstOf gets no backends.
var ErrNoBackends = errors.New("firstof: no backends")

// FirstOf queries all backends concurrently and returns the first answer, or
// ctx's error on timeout/cancel. Losers see ctx cancelled; nothing leaks, even
// when a backend ignores ctx.
func FirstOf(ctx context.Context, timeout time.Duration, q string, backends ...Backend) (string, error) {
	return "", nil
}
