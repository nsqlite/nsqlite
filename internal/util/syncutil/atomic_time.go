package syncutil

import "time"

// AtomicTime is a time.Time type that can be atomically loaded and stored
// by multiple goroutines safely.
type AtomicTime = Atomic[time.Time]

// NewAtomicTime creates a new AtomicTime with an initial value.
func NewAtomicTime(initial time.Time) *AtomicTime {
	atomicInst := &AtomicTime{}
	atomicInst.Store(initial)
	return atomicInst
}
