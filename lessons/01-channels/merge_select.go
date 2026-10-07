package channels

import "context"

// ---------------------------------------------------------------------------
// Stretch: Merge with a single select
// ---------------------------------------------------------------------------

// MergeSelect behaves exactly like Merge (same tests), but its loop has a
// single select statement that handles receiving from a, receiving from b,
// forwarding to out, and cancellation all at once.
//
// The trick is the one you already used for closed inputs, applied in more
// places: a nil channel in a select is a case that's switched off. Keep local
// channel variables that point at the real channel when that case should be
// live, and are nil when it shouldn't.
func MergeSelect[T any](ctx context.Context, a, b <-chan T) <-chan T {
	// TODO(you): one goroutine, one loop, ONE select. Work out first:
	//   - when should the "send to out" case be live? What does its channel
	//     variable point at then, and at other times?
	//   - while a value is waiting to be sent, should the receive cases be
	//     live? What would a second receive do to the pending value?
	//   - the loop used to end when a and b were both nil. Is that enough
	//     now, or can something still be in flight?
	return nil
}
