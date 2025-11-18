package dshot_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/overdevelop/dshot"
)

// Test types
type Service struct {
	Name string
}

type Database struct {
	ConnectionString string
}

type Repository struct {
	DB *Database
}

type ComplexService struct {
	Repo    *Repository
	Service *Service
}

func TestNew(t *testing.T) {
	c := dshot.New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.Parent() != nil {
		t.Error("New dshot should not have a parent")
	}
}

func TestNewScoped(t *testing.T) {
	parent := dshot.New()
	scoped := dshot.NewScoped(parent)

	if scoped == nil {
		t.Fatal("NewScoped() returned nil")
	}
	if scoped.Parent() != parent {
		t.Error("Scoped dshot should have correct parent")
	}
}

func TestNewScoped_NilParentPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil parent")
		}
	}()
	dshot.NewScoped(nil)
}

func TestProvide(t *testing.T) {
	c := dshot.New()
	svc := &Service{Name: "TestService"}
	c.Provide(svc)

	resolved, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve provided service")
	}

	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc.Name != "TestService" {
		t.Errorf("Expected name 'TestService', got '%s'", resolvedSvc.Name)
	}
}

func TestProvide_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil value")
		}
	}()
	c := dshot.New()
	c.Provide(nil)
}

func TestProvideFactory(t *testing.T) {
	c := dshot.New()
	callCount := 0

	factory := func() *Service {
		callCount++
		return &Service{Name: "Factory"}
	}

	c.ProvideFactory(factory)

	// First resolution
	resolved1, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve factory service")
	}

	// Second resolution - should return same instance (singleton)
	resolved2, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve factory service second time")
	}

	if callCount != 1 {
		t.Errorf("Expected factory to be called once, got %d", callCount)
	}

	if resolved1 != resolved2 {
		t.Error("Singleton factory should return same instance")
	}
}

func TestProvidePrototype(t *testing.T) {
	c := dshot.New()
	callCount := 0

	factory := func() *Service {
		callCount++
		return &Service{Name: "Prototype"}
	}

	c.ProvidePrototype(factory)

	// First resolution
	resolved1, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve prototype service")
	}

	// Second resolution - should return new instance (prototype)
	resolved2, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve prototype service second time")
	}

	if callCount != 2 {
		t.Errorf("Expected factory to be called twice, got %d", callCount)
	}

	if resolved1 == resolved2 {
		t.Error("Prototype factory should return different instances")
	}
}

func TestRegisterWithToken(t *testing.T) {
	c := dshot.New()
	token := dshot.NewToken[*Service]("my-service")
	svc := &Service{Name: "TokenService"}

	c.Register(dshot.Bind(token, svc))

	resolved := c.Get(token)
	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc.Name != "TokenService" {
		t.Errorf("Expected name 'TokenService', got '%s'", resolvedSvc.Name)
	}
}

func TestRegisterFactoryWithToken(t *testing.T) {
	c := dshot.New()
	token := dshot.NewToken[*Service]("factory-service")

	c.Register(dshot.BindAutoFactory(token, func() *Service {
		return &Service{Name: "FactoryService"}
	}))

	resolved := c.Get(token)
	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc.Name != "FactoryService" {
		t.Errorf("Expected name 'FactoryService', got '%s'", resolvedSvc.Name)
	}
}

func TestRegisterPrototypeWithToken(t *testing.T) {
	c := dshot.New()
	token := dshot.NewToken[*Service]("prototype-service")
	callCount := 0

	c.Register(dshot.BindAutoPrototype(token, func() *Service {
		callCount++
		return &Service{Name: "PrototypeService"}
	}))

	resolved1 := c.Get(token)
	resolved2 := c.Get(token)

	if callCount != 2 {
		t.Errorf("Expected factory to be called twice, got %d", callCount)
	}

	if resolved1 == resolved2 {
		t.Error("Prototype should return different instances")
	}
}

func TestGet_NotFoundPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for missing dependency")
		}
	}()

	c := dshot.New()
	token := dshot.NewToken[*Service]("missing")
	c.Get(token)
}

func TestResolve(t *testing.T) {
	c := dshot.New()
	svc := &Service{Name: "ResolveTest"}
	c.Provide(svc)

	resolved, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve service")
	}

	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc != svc {
		t.Error("Resolved service should be the same instance")
	}
}

