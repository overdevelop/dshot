package dshot

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"
)

// primitiveKinds lists types that cannot be auto-resolved
var primitiveKinds = []reflect.Kind{
	reflect.Bool,
	reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.Float32, reflect.Float64,
	reflect.String,
	reflect.Slice,
	reflect.Map,
	reflect.Chan,
}

func isPrimitive(kind reflect.Kind) bool {
	return slices.Contains(primitiveKinds, kind)
}

// BindAutoFactory creates a registration with a factory that auto-wires dependencies.
// Dependencies are resolved from the specified container (or default if not provided).
//
// Example:
//
//	container.Register(
//	    container.BindAutoFactory(repoToken, func(db *sqlx.DB) *Repository {
//	        return NewRepository(db)
//	    }),
//	)
func BindAutoFactory[T any](token *Token[T], factory any, containers ...*Container) Registration[T] {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}
	return buildAutoFactory(token, factory, Singleton, false, c)
}

// BindAutoPrototype is like BindAutoFactory but with Prototype lifecycle
func BindAutoPrototype[T any](token *Token[T], factory any, containers ...*Container) Registration[T] {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}
	return buildAutoFactory(token, factory, Prototype, false, c)
}

// BindAutoSingleton is an alias for BindAutoFactory
func BindAutoSingleton[T any](token *Token[T], factory any, containers ...*Container) Registration[T] {
	return BindAutoFactory(token, factory, containers...)
}

// ProvideAutoFactory registers a singleton factory that auto-wires dependencies without requiring a token.
// Dependencies are resolved from the container at the time of factory invocation.
//
// Example:
//
//	container.ProvideAutoFactory(func(db *sqlx.DB, logger *Logger) *Repository {
//	    return NewRepository(db, logger)
//	})
func ProvideAutoFactory(factory any, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}
	c.provideAutoFactoryWithLifecycle(factory, Singleton, false)
}

// ProvideAutoFactories registers multiple singleton factories that auto-wire dependencies without requiring tokens.
// The last argument can optionally be a Container instance.
//
// Example:
//
//	container.ProvideAll(
//	    func(db *sqlx.DB) *Repository {
//	        return NewRepository(db)
//	    },
//	    func(repo *Repository) *Service {
//	        return NewService(repo)
//	    },
//	)
func ProvideAutoFactories(items ...any) {
	c := defaultContainer

	if len(items) > 1 && items[len(items)-1] != nil {
		if cont, ok := items[len(items)-1].(*Container); ok {
			c = cont
			items = items[:len(items)-1]
		}
	}

	for _, factory := range items {
		c.provideAutoFactoryWithLifecycle(factory, Singleton, false)
	}
}

// ProvideAutoPrototype registers a prototype factory that auto-wires dependencies without requiring a token.
// A new instance is created on each resolution.
//
// Example:
//
//	container.ProvideAutoPrototype(func(db *sqlx.DB) *Request {
//	    return NewRequest(db)
//	})
func ProvideAutoPrototype(factory any, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}
	c.provideAutoFactoryWithLifecycle(factory, Prototype, false)
}

// ProvideAutoSingleton is an alias for ProvideAutoFactory
func ProvideAutoSingleton(factory any, containers ...*Container) {
	ProvideAutoFactory(factory, containers...)
}

// Wrap takes a factory function that returns a handler function and wraps it with dependency injection.
// The factory is called once with injected dependencies, and returns the actual handler.
//
// Example:
//
//	func makeHandler(deps struct {
//	    DB     *sqlx.DB
//	    Logger *Logger
//	}) func(ctx context.Context, event MyEvent) error {
//	    return func(ctx context.Context, event MyEvent) error {
//	        // use deps.DB and deps.Logger
//	        return nil
//	    }
//	}
//
//	handler := container.Wrap(makeHandler)
//	// handler is now: func(ctx context.Context, event MyEvent) error
func Wrap[T, Arg any](factory func(Arg) T, containers ...*Container) T {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	fnValue := reflect.ValueOf(factory)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("Wrap: factory must be a function")
	}

	if fnType.NumOut() != 1 {
		panic("Wrap: factory must return exactly one value (the handler function)")
	}

	handlerType := fnType.Out(0)
	if handlerType.Kind() != reflect.Func {
		panic("Wrap: factory must return a function")
	}

	// Resolve factory parameters and call it once to get the handler
	numIn := fnType.NumIn()
	args := make([]reflect.Value, numIn)

	for i := 0; i < numIn; i++ {
		paramType := fnType.In(i)
		arg, err := resolveParameter(c, paramType, numIn)
		if err != nil {
			panic(fmt.Sprintf("Wrap: factory parameter %d (%s): %v", i, paramType, err))
		}
		args[i] = arg
	}

	// Call the factory to get the handler
	results := fnValue.Call(args)
	handler := results[0].Interface().(T)

	return handler
}

