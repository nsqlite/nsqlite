package syncutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAtomicTime(t *testing.T) {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)

	atom := NewAtomicTime(now)
	assert.Equal(t, now, atom.Load())

	atom.Store(tomorrow)
	assert.Equal(t, tomorrow, atom.Load())
}
