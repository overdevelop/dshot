package dshot

import (
	"fmt"
	"reflect"
)

var defaultContainer = New()

// Register adds token-based dependencies to the global container
func Register(registrations ...registration) {
	defaultContainer.Register(registrations...)
}

// Provide registers a value in the specified container (or global if nil)
func Provide[T any](value T, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	c.Provide(value)
}

// ProvideFactory registers a singleton factory in the specified container (or global if nil)
func ProvideFactory[T any](factory func() T, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	c.ProvideFactory(factory)
}

// ProvidePrototype registers a prototype factory in the specified container (or global if nil)
func ProvidePrototype[T any](factory func() T, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	c.ProvidePrototype(factory)
}

// ProvideSingleton is an alias for ProvideFactory
func ProvideSingleton[T any](factory func() T, containers ...*Container) {
	ProvideFactory(factory, containers...)
}

// Get retrieves a value by token from the specified container (or global if nil)
func Get[T any](token *Token[T], containers ...*Container) T {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}
	return c.Get(token).(T)
}

// Find retrieves a value by token, returns false if not found
func Find[T any](token *Token[T], containers ...*Container) (T, bool) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	var zero T
	e, ok := c.getEntry(token)
	if !ok {
		return zero, false
	}

	return e.resolve().(T), true
}

// Resolve attempts to find a dependency by type
func Resolve[T any](containers ...*Container) (T, bool) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	var zero T
	targetType := reflect.TypeOf(zero)

	if targetType == nil {
		return zero, false
	}

	val, ok := c.Resolve(targetType)
	if !ok {
		return zero, false
	}

	return val.(T), true
}

// MustResolve resolves by type and panics if not found
func MustResolve[T any](containers ...*Container) T {
	val, ok := Resolve[T](containers...)
	if !ok {
		var target T
		targetType := reflect.TypeOf(target)
		panic(fmt.Sprintf("could not resolve dependency of type %s", targetType))
	}
	return val
}

// ResolveAll returns all registered values of type T
func ResolveAll[T any](containers ...*Container) []T {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	targetType := reflect.TypeFor[T]()
	if targetType == nil {
		return nil
	}

	results := c.ResolveAll(targetType)

	typed := make([]T, len(results))
	for i, val := range results {
		typed[i] = val.(T)
	}

	return typed
}

// Clear removes all dependencies from the global container
func Clear() {
	defaultContainer.Clear()
}

// Default returns the default global container
func Default() *Container {
	return defaultContainer
}
