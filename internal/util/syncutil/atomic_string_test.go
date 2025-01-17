package syncutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAtomicString(t *testing.T) {
	atom := NewAtomicString("")
	assert.Equal(t, "", atom.Load())

	atom.Store("foo")
	assert.Equal(t, "foo", atom.Load())

	atom.Store("bar")
	assert.Equal(t, "bar", atom.Load())

	atom = NewAtomicString("baz")
	assert.Equal(t, "baz", atom.Load())
}
