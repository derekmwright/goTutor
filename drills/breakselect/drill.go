// Drill: write it from memory, no notes. Reset with `task drill -- breakselect`.
package breakselect

import "context"

// Collect receives from in and returns the values seen before the first
// value equal to stop (not included), or before in closes or ctx is done.
// It must not read past stop. Use a for loop around a select.
func Collect(ctx context.Context, in <-chan int, stop int) []int {
	return nil
}
