package dshot

import "reflect"

type Token[T any] struct {
	key string
}

type tokenKey struct {
	key string
}

// NewToken creates a new typed token for dependency injection.
// Optionally accepts a name; otherwise generates one from the type.
func NewToken[T any](name ...string) *Token[T] {
	var key string

	if len(name) > 0 && name[0] != "" {
		key = name[0]
	} else {
		var zero T
		typ := reflect.TypeOf(zero)

		// Handle nil interface case
		if typ == nil {
			panic("cannot create token for nil interface without explicit name")
		}

		// For pointer types, use the underlying type name
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}

		key = typ.PkgPath() + "." + typ.Name()

		// Fallback for anonymous types
		if key == "." {
			key = typ.String()
		}
	}

	return &Token[T]{key: key}
}

func (t *Token[T]) String() string {
	return t.key
}
