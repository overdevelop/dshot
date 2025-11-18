package dshot

import (
	"fmt"
	"log/slog"
	"reflect"
	"sync"

	"github.com/overdevelop/dshot/internal/logger"
)

// Lifecycle determines how a factory-based dependency is instantiated
type Lifecycle int

const (
	Singleton Lifecycle = iota
	Prototype
)

// Container holds a registry of dependencies
type Container struct {
	registry     map[any]*entry
	typeRegistry map[reflect.Type][]*entry
	parent       *Container // Parent container for scoped lookups
	mu           sync.RWMutex
}

// New creates a new isolated container instance.
// The container is completely independent with no fallback.
//
// Example:
//
//	c := container.New()
//	c.Provide(&Config{...})
//	config := container.MustResolve[*Config](c)
func New() *Container {
	return &Container{
		registry:     make(map[any]*entry),
		typeRegistry: make(map[reflect.Type][]*entry),
		parent:       nil,
	}
}

// NewScoped creates a new container that falls back to a parent container.
// Registrations are local to this scope, but lookups check parent if not found locally.
// Useful for request-scoped dependencies.
//
// Example:
//
//	// Global app container
//	appContainer := container.Default()
//
//	// Request-scoped container
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    reqContainer := container.NewScoped(appContainer)
//	    reqContainer.Provide(&RequestContext{ID: uuid.New()})
//
//	    // Can resolve both request-scoped and app-scoped deps
//	    reqCtx := container.MustResolve[*RequestContext](reqContainer)
//	    config := container.MustResolve[*Config](reqContainer) // Falls back to parent
//	}
func NewScoped(parent *Container) *Container {
	if parent == nil {
		panic("NewScoped: parent container cannot be nil")
	}

	return &Container{
		registry:     make(map[any]*entry),
		typeRegistry: make(map[reflect.Type][]*entry),
		parent:       parent,
	}
}

