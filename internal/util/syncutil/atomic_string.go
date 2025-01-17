package syncutil

// AtomicString is a string type that can be atomically loaded and stored
// by multiple goroutines safely.
type AtomicString = Atomic[string]

// NewAtomicString creates a new AtomicString with an initial value.
func NewAtomicString(initial string) *AtomicString {
	atomicInst := &AtomicString{}
	atomicInst.Store(initial)
	return atomicInst
}