func TestResolve_NotFound(t *testing.T) {
	c := dshot.New()

	_, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if ok {
		t.Error("Should not resolve non-existent type")
	}
}

func TestResolveAll(t *testing.T) {
	c := dshot.New()

	svc1 := &Service{Name: "Service1"}
	svc2 := &Service{Name: "Service2"}

	c.Provide(svc1)
	c.Provide(svc2)

	results := c.ResolveAll(reflect.TypeOf((*Service)(nil)))

	if len(results) != 2 {
		t.Errorf("Expected 2 services, got %d", len(results))
	}
}

func TestResolveAll_WithParent(t *testing.T) {
	parent := dshot.New()
	parent.Provide(&Service{Name: "ParentService"})

	scoped := dshot.NewScoped(parent)
	scoped.Provide(&Service{Name: "ScopedService"})

	results := scoped.ResolveAll(reflect.TypeOf((*Service)(nil)))

	// Should get both parent and scoped services
	if len(results) < 2 {
		t.Errorf("Expected at least 2 services, got %d", len(results))
	}
}

func TestInject(t *testing.T) {
	c := dshot.New()

	db := &Database{ConnectionString: "localhost:5432"}
	c.Provide(db)

	target := &Repository{}
	c.Inject(target)

	if target.DB == nil {
		t.Fatal("DB field was not injected")
	}

	if target.DB.ConnectionString != "localhost:5432" {
		t.Errorf("Expected connection string 'localhost:5432', got '%s'", target.DB.ConnectionString)
	}
}

func TestInject_NestedStruct(t *testing.T) {
	c := dshot.New()

	db := &Database{ConnectionString: "localhost:5432"}
	svc := &Service{Name: "InjectedService"}
	repo := &Repository{DB: db}

	c.Provide(db)
	c.Provide(svc)
	c.Provide(repo)

	target := &ComplexService{}
	c.Inject(target)

	if target.Service == nil {
		t.Fatal("Service field was not injected")
	}

	if target.Repo == nil {
		t.Fatal("Repo field was not injected")
	}

	if target.Repo.DB == nil {
		t.Fatal("Nested DB field was not injected")
	}
}

func TestInject_NotPointerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-pointer target")
		}
	}()

	c := dshot.New()
	c.Inject(Service{})
}

func TestInject_MissingDependencyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for missing dependency")
		}
	}()

	c := dshot.New()
	target := &Repository{}
	c.Inject(target)
}

func TestScopeddshot_FallbackToParent(t *testing.T) {
	parent := dshot.New()
	parentSvc := &Service{Name: "ParentService"}
	parent.Provide(parentSvc)

	scoped := dshot.NewScoped(parent)

	// Should resolve from parent
	resolved, ok := scoped.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve from parent")
	}

	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc != parentSvc {
		t.Error("Should resolve parent's service")
	}
}

func TestScopeddshot_LocalOverridesParent(t *testing.T) {
	parent := dshot.New()
	parent.Provide(&Service{Name: "ParentService"})

	scoped := dshot.NewScoped(parent)
	scopedSvc := &Service{Name: "ScopedService"}
	scoped.Provide(scopedSvc)

	// Should resolve from scoped, not parent
	resolved, ok := scoped.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Failed to resolve from scoped")
	}

	resolvedSvc, ok := resolved.(*Service)
	if !ok {
		t.Fatal("Resolved value is not *Service")
	}

	if resolvedSvc.Name != "ScopedService" {
		t.Error("Should resolve scoped service, not parent")
	}
}

func TestClear(t *testing.T) {
	c := dshot.New()
	c.Provide(&Service{Name: "Test"})

	_, ok := c.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Fatal("Service should be registered")
	}

	c.Clear()

	_, ok = c.Resolve(reflect.TypeOf((*Service)(nil)))
	if ok {
		t.Error("Service should be cleared")
	}
}

func TestClear_DoesNotAffectParent(t *testing.T) {
	parent := dshot.New()
	parent.Provide(&Service{Name: "ParentService"})

	scoped := dshot.NewScoped(parent)
	scoped.Provide(&Database{ConnectionString: "scoped"})

	scoped.Clear()

	// Parent should still have its service
	_, ok := parent.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Error("Parent service should still exist after scoped clear")
	}

	// Scoped should be able to resolve from parent
	_, ok = scoped.Resolve(reflect.TypeOf((*Service)(nil)))
	if !ok {
		t.Error("Scoped should still resolve from parent after clear")
	}
}

