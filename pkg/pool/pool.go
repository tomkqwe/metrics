// Package pool provides reusable objects that are reset before being pooled.
package pool

import "sync"

// Pool holds objects of one type with a Reset method. It is safe for concurrent
// use, but individual objects must have a single owner. A Pool must be created
// with New and must not be copied after first use.
//
// Like sync.Pool, it may discard cached objects at any time. It is not intended
// for resources that require explicit cleanup, such as files or connections.
type Pool[T interface{ Reset() }] struct {
	items sync.Pool
}

// New creates a pool using newObject whenever no cached object is available.
// newObject must be non-nil, safe for concurrent calls, and return a fresh,
// ready-to-use object on each call (normally a non-nil pointer to a struct).
func New[T interface{ Reset() }](newObject func() T) *Pool[T] {
	return &Pool[T]{
		items: sync.Pool{New: func() any { return newObject() }},
	}
}

// Get returns a cached object or creates one using the factory passed to New.
// The caller exclusively owns the object until it is returned with Put.
func (p *Pool[T]) Get() T {
	return p.items.Get().(T)
}

// Put resets obj before making it available for reuse. The caller must not use
// obj or references to its mutable fields after Put, or put the same object twice.
// obj must be a valid object on which Reset can be called.
func (p *Pool[T]) Put(obj T) {
	obj.Reset()
	p.items.Put(obj)
}
