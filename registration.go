package dshot

import "reflect"

type registration interface {
	registerTo(c *Container)
}

type Registration[T any] struct {
	token     *Token[T]
	value     T
	factory   func() T
	lifecycle Lifecycle
}

func (r Registration[T]) registerTo(c *Container) {
	e := &entry{
		lifecycle: r.lifecycle,
	}

	if r.factory != nil {
		e.factory = func() any {
			return r.factory()
		}
	} else {
		e.value = r.value
	}

	var zero T
	typ := reflect.TypeOf(zero)
	if typ != nil {
		e.depType = typ
		c.typeRegistry[typ] = append(c.typeRegistry[typ], e)
	}

	c.registry[r.token] = e
}

func Bind[T any](token *Token[T], value T) Registration[T] {
	return Registration[T]{
		token: token,
		value: value,
	}
}
