package generics

import "fmt"

// Number is a numeric constraint.
type Number interface {
	~int | ~int64 | ~float64
}

// Opt is an optional value.
type Opt[T comparable] struct {
	// Value is the value.
	Value T
	set   bool
}

// NewOpt returns a set [Opt].
func NewOpt[T comparable](v T) Opt[T] {
	return Opt[T]{Value: v, set: true}
}

// Or returns the value or fallback.
func (o Opt[T]) Or(fallback T) T {
	return fallback
}

// Pair holds two values.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// Item is an item.
type Item struct{}

// Sum adds numbers. Its type parameter shadows [Item].
func Sum[Item Number](values ...Item) Item {
	var total Item
	return total
}

// Lookup returns the [Pair] for key.
func Lookup[K comparable, V any](pairs []Pair[K, V], key K) (Opt[V], bool) {
	var zero Opt[V]
	return zero, false
}

// Stringish is satisfied by string types that can print themselves.
type Stringish interface {
	~string
	fmt.Stringer
	String() string
}

// Base is embedded by value.
type Base struct{ ID string }

// Meta is embedded by pointer.
type Meta struct{ Version int }

// Page is a generic type embedded with a type argument.
type Page[T any] struct{ Items []T }

// Resource embeds a local type by value, by pointer, and generically.
type Resource struct {
	Base
	*Meta
	Page[Item]
}

// Store is an interface embedded by name.
type Store interface {
	Get(key string) (string, bool)
}

// Constraint mixes predeclared interfaces, a local interface, and type terms.
type Constraint interface {
	error
	any
	Store
	int | string
	int
}
