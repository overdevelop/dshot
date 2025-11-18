# DSHOT

A powerful, type-safe dependency injection container for Go with support for generics, auto-wiring, scoped containers, and context integration.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Tokens](#tokens)
  - [Type-Based Registration (Provide)](#type-based-registration-provide)
  - [Token-Based Registration (Register)](#token-based-registration-register)
  - [Lifecycles](#lifecycles)
- [Container Types](#container-types)
  - [Global Container](#global-container)
  - [Isolated Container](#isolated-container)
  - [Scoped Container](#scoped-container)
- [Auto-Wiring](#auto-wiring)
- [Context Integration](#context-integration)
- [API Reference](#api-reference)
- [Patterns & Best Practices](#patterns--best-practices)

## Features

- ✅ **Type-Safe**: Full generics support with compile-time type checking
- ✅ **Multiple Registration Strategies**: Token-based or type-based
- ✅ **Auto-Wiring**: Automatic dependency resolution in functions
- ✅ **Scoped Containers**: Hierarchical containers with parent fallback
- ✅ **Context Integration**: Store and resolve from `context.Context`
- ✅ **Lifecycle Management**: Singleton and Prototype patterns
- ✅ **Error Handling**: Factory functions with error returns
- ✅ **Thread-Safe**: Safe for concurrent use
- ✅ **Zero Dependencies**: Only uses Go standard library

## Quick Start
```go
package main

import (
    "github.com/overdevelop/dshot"
)

type Config struct {
    DBUrl string
}

type Service struct {
    config *Config
}

func main() {
    // Provide a value (type-based)
    dshot.Provide(&Config{DBUrl: "postgres://localhost/db"})

    // Resolve by type
    config := dshot.MustResolve[*Config]()
    
    // Auto-wire a constructor
    service := dshot.Call[*Service](func(config *Config) *Service {
        return &Service{config: config}
    })
    
    println(service.config.DBUrl)
}
```
## Core Concepts

### Tokens

Tokens are type-safe keys for explicitly registering and retrieving dependencies.
```go
// Create a token
var configToken = dshot.NewToken[*Config]()

// Or with a custom name
var configToken = dshot.NewToken[*Config]("app-config")

// Register with token
dshot.Register(
    dshot.Bind(configToken, &Config{...}),
)

// Get by token
config := dshot.Get(configToken)
```
### Type-Based Registration (Provide)

The simplest way to register dependencies without explicit tokens. Dependencies are resolved by their type.
```go
// Provide a value
dshot.Provide(&Config{...})

// Provide a factory (singleton)
dshot.ProvideFactory(func() *Logger {
    return NewLogger()
})

// Provide a prototype factory (new instance each time)
dshot.ProvidePrototype(func() *http.Client {
    return &http.Client{Timeout: 30 * time.Second}
})

// Resolve by type
config := dshot.MustResolve[*Config]()
logger := dshot.MustResolve[*Logger]()
client1 := dshot.MustResolve[*http.Client]() // New instance
client2 := dshot.MustResolve[*http.Client]() // Another new instance
```
### Token-Based Registration (Register)

Explicit token-based registration for more control, especially useful when you need multiple instances of the same type or want to avoid global registration.
```go
dbToken := dshot.NewToken[*Database]("primary-db")
cacheToken := dshot.NewToken[*Database]("cache-db")

dshot.Register(
    dshot.Bind(dbToken, primaryDB),
    dshot.Bind(cacheToken, cacheDB),
    dshot.BindFactory(loggerToken, func() *Logger {
        return NewLogger()
    }),
)

primaryDB := dshot.Get(dbToken)
cacheDB := dshot.Get(cacheToken)
```
### Lifecycles

**Singleton** (default): Created once and reused.
```go
dshot.ProvideFactory(func() *Config {
    return LoadConfig() // Called only once
})
```
**Prototype**: Created every time it's resolved.
```go
dshot.ProvidePrototype(func() *http.Client {
    return &http.Client{} // Called every time
})
```
## Container Types

### Global Container

The default container available through package-level functions.
```go
// Register to global container
dshot.Provide(&Config{...})

// Resolve from global container
config := dshot.MustResolve[*Config]()
```
### Isolated Container

Completely independent container instances, useful for testing.
```go
// Create isolated container
testContainer := dshot.New()
testContainer.Provide(&MockConfig{...})

// Resolve from this container only
config := dshot.MustResolve[*MockConfig](testContainer)

// Or use container methods directly
config := testContainer.MustResolve[*MockConfig]()
```
### Scoped Container

Creates a child container that falls back to a parent dshot. Perfect for request-scoped dependencies.
```go
// Global dependencies
dshot.Provide(&Config{...})
dshot.Provide(&Database{...})

// Request-scoped container
reqContainer := dshot.NewScoped(dshot.Default())
reqContainer.Provide(&RequestContext{
    ID:     uuid.New(),
    UserID: "user123",
})

// Resolves RequestContext from scope, Database from parent
reqCtx := dshot.MustResolve[*RequestContext](reqContainer)
db := dshot.MustResolve[*Database](reqContainer) // Falls back to parent
```
## Auto-Wiring

Automatically resolve function parameters from the dshot.

### Basic Auto-Wiring
```go
// Simple function call
service := dshot.Call[*Service](func(config *Config, db *Database) *Service {
    return NewService(config, db)
})
```
### With Error Handling

```go
service, err := dshot.CallErr[*Service](func(db *Database) (*Service, error) {
    if !db.IsConnected() {
        return nil, fmt.Errorf("database not connected")
    }
    return NewService(db), nil
})
```

### Context-Aware Functions

```go
ctx := r.Context()
service := dshot.CallContext[*Service](ctx,
    func(ctx context.Context, config *Config, db *Database) *Service {
        return NewServiceWithContext(ctx, config, db)
    },
)
```

### Auto-Wired Factory Registration

```go
// Factory with auto-resolved dependencies
dshot.Register(
    dshot.BindAutoFactory(serviceToken, func(config *Config, db *Database) *Service {
        return NewService(config, db)
    }),
)

// Factory with error handling
dshot.Register(
    dshot.BindAutoFactoryErr(dbToken, func(config *Config) (*Database, error) {
        return sql.Open("postgres", config.DBUrl)
    }),
)
```

### Struct Injection

```go
type Dependencies struct {
    Config   *Config
    Database *Database
    Logger   *Logger
}

var deps Dependencies
dshot.Inject(&deps)

// Now use deps.Config, deps.Database, deps.Logger
```

## Context Integration

Store and retrieve containers from `context.Context` - the idiomatic Go way for request-scoped dependencies.

### HTTP Middleware Pattern

```go
func ContainerMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Create request-scoped container
        reqContainer := dshot.NewScoped(dshot.Default())
        reqdshot.Provide(&RequestContext{
            ID:      uuid.New(),
            UserID:  getUserID(r),
            TraceID: getTraceID(r),
        })
        
        // Store in context
        ctx := dshot.WithContainer(r.Context(), reqContainer)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Resolve from context
    reqCtx := dshot.MustResolveCtx[*RequestContext](ctx)
    config := dshot.MustResolveCtx[*Config](ctx) // Falls back to global
    
    // Use CallCtx for auto-wiring
    service := dshot.CallCtx[*Service](ctx,
        func(config *Config, reqCtx *RequestContext) *Service {
            return NewService(config, reqCtx)
        },
    )
    
    service.Process()
}
```

### gRPC Interceptor Pattern
```go
func ContainerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        reqContainer := dshot.NewScoped(dshot.Default())
        reqdshot.Provide(&RequestMetadata{
            Method: info.FullMethod,
            Time:   time.Now(),
        })
        
        ctx = dshot.WithContainer(ctx, reqContainer)
        return handler(ctx, req)
    }
}

func (s *Service) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    metadata := dshot.MustResolveCtx[*RequestMetadata](ctx)
    db := dshot.MustResolveCtx[*Database](ctx)
    
    return db.FindUser(ctx, req.Id)
}
```

## API Reference

### Type-Based Registration

```go
Provide[T](value T)                      // Register a value
ProvideFactory[T](factory func() T)     // Register a singleton factory
ProvidePrototype[T](factory func() T)   // Register a prototype factory
```


### Token-Based Registration

```go
NewToken[T](name ...string) *Token[T]                    // Create a token
Bind[T](token *Token[T], value T) Registration[T]       // Create a registration
BindFactory[T](token *Token[T], factory func() T)       // Factory registration
BindPrototype[T](token *Token[T], factory func() T)     // Prototype registration
Register(registrations ...registration)                  // Register with tokens
```


### Auto-Wired Registration

```go
BindAutoFactory[T, F](token *Token[T], factory F) Registration[T]
BindAutoFactoryErr[T, F](token *Token[T], factory F) Registration[T]
BindAutoPrototype[T, F](token *Token[T], factory F) Registration[T]
BindAutoPrototypeErr[T, F](token *Token[T], factory F) Registration[T]
```


### Resolution

```go
Get[T](token *Token[T], containers ...*Container) T       // Get by token
Find[T](token *Token[T], containers ...*Container) (T, bool)
Resolve[T](containers ...*Container) (T, bool)             // Resolve by type
MustResolve[T](containers ...*Container) T                 // Panic if not found
ResolveAll[T](containers ...*Container) []T                // Get all of type
```


### Auto-Wiring

```go
Call[T, F](fn F, containers ...*Container) T
CallErr[T, F](fn F, containers ...*Container) (T, error)
CallContext[T, F](ctx context.Context, fn F, containers ...*Container) T
CallContextErr[T, F](ctx context.Context, fn F, containers ...*Container) (T, error)
Inject(target any, containers ...*Container)
Build[T, F](constructor F, containers ...*Container) T
```


### Context Integration

```go
WithContainer(ctx context.Context, c *Container) context.Context
FromContext(ctx context.Context) *Container
GetCtx[T](ctx context.Context, token *Token[T]) T
FindCtx[T](ctx context.Context, token *Token[T]) (T, bool)
ResolveCtx[T](ctx context.Context) (T, bool)
MustResolveCtx[T](ctx context.Context) T
ResolveAllCtx[T](ctx context.Context) []T
InjectCtx(ctx context.Context, target any)
CallCtx[T, F](ctx context.Context, fn F) T
CallCtxErr[T, F](ctx context.Context, fn F) (T, error)
BuildCtx[T, F](ctx context.Context, constructor F) T
```


### Container Management

```go
New() *Container                           // Create isolated container
NewScoped(parent *Container) *Container   // Create scoped container
Default() *Container                       // Get global container
Clear()                                    // Clear global container
```


## Patterns & Best Practices

### 1. Use Type-Based Registration for Simple Cases

```go
// ✅ Simple and clean
dshot.Provide(&Config{...})
dshot.ProvideFactory(func() *Logger { return NewLogger() })
```


### 2. Use Token-Based Registration for Multiple Instances

```go
// ✅ When you need multiple of the same type
primaryDB := dshot.NewToken[*Database]("primary")
replicaDB := dshot.NewToken[*Database]("replica")

dshot.Register(
    dshot.Bind(primaryDB, connectToPrimary()),
    dshot.Bind(replicaDB, connectToReplica()),
)
```


### 3. Use Scoped Containers for Request Lifecycles

```go
// ✅ Request-scoped dependencies
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    reqContainer := dshot.NewScoped(dshot.Default())
    reqdshot.Provide(&RequestContext{...})
    
    ctx := dshot.WithContainer(r.Context(), reqContainer)
    // ... handle request with scoped context
}
```


### 4. Use Auto-Wiring for Complex Constructors

```go
// ✅ Auto-wire instead of manual resolution
service := dshot.Call[*Service](func(
    config *Config,
    db *Database,
    logger *Logger,
    cache *Cache,
) *Service {
    return NewService(config, db, logger, cache)
})
```


### 5. Use Isolated Containers for Testing

```go
// ✅ No global state pollution
func TestMyService(t *testing.T) {
    testContainer := dshot.New()
    testdshot.Provide(&MockDatabase{})
    testdshot.Provide(&TestConfig{})
    
    service := dshot.MustResolve[*Service](testContainer)
    // ... test with isolated dependencies
}
```


### 6. Organize Registrations

```go
// ✅ Group related registrations
func RegisterInfrastructure() {
    dshot.ProvideFactory(func() *Database {
        return ConnectDatabase()
    })
    dshot.ProvideFactory(func() *Cache {
        return ConnectCache()
    })
}

func RegisterServices() {
    dshot.Register(
        dshot.BindAutoFactory(userServiceToken, NewUserService),
        dshot.BindAutoFactory(authServiceToken, NewAuthService),
    )
}

func main() {
    RegisterInfrastructure()
    RegisterServices()
    // ... start app
}
```

## Testing Guide

## Running Tests

```bash
# Run container tests
make test

# Run with HTML coverage report
make test-coverage
```

## Running Benchmarks

```bash
# Run container tests
make benchmark

```