// Invoke calls a function, automatically resolving its dependencies from the specified container.
func Invoke(fn any, containers ...*Container) []any {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("Invoke: argument must be a function")
	}

	args := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		arg, err := resolveParameter(c, paramType, fnType.NumIn())
		if err != nil {
			panic(fmt.Sprintf("Invoke: parameter %d (%s): %v", i, paramType, err))
		}
		args[i] = arg
	}

	results := fnValue.Call(args)

	out := make([]any, len(results))
	for i, result := range results {
		out[i] = result.Interface()
	}

	return out
}

// Call is a type-safe version of Invoke that returns T.
//
// Example:
//
//	service := container.Call[*Service](func(db *Database, logger *Logger) *Service {
//	    return NewService(db, logger)
//	})
func Call[T any](fn any, containers ...*Container) T {
	results := Invoke(fn, containers...)
	return results[0].(T)
}

// CallErr is a type-safe version that handles functions returning (T, error).
//
// Example:
//
//	service, err := container.CallErr[*Service](func(db *Database) (*Service, error) {
//	    return NewService(db)
//	})
func CallErr[T any](fn any, containers ...*Container) (T, error) {
	results := Invoke(fn, containers...)

	var zero T
	if len(results) != 2 {
		return zero, fmt.Errorf("CallErr: function must return (T, error)")
	}

	val := results[0].(T)
	if results[1] == nil {
		return val, nil
	}

	err := results[1].(error)
	return val, err
}

// CallContext calls a context-aware function with the provided context.
//
// Example:
//
//	service := container.CallContext[*Service](ctx, func(ctx context.Context, db *Database) *Service {
//	    return NewServiceWithContext(ctx, db)
//	})
func CallContext[T any](ctx context.Context, fn any, containers ...*Container) T {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("CallContext: argument must be a function")
	}

	if fnType.NumIn() < 1 {
		panic("CallContext: function must have at least one parameter (context.Context)")
	}

	ctxType := reflect.TypeFor[context.Context]()
	if fnType.In(0) != ctxType {
		panic("CallContext: first parameter must be context.Context")
	}

	args := make([]reflect.Value, fnType.NumIn())
	args[0] = reflect.ValueOf(ctx)

	for i := 1; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		arg, err := resolveParameter(c, paramType, fnType.NumIn())
		if err != nil {
			panic(fmt.Sprintf("CallContext: parameter %d (%s): %v", i, paramType, err))
		}
		args[i] = arg
	}

	results := fnValue.Call(args)
	return results[0].Interface().(T)
}

// CallContextErr calls a context-aware function that returns (T, error).
//
// Example:
//
//	service, err := container.CallContextErr[*Service](ctx, func(ctx context.Context, db *Database) (*Service, error) {
//	    return InitService(ctx, db)
//	})
func CallContextErr[T any](ctx context.Context, fn any, containers ...*Container) (T, error) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("CallContextErr: argument must be a function")
	}

	if fnType.NumIn() < 1 {
		panic("CallContextErr: function must have at least one parameter (context.Context)")
	}

	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if fnType.In(0) != ctxType {
		panic("CallContextErr: first parameter must be context.Context")
	}

	args := make([]reflect.Value, fnType.NumIn())
	args[0] = reflect.ValueOf(ctx)

	for i := 1; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		arg, err := resolveParameter(c, paramType, fnType.NumIn())
		if err != nil {
			panic(fmt.Sprintf("CallContextErr: parameter %d (%s): %v", i, paramType, err))
		}
		args[i] = arg
	}

	results := fnValue.Call(args)

	var zero T
	if len(results) != 2 {
		return zero, fmt.Errorf("function must return (T, error)")
	}

	val := results[0].Interface().(T)
	if results[1].IsNil() {
		return val, nil
	}

	return val, results[1].Interface().(error)
}

// Inject populates a struct's fields by resolving them from the specified container.
func Inject(target any, containers ...*Container) {
	c := defaultContainer
	if len(containers) > 0 && containers[0] != nil {
		c = containers[0]
	}

	c.Inject(target)
}