func TestConcurrency_SingletonFactory(t *testing.T) {
	c := dshot.New()
	callCount := 0
	var mu sync.Mutex

	factory := func() *Service {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &Service{Name: "Concurrent"}
	}

	c.ProvideFactory(factory)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Resolve(reflect.TypeOf((*Service)(nil)))
		}()
	}

	wg.Wait()

	if callCount != 1 {
		t.Errorf("Expected factory to be called once despite concurrency, got %d", callCount)
	}
}

func TestConcurrency_PrototypeFactory(t *testing.T) {
	c := dshot.New()
	callCount := 0
	var mu sync.Mutex

	factory := func() *Service {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &Service{Name: "Concurrent"}
	}

	c.ProvidePrototype(factory)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Resolve(reflect.TypeOf((*Service)(nil)))
		}()
	}

	wg.Wait()

	if callCount != iterations {
		t.Errorf("Expected factory to be called %d times, got %d", iterations, callCount)
	}
}

func TestMultipleRegistrations(t *testing.T) {
	c := dshot.New()

	token1 := dshot.NewToken[*Service]("service1")
	token2 := dshot.NewToken[*Service]("service2")
	token3 := dshot.NewToken[*Database]("db")

	c.Register(
		dshot.Bind(token1, &Service{Name: "Service1"}),
		dshot.Bind(token2, &Service{Name: "Service2"}),
		dshot.Bind(token3, &Database{ConnectionString: "localhost"}),
	)

	svc1 := c.Get(token1).(*Service)
	svc2 := c.Get(token2).(*Service)
	db := c.Get(token3).(*Database)

	if svc1.Name != "Service1" {
		t.Errorf("Expected Service1, got %s", svc1.Name)
	}

	if svc2.Name != "Service2" {
		t.Errorf("Expected Service2, got %s", svc2.Name)
	}

	if db.ConnectionString != "localhost" {
		t.Errorf("Expected localhost, got %s", db.ConnectionString)
	}
}

func TestToken_WithName(t *testing.T) {
	token := dshot.NewToken[*Service]("custom-name")
	if token.String() != "custom-name" {
		t.Errorf("Expected token name 'custom-name', got '%s'", token.String())
	}
}

func TestToken_WithoutName(t *testing.T) {
	token := dshot.NewToken[*Service]()
	tokenStr := token.String()

	// Should contain package path and type name
	if tokenStr == "" {
		t.Error("Token string should not be empty")
	}
}

func TestEntry_Resolve_Singleton(t *testing.T) {
	c := dshot.New()
	callCount := 0

	c.ProvideFactory(func() *Service {
		callCount++
		return &Service{Name: "Singleton"}
	})

	// Multiple resolutions
	c.Resolve(reflect.TypeOf((*Service)(nil)))
	c.Resolve(reflect.TypeOf((*Service)(nil)))
	c.Resolve(reflect.TypeOf((*Service)(nil)))

	if callCount != 1 {
		t.Errorf("Singleton factory should be called once, got %d", callCount)
	}
}

func TestEntry_Resolve_Prototype(t *testing.T) {
	c := dshot.New()
	callCount := 0

	c.ProvidePrototype(func() *Service {
		callCount++
		return &Service{Name: "Prototype"}
	})

	// Multiple resolutions
	c.Resolve(reflect.TypeOf((*Service)(nil)))
	c.Resolve(reflect.TypeOf((*Service)(nil)))
	c.Resolve(reflect.TypeOf((*Service)(nil)))

	if callCount != 3 {
		t.Errorf("Prototype factory should be called 3 times, got %d", callCount)
	}
}

func TestEntry_Resolve_Value(t *testing.T) {
	c := dshot.New()
	svc := &Service{Name: "Value"}
	c.Provide(svc)

	resolved1, _ := c.Resolve(reflect.TypeOf((*Service)(nil)))
	resolved2, _ := c.Resolve(reflect.TypeOf((*Service)(nil)))

	if resolved1 != resolved2 {
		t.Error("Value should return same instance")
	}

	if resolved1 != svc {
		t.Error("Resolved value should be the original instance")
	}
}
