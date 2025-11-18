package dshot

import (
	"reflect"
	"sync"
)

type entry struct {
	value     any
	factory   func() any
	depType   reflect.Type
	lifecycle Lifecycle
	once      sync.Once
	mu        sync.Mutex
}

func (e *entry) resolve() any {
	if e.factory == nil {
		return e.value
	}

	if e.lifecycle == Prototype {
		return e.factory()
	}

	e.once.Do(
		func() {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.value = e.factory()
		},
	)

	return e.value
}
