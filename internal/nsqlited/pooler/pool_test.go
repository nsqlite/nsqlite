package pooler

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockResource struct {
	id int64
}

func TestResourcePool_CreateAndClose(t *testing.T) {
	var created int64
	var closed int64

	newFunc := func() (mockResource, error) {
		return mockResource{id: atomic.AddInt64(&created, 1)}, nil
	}

	closeFunc := func(r mockResource) error {
		atomic.AddInt64(&closed, 1)
		return nil
	}

	pool, err := NewPool(Config[mockResource]{
		MaxItems:  3,
		MaxIdle:   2,
		NewFunc:   newFunc,
		CloseFunc: closeFunc,
	})
	assert.NoError(t, err)
	assert.NotNil(t, pool)

	res1, err := pool.Get()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, res1.id)

	err = pool.Close()
	assert.NoError(t, err)

	res2, err := pool.Get()
	assert.Error(t, err)
	assert.EqualError(t, err, "pool is closed")
	assert.Zero(t, res2.id)
}

func TestResourcePool_MaxIdle(t *testing.T) {
	var created int64
	var closed int64

	newFunc := func() (mockResource, error) {
		return mockResource{id: atomic.AddInt64(&created, 1)}, nil
	}

	closeFunc := func(r mockResource) error {
		atomic.AddInt64(&closed, 1)
		return nil
	}

	pool, err := NewPool(Config[mockResource]{
		MaxItems:  5,
		MaxIdle:   2,
		NewFunc:   newFunc,
		CloseFunc: closeFunc,
	})
	assert.NoError(t, err)

	r1, err := pool.Get()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, r1.id)

	r2, err := pool.Get()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, r2.id)

	r3, err := pool.Get()
	assert.NoError(t, err)
	assert.EqualValues(t, 3, r3.id)

	err = pool.Put(r1)
	assert.NoError(t, err)

	err = pool.Put(r2)
	assert.NoError(t, err)

	err = pool.Put(r3)
	assert.NoError(t, err)

	assert.EqualValues(t, 3, created)
	assert.EqualValues(t, 1, closed)

	pool.Close()

	assert.EqualValues(t, 3, created)
	assert.EqualValues(t, 3, closed)
}

func TestResourcePool_BlockWhenFull(t *testing.T) {
	var created int64
	newFunc := func() (mockResource, error) {
		return mockResource{id: atomic.AddInt64(&created, 1)}, nil
	}
	closeFunc := func(r mockResource) error {
		return nil
	}

	pool, err := NewPool(Config[mockResource]{
		MaxItems:  2,
		MaxIdle:   1,
		NewFunc:   newFunc,
		CloseFunc: closeFunc,
	})
	assert.NoError(t, err)
	defer pool.Close()

	r1, err := pool.Get()
	assert.NoError(t, err)
	_, err = pool.Get()
	assert.NoError(t, err)

	ch := make(chan struct{})
	go func() {
		r3, getErr := pool.Get()
		assert.NoError(t, getErr)
		assert.NotZero(t, r3.id)
		close(ch)
	}()

	assert.EqualValues(t, 2, created)
	_ = pool.Put(r1)
	<-ch
}
