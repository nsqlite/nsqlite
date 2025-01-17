package syncutil

import (
	"sync/atomic"
)

type Atomic[T any] struct {
	value atomic.Value
}

// NewAtomic creates a new Atomic instance initialized with the given value.
func NewAtomic[T any](initial T) *Atomic[T] {
	a := &Atomic[T]{}
	a.Store(initial)
	return a
}

// Load returns the current value of the Atomic instance.
func (a *Atomic[T]) Load() T {
	val := a.value.Load()

	switch v := val.(type) {
	case T:
		return v
	default:
		var zero T
		return zero // Return the zero value for the type T
	}
}

// Store sets the value of the Atomic instance.
func (a *Atomic[T]) Store(value T) {
	a.value.Store(value)
}
