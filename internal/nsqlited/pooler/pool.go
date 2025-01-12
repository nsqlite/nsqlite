package pooler

import (
	"errors"
	"sync"
)

type Config[T any] struct {
	// MaxItems is the maximum total number of items allowed in the pool.
	// Must be greater than zero.
	MaxItems int
	// MaxIdle is the maximum number of items allowed to remain idle.
	// Must be greater than or equal to zero.
	// Must not exceed MaxItems.
	MaxIdle int
	// NewFunc is the function to create a new item.
	NewFunc func() (T, error)
	// CloseFunc is the function to close an item.
	CloseFunc func(T) error
}

// Pool is a generic, thread-safe pool for any resource type T.
// It enforces a maximum number of total items (maxItems) and a maximum
// number of idle items (maxIdle). When Put() is called, if maxIdle is reached,
// the resource is closed rather than stored.
type Pool[T any] struct {
	Config[T]

	mu     sync.Mutex
	cond   *sync.Cond
	closed bool

	totalItems int
	idleItems  []T
}

// NewPool creates a ResourcePool with the specified limits and functions.
// maxItems is the maximum total number of items allowed in the pool.
// maxIdle is the maximum number of items allowed to remain idle.
func NewPool[T any](config Config[T]) (*Pool[T], error) {
	if config.MaxItems <= 0 {
		return nil, errors.New("maxItems must be greater than zero")
	}
	if config.MaxIdle < 0 {
		return nil, errors.New("maxIdle cannot be negative")
	}
	if config.MaxIdle > config.MaxItems {
		return nil, errors.New("maxIdle cannot exceed maxItems")
	}
	if config.NewFunc == nil {
		return nil, errors.New("newFunc must not be nil")
	}
	if config.CloseFunc == nil {
		return nil, errors.New("closeFunc must not be nil")
	}

	p := &Pool[T]{
		Config:    config,
		idleItems: make([]T, 0, config.MaxIdle),
	}
	p.cond = sync.NewCond(&p.mu)
	return p, nil
}

// Get retrieves a resource from the pool. If the pool is closed,
// an error is returned. If there are no idle items and the pool
// has reached maxItems, this call will block until an item is Put back.
func (p *Pool[T]) Get() (T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		if p.closed {
			var zero T
			return zero, errors.New("pool is closed")
		}

		if len(p.idleItems) > 0 {
			idx := len(p.idleItems) - 1
			res := p.idleItems[idx]
			p.idleItems = p.idleItems[:idx]
			return res, nil
		}

		if p.totalItems < p.MaxItems {
			res, err := p.NewFunc()
			if err != nil {
				var zero T
				return zero, err
			}
			p.totalItems++
			return res, nil
		}

		p.cond.Wait()
	}
}

// Put returns a resource to the pool. If the pool is closed,
// or if maxIdle is already reached, the resource will be closed.
func (p *Pool[T]) Put(res T) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return p.CloseFunc(res)
	}

	if len(p.idleItems) < p.MaxIdle {
		p.idleItems = append(p.idleItems, res)
		p.cond.Signal()
		return nil
	}

	p.totalItems--
	p.cond.Signal()
	return p.CloseFunc(res)
}

// Close closes the pool and all idle items. Any subsequent call to Get()
// will fail. Items that are not idle (checked out) must be closed by
// the caller when no longer needed.
func (p *Pool[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	var err error
	for _, res := range p.idleItems {
		if e := p.CloseFunc(res); e != nil && err == nil {
			err = e
		}
	}
	p.idleItems = nil
	p.cond.Broadcast()
	return err
}