// Provide registers a value without a token (type-based registration).
func (c *Container) Provide(value any) {
	typ := reflect.TypeOf(value)
	if typ == nil {
		panic("Provide: cannot register nil value")
	}

	token := &tokenKey{
		key: fmt.Sprintf("__provided__%s", typ.String()),
	}

	e := &entry{
		value:     value,
		lifecycle: Singleton,
		depType:   typ,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry[token] = e
	c.typeRegistry[typ] = append(c.typeRegistry[typ], e)
}

// ProvideFactory registers a singleton factory function without a token.
func (c *Container) ProvideFactory(factory any) {
	c.provideFactoryWithLifecycle(factory, Singleton)
}

// ProvidePrototype registers a prototype factory without a token.
func (c *Container) ProvidePrototype(factory any) {
	c.provideFactoryWithLifecycle(factory, Prototype)
}

// Register adds one or more token-based dependencies to the container.
func (c *Container) Register(registrations ...registration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, reg := range registrations {
		reg.registerTo(c)
	}
}

// getEntry retrieves an entry, checking parent if not found locally
func (c *Container) getEntry(token any) (*entry, bool) {
	c.mu.RLock()
	e, ok := c.registry[token]
	c.mu.RUnlock()

	if ok {
		return e, true
	}

	if c.parent != nil {
		return c.parent.getEntry(token)
	}

	return nil, false
}

// Get retrieves a value from the container by token.
// Falls back to the parent container if this is a scoped container.
func (c *Container) Get(token any) any {
	if token == nil {
		panic("cannot get with nil token")
	}

	e, ok := c.getEntry(token)
	if !ok {
		panic(fmt.Sprintf("dependency not found: %v", token))
	}

	return e.resolve()
}

// Resolve attempts to find a dependency by type.
// Falls back to the parent container if this is a scoped container.
func (c *Container) Resolve(targetType reflect.Type) (any, bool) {
	c.mu.RLock()
	if entries, ok := c.typeRegistry[targetType]; ok && len(entries) > 0 {
		c.mu.RUnlock()
		if len(entries) > 1 {
			panic(
				fmt.Errorf(
					"multiple candidates found for type %s: found %d registrations",
					targetType.String(),
					len(entries),
				),
			)
		}
		return entries[0].resolve(), true
	}
	c.mu.RUnlock()

	return c.findSingleEntry(targetType)
}

// findSingleEntry scans registry for a single matching entry
func (c *Container) findSingleEntry(targetType reflect.Type) (any, bool) {
	var exactMatch *entry
	var similarMatch *entry

	c.mu.RLock()
	for _, e := range c.registry {
		valType := e.depType

		if c.isExactMatch(targetType, valType) {
			if exactMatch != nil {
				c.mu.RUnlock()
				panic(
					fmt.Errorf(
						"multiple candidates found for type %s in registry",
						targetType.String(),
					),
				)
			}
			exactMatch = e
		} else if similarMatch == nil && c.isSimilarType(targetType, valType) {
			similarMatch = e
		}
	}
	c.mu.RUnlock()

	if exactMatch != nil {
		return exactMatch.resolve(), true
	}

	if c.parent != nil {
		if val, ok := c.parent.findSingleEntry(targetType); ok {
			return val, true
		}
	}

	if similarMatch != nil {
		logger.Warn(
			fmt.Sprintf(
				"No exact match for type %s, using similar type. "+
					"Consider registering the exact type.",
				targetType,
			),
			slog.String("targetType", targetType.String()),
		)
		return c.resolveAndConvert(targetType, similarMatch, true)
	}

	return nil, false
}

// ResolveAll returns all registered values of type T.
// Includes values from parent containers.
func (c *Container) ResolveAll(targetType reflect.Type) []any {
	seen := make(map[*entry]bool)

	c.mu.RLock()
	capacity := len(c.typeRegistry[targetType])
	c.mu.RUnlock()

	results := make([]any, 0, capacity+4)

	c.mu.RLock()
	if typeEntries, ok := c.typeRegistry[targetType]; ok {
		for _, e := range typeEntries {
			if !seen[e] {
				seen[e] = true
				results = append(results, e.resolve())
			}
		}
	}
	c.mu.RUnlock()

	c.collectEntriesDirectly(targetType, seen, &results)

	return results
}

// collectEntriesDirectly scans the registry and appends resolved values directly to results
func (c *Container) collectEntriesDirectly(targetType reflect.Type, seen map[*entry]bool, results *[]any) {
	var similarEntries []*entry
	hasExactMatch := false

	c.mu.RLock()
	for _, e := range c.registry {
		if seen[e] {
			continue
		}
		valType := e.depType

		if c.isExactMatch(targetType, valType) {
			seen[e] = true
			*results = append(*results, e.resolve())
			hasExactMatch = true
		} else if c.isSimilarType(targetType, valType) {
			similarEntries = append(similarEntries, e)
			seen[e] = true
		}
	}
	c.mu.RUnlock()

	if c.parent != nil {
		c.parent.collectEntriesDirectly(targetType, seen, results)
	}

	if !hasExactMatch && len(similarEntries) > 0 {
		logger.Warn(
			fmt.Sprintf(
				"No exact match for type %s, using %d similar type(s). "+
					"Consider registering the exact type.",
				targetType,
				len(similarEntries),
			),
			slog.String("targetType", targetType.String()),
			slog.Int("similarEntries", len(similarEntries)),
		)

		for _, e := range similarEntries {
			if resolved, ok := c.resolveAndConvert(targetType, e, true); ok {
				*results = append(*results, resolved)
			}
		}
	}
}

// isExactMatch checks if valType exactly matches or is assignable to targetType
func (c *Container) isExactMatch(targetType, valType reflect.Type) bool {
	if targetType.Kind() == reflect.Interface {
		return valType.Implements(targetType)
	}
	return valType.AssignableTo(targetType)
}

// isSimilarType checks if valType is a similar type (pointer mismatch)
func (c *Container) isSimilarType(targetType, valType reflect.Type) bool {
	if targetType == valType {
		return false
	}

	// Target wants *T, we have T
	if targetType.Kind() == reflect.Ptr && valType.Kind() != reflect.Ptr {
		return targetType.Elem() == valType
	}
	// Target wants T, we have *T
	if targetType.Kind() != reflect.Ptr && valType.Kind() == reflect.Ptr {
		return valType.Elem() == targetType
	}
	return false
}

// resolveAndConvert resolves an entry and converts it to the target type if needed
func (c *Container) resolveAndConvert(targetType reflect.Type, e *entry, needsConversion bool) (any, bool) {
	resolved := e.resolve()

	if !needsConversion {
		return resolved, true
	}

	resolvedVal := reflect.ValueOf(resolved)
	resolvedType := resolvedVal.Type()

	if resolvedType == targetType {
		return resolved, true
	}

	if targetType.Kind() == reflect.Ptr && resolvedType.Kind() != reflect.Ptr {
		if targetType.Elem() != resolvedType {
			logger.Warn(
				fmt.Sprintf("Type mismatch: cannot convert %s to %s", resolvedType, targetType),
				slog.String("resolvedType", resolvedType.String()),
				slog.String("targetType", targetType.String()),
			)
			return nil, false
		}

		// Target wants *T, we have T -> take address
		if resolvedVal.CanAddr() {
			return resolvedVal.Addr().Interface(), true
		}
		// Value is not addressable, create a new pointer
		ptr := reflect.New(resolvedType)
		ptr.Elem().Set(resolvedVal)
		return ptr.Interface(), true
	}

	if targetType.Kind() != reflect.Ptr && resolvedType.Kind() == reflect.Ptr {
		if resolvedType.Elem() != targetType {
			logger.Warn(
				fmt.Sprintf("Type mismatch: cannot convert %s to %s", resolvedType, targetType),
				slog.String("resolvedType", resolvedType.String()),
				slog.String("targetType", targetType.String()),
			)
			return nil, false
		}

		// Target wants T, we have *T -> dereference
		if !resolvedVal.IsNil() {
			return resolvedVal.Elem().Interface(), true
		}
		// Skip nil pointers
		return nil, false
	}

	return resolved, true
}

// Inject populates a struct's fields by resolving them from the container.
func (c *Container) Inject(target any) {
	targetValue := reflect.ValueOf(target)
	targetType := targetValue.Type()

	if targetType.Kind() != reflect.Ptr {
		panic("Inject: target must be a pointer to a struct")
	}

	targetType = targetType.Elem()
	targetValue = targetValue.Elem()

	if targetType.Kind() != reflect.Struct {
		panic("Inject: target must be a pointer to a struct")
	}

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		fieldValue := targetValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		if val, ok := c.Resolve(field.Type); ok {
			fieldValue.Set(reflect.ValueOf(val))
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			newStruct := reflect.New(field.Type)
			c.Inject(newStruct.Interface())
			fieldValue.Set(newStruct.Elem())
			continue
		}

		panic(
			fmt.Sprintf(
				"Inject: could not resolve field %s (%s) in struct %s",
				field.Name, field.Type, targetType.Name(),
			),
		)
	}
}

// Clear removes all dependencies from this container (does not affect parent)
func (c *Container) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry = make(map[any]*entry)
	c.typeRegistry = make(map[reflect.Type][]*entry)
}

// Parent returns the parent container, or nil if this is a root container
func (c *Container) Parent() *Container {
	return c.parent
}

func (c *Container) provideFactoryWithLifecycle(factory any, lifecycle Lifecycle) {
	fnValue := reflect.ValueOf(factory)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("factory must be a function")
	}

	if fnType.NumOut() != 1 {
		panic("factory must return exactly one value")
	}

	returnType := fnType.Out(0)
	token := &tokenKey{
		key: fmt.Sprintf("__provided__%s", returnType.String()),
	}

	e := &entry{
		factory: func() any {
			results := fnValue.Call(nil)
			return results[0].Interface()
		},
		lifecycle: lifecycle,
		depType:   returnType,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry[token] = e
	c.typeRegistry[returnType] = append(c.typeRegistry[returnType], e)
}