// Build creates an instance by injecting dependencies into the provided constructor.
func Build[T any](constructor any, containers ...*Container) T {
	return Call[T](constructor, containers...)
}

// resolveParameter resolves a single parameter by type from the specified container
func resolveParameter(c *Container, paramType reflect.Type, numIn int) (reflect.Value, error) {
	isPtr := paramType.Kind() == reflect.Ptr
	searchType := paramType
	if isPtr {
		searchType = paramType.Elem()
	}

	if isPrimitive(searchType.Kind()) {
		return reflect.Value{}, fmt.Errorf("cannot auto-resolve primitive type %s", paramType)
	}

	val, ok := c.Resolve(paramType)
	if ok {
		return reflect.ValueOf(val), nil
	}

	if numIn == 1 && searchType.Kind() == reflect.Struct {
		argValue := reflect.New(searchType)

		c.Inject(argValue.Interface())

		return argValue.Elem(), nil
	}

	return reflect.Value{}, fmt.Errorf("no registration found for type %s", paramType)
}

// buildAutoFactory is the internal implementation for auto-wiring factories
func buildAutoFactory[T any](
	token *Token[T],
	factory any,
	lifecycle Lifecycle,
	withError bool,
	container *Container,
) Registration[T] {
	fnValue := reflect.ValueOf(factory)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("factory must be a function")
	}

	var zero T
	expectedType := reflect.TypeOf(zero)

	if withError {
		if fnType.NumOut() != 2 {
			panic("factory with error must return (T, error)")
		}
		if fnType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			panic("factory second return must be error")
		}
		if fnType.Out(0) != expectedType {
			panic(
				fmt.Sprintf(
					"factory return type %v doesn't match token type %v",
					fnType.Out(0), expectedType,
				),
			)
		}
	} else {
		if fnType.NumOut() != 1 {
			panic("factory must return exactly one value")
		}
		if fnType.Out(0) != expectedType {
			panic(
				fmt.Sprintf(
					"factory return type %v doesn't match token type %v",
					fnType.Out(0), expectedType,
				),
			)
		}
	}

	wrappedFactory := func() T {
		return resolveAndCall[T](container, fnValue, fnType, withError, token.key)
	}

	return Registration[T]{
		token:     token,
		factory:   wrappedFactory,
		lifecycle: lifecycle,
	}
}

// resolveAndCall resolves parameters and calls the function
func resolveAndCall[T any](
	c *Container,
	fnValue reflect.Value,
	fnType reflect.Type,
	withError bool,
	tokenKey string,
) T {
	numIn := fnType.NumIn()
	args := make([]reflect.Value, numIn)

	for i := 0; i < fnType.NumIn(); i++ {
		paramType := fnType.In(i)
		arg, err := resolveParameter(c, paramType, numIn)
		if err != nil {
			panic(
				fmt.Sprintf(
					"auto-wire factory[%v]: parameter %d (%s): %v",
					tokenKey, i, paramType, err,
				),
			)
		}
		args[i] = arg
	}

	results := fnValue.Call(args)

	if withError {
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			panic(fmt.Sprintf("factory[%v] returned error: %v", tokenKey, err))
		}
		return results[0].Interface().(T)
	}

	return results[0].Interface().(T)
}

// provideAutoFactoryWithLifecycle is the internal implementation for auto-wiring factories without tokens
func (c *Container) provideAutoFactoryWithLifecycle(factory any, lifecycle Lifecycle, withError bool) {
	fnValue := reflect.ValueOf(factory)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("factory must be a function")
	}

	var returnType reflect.Type
	if withError {
		if fnType.NumOut() != 2 {
			panic("factory with error must return (T, error)")
		}
		if fnType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
			panic("factory second return must be error")
		}
		returnType = fnType.Out(0)
	} else {
		if fnType.NumOut() != 1 {
			panic("factory must return exactly one value")
		}
		returnType = fnType.Out(0)
	}

	token := &tokenKey{
		key: fmt.Sprintf("__provided__%s_%d", returnType.String(), time.Now().UnixNano()),
	}

	wrappedFactory := func() any {
		return resolveAndCall[any](c, fnValue, fnType, withError, token.key)
	}

	e := &entry{
		factory:   wrappedFactory,
		lifecycle: lifecycle,
		depType:   returnType,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.registry[token] = e
	c.typeRegistry[returnType] = append(c.typeRegistry[returnType], e)
}
