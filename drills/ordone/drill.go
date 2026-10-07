// Drill: write it from memory, no notes. Reset with `task drill -- ordone`.
package ordone

import "context"

// OrDone forwards values from in until in is closed or ctx is done, then
// closes the returned channel. It must not leak if the consumer stops reading.
func OrDone[T any](ctx context.Context, in <-chan T) <-chan T {
	return nil
}
