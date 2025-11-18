package dshot

import (
	"context"
	"fmt"
	"reflect"
)

type containerCtxKey struct{}

// WithContainer returns a new context with the container attached.
// This is useful for request-scoped containers in HTTP handlers.
//
// Example:
//
//	func middleware(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        reqContainer := container.NewScoped(container.Default())
//	        ctx := container.WithContainer(r.Context(), reqContainer)
//	        next.ServeHTTP(w, r.WithContext(ctx))
//	    })
//	}
func WithContainer(ctx context.Context, c *Container) context.Context {
	return context.WithValue(ctx, containerCtxKey{}, c)
}

// FromContext retrieves the container from the context.
// Returns the default container if no container is found in context.
//
// Example:
//
//	c := container.FromContext(ctx)
//	service := container.MustResolve[*Service](c)
func FromContext(ctx context.Context) *Container {
	if c, ok := ctx.Value(containerCtxKey{}).(*Container); ok {
		return c
	}
	return defaultContainer
}

// GetCtx retrieves a value by token from the container in context.
// Falls back to the default container if no container is in context.
//
// Example:
//
//	broker := container.GetCtx[*Broker](ctx, brokerToken)
func GetCtx[T any](ctx context.Context, token *Token[T]) T {
	return FromContext(ctx).Get(token).(T)
}

// FindCtx retrieves a value by token from the container in context.
// Returns false if not found.
//
// Example:
//
//	if broker, ok := container.FindCtx[*Broker](ctx, brokerToken); ok {
//	    // use broker
//	}
func FindCtx[T any](ctx context.Context, token *Token[T]) (T, bool) {
	c := FromContext(ctx)
	e, ok := c.getEntry(token)
	if !ok {
		var zero T
		return zero, false
	}
	return e.resolve().(T), true
}

// ResolveCtx attempts to find a dependency by type from the container in context.
//
// Example:
//
//	if config, ok := container.ResolveCtx[*Config](ctx); ok {
//	    // use config
//	}
func ResolveCtx[T any](ctx context.Context) (T, bool) {
	var zero T
	targetType := reflect.TypeOf(zero)

	if targetType == nil {
		return zero, false
	}

	c := FromContext(ctx)
	val, ok := c.Resolve(targetType)
	if !ok {
		return zero, false
	}

	return val.(T), true
}

// MustResolveCtx resolves by type from the container in context and panics if not found.
//
// Example:
//
//	config := container.MustResolveCtx[*Config](ctx)
func MustResolveCtx[T any](ctx context.Context) T {
	val, ok := ResolveCtx[T](ctx)
	if !ok {
		var target T
		targetType := reflect.TypeOf(target)
		panic(fmt.Sprintf("could not resolve dependency of type %s from context", targetType))
	}
	return val
}

// ResolveAllCtx returns all registered values of type T from the container in context.
//
// Example:
//
//	handlers := container.ResolveAllCtx[Handler](ctx)
//	for _, h := range handlers {
//	    h.Handle()
//	}
func ResolveAllCtx[T any](ctx context.Context) []T {
	targetType := reflect.TypeFor[T]()

	if targetType == nil {
		return nil
	}

	c := FromContext(ctx)
	results := c.ResolveAll(targetType)

	typed := make([]T, len(results))
	for i, val := range results {
		typed[i] = val.(T)
	}

	return typed
}

// InjectCtx populates a struct's fields by resolving them from the container in context.
//
// Example:
//
//	type Dependencies struct {
//	    Config  *Config
//	    ReqCtx  *RequestContext
//	}
//	var deps Dependencies
//	container.InjectCtx(ctx, &deps)
func InjectCtx(ctx context.Context, target any) {
	FromContext(ctx).Inject(target)
}

// CallCtx calls a function, resolving its dependencies from the container in context.
//
// Example:
//
//	service := container.CallCtx[*Service](ctx, func(config *Config, reqCtx *RequestContext) *Service {
//	    return NewService(config, reqCtx)
//	})
func CallCtx[T any](ctx context.Context, fn any) T {
	return Call[T](fn, FromContext(ctx))
}

// CallCtxErr calls a function that returns (T, error), resolving from context.
//
// Example:
//
//	service, err := container.CallCtxErr[*Service](ctx, func(config *Config) (*Service, error) {
//	    return NewService(config)
//	})
func CallCtxErr[T any](ctx context.Context, fn any) (T, error) {
	return CallErr[T](fn, FromContext(ctx))
}

// BuildCtx creates an instance by injecting dependencies from the container in context.
//
// Example:
//
//	type ServiceDeps struct {
//	    Config *Config
//	    ReqCtx *RequestContext
//	}
//	service := container.BuildCtx[*Service](ctx, func(deps ServiceDeps) *Service {
//	    return &Service{config: deps.Config, reqCtx: deps.ReqCtx}
//	})
func BuildCtx[T any](ctx context.Context, constructor any) T {
	return Build[T](constructor, FromContext(ctx))
}
