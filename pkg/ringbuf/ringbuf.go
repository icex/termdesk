package ringbuf

// RingBuf is a fixed-capacity ring buffer that overwrites the oldest entry when full.
type RingBuf[T any] struct {
	items []T
	cap   int
}

// New creates a ring buffer with the given capacity.
func New[T any](capacity int) *RingBuf[T] {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuf[T]{cap: capacity}
}

// Push adds an item. If at capacity, the oldest item is dropped.
func (r *RingBuf[T]) Push(item T) {
	if len(r.items) < r.cap {
		r.items = append(r.items, item)
	} else {
		// Shift left and replace last
		copy(r.items, r.items[1:])
		r.items[len(r.items)-1] = item
	}
}

// Items returns all items from oldest to newest.
func (r *RingBuf[T]) Items() []T {
	out := make([]T, len(r.items))
	copy(out, r.items)
	return out
}

// Latest returns the most recently pushed item and true, or zero value and false if empty.
func (r *RingBuf[T]) Latest() (T, bool) {
	if len(r.items) == 0 {
		var zero T
		return zero, false
	}
	return r.items[len(r.items)-1], true
}

// Len returns the current number of items.
func (r *RingBuf[T]) Len() int {
	return len(r.items)
}

// Cap returns the capacity.
func (r *RingBuf[T]) Cap() int {
	return r.cap
}

// Clear removes all items.
func (r *RingBuf[T]) Clear() {
	r.items = nil
}
