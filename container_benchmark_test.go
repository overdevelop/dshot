package dshot_test

import (
	"reflect"
	"testing"

	"github.com/overdevelop/dshot"
)

func BenchmarkProvide(b *testing.B) {
	c := dshot.New()
	svc := &Service{Name: "Benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Provide(svc)
	}
}

func BenchmarkResolve(b *testing.B) {
	c := dshot.New()
	c.Provide(&Service{Name: "Benchmark"})
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Resolve(typ)
	}
}

func BenchmarkResolve_WithToken(b *testing.B) {
	c := dshot.New()
	token := dshot.NewToken[*Service]("bench-service")
	c.Register(dshot.Bind(token, &Service{Name: "Benchmark"}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(token)
	}
}

func BenchmarkSingletonFactory(b *testing.B) {
	c := dshot.New()
	c.ProvideFactory(
		func() *Service {
			return &Service{Name: "Benchmark"}
		},
	)
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Resolve(typ)
	}
}

func BenchmarkPrototypeFactory(b *testing.B) {
	c := dshot.New()
	c.ProvidePrototype(
		func() *Service {
			return &Service{Name: "Benchmark"}
		},
	)
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Resolve(typ)
	}
}

func BenchmarkInject(b *testing.B) {
	c := dshot.New()
	c.Provide(&Database{ConnectionString: "localhost:5432"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := &Repository{}
		c.Inject(target)
	}
}

func BenchmarkInject_WithToken(b *testing.B) {
	c := dshot.New()
	token := dshot.NewToken[*Database]("db")
	c.Register(dshot.Bind(token, &Database{ConnectionString: "localhost:5432"}))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := &Repository{}
		c.Inject(target)
	}
}

func BenchmarkCall(b *testing.B) {
	c := dshot.New()
	c.Provide(&Service{Name: "Benchmark"})
	c.Provide(&Repository{})

	fn := func(svc *Service, repo *Repository) error {
		if svc == nil || repo == nil {
			panic("nil service or repo")
		}

		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dshot.Invoke(fn, c)
	}
}

func BenchmarkFallbackToParent(b *testing.B) {
	parent := dshot.New()
	parent.Provide(&Service{Name: "Parent"})

	scoped := dshot.NewScoped(parent)
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scoped.Resolve(typ)
	}
}

func BenchmarkScopedResolve(b *testing.B) {
	parent := dshot.New()
	parent.Provide(&Service{Name: "Parent"})

	scoped := dshot.NewScoped(parent)
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scoped.Resolve(typ)
	}
}

func BenchmarkResolveAll(b *testing.B) {
	c := dshot.New()
	for i := 0; i < 10; i++ {
		c.Provide(&Service{Name: "Benchmark"})
	}
	typ := reflect.TypeFor[*Service]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.ResolveAll(typ)
	}
}

func BenchmarkConcurrentResolve(b *testing.B) {
	c := dshot.New()
	c.ProvideFactory(
		func() *Service {
			return &Service{Name: "Concurrent"}
		},
	)
	typ := reflect.TypeOf((*Service)(nil))

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				c.Resolve(typ)
			}
		},
	)
}

func BenchmarkToken_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dshot.NewToken[*Service]()
	}
}

func BenchmarkToken_CreationWithName(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dshot.NewToken[*Service]("named-token")
	}
}
