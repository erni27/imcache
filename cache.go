// Package imcache provides a simple thread-safe in-memory cache.
// It supports expiration, sliding expiration, eviction callbacks and sharding.
//
// All imcache functions use the global Cache instance.
// By default the global Cache instance has no shards, no default expiration,
// no default sliding expiration and no eviction callback.
// It can be replaced using the SetInstance function.
//
// The New function creates a new Cache instance.
//
// The NewSharded function creates a new sharded Cache instance.
//
// The Get function returns the value for a given key.
//
// The Set function sets the value for a given key.
//
// The Add function adds the value for a given key only if it doesn't exist.
//
// The Replace function replaces the value for a given key only if it already exists.
//
// The Remove function removes the value for a given key.
//
// The RemoveAll function removes all entries from the cache.
//
// The RemoveStale function removes all expired entries from the cache.
package imcache

import (
	"errors"
)

var (
	// ErrNotFound is returned when an entry is not found in the cache.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned when an entry already exists in the cache.
	ErrAlreadyExists = errors.New("already exists")
	// ErrTypeAssertion is returned when an entry is found in the cache but its value's type is different than the expected type.
	ErrTypeAssertion = errors.New("type assertion failure")
)

// Cache is the interface that wraps the basic cache operations.
type Cache interface {
	// Get returns the value for the given key.
	// If the entry is not found, ErrNotFound is returned.
	// If the entry is found but it has expired, ErrNotFound is returned and the entry is evicted.
	Get(key string) (interface{}, error)
	// Set sets the value for the given key.
	// If the entry already exists, it is replaced.
	// If you don't want to replace an existing entry, use Add func instead.
	// If you don't want to add a new entry if it doesn't exist, use Replace func instead.
	Set(key string, val interface{}, exp Expiration)
	// Add adds the value for the given key.
	// If the entry already exists, ErrAlreadyExists is returned.
	// If you want to replace an existing entry, use Set func instead.
	Add(key string, val interface{}, exp Expiration) error
	// Replace replaces the value for the given key.
	// If the entry is not found, ErrNotFound is returned. If the entry is found but it has expired,
	// ErrNotFound is returned and the entry is evicted.
	// If you want to add a new entry if it doesn't exist, use Set func instead.
	Replace(key string, val interface{}, exp Expiration) error
	// Remove removes the cache entry for the given key.
	// If the entry is found, it is removed and a nil error is returned,
	// otherwise ErrNotFound is returned.
	Remove(key string) error
	// RemoveAll removes all entries.
	// If eviction callback is set, it is called for each removed entry.
	RemoveAll()
	// RemoveStale removes all expired entries.
	// If eviction callback is set, it is called for each removed entry.
	RemoveStale()
}

func init() {
	c = New()
}

// c is the global Cache instance.
var c Cache

// New returns a new non-sharded Cache instance.
//
// By default a returned Cache has no default expiration,
// no default sliding expiration and no eviction callback.
// Option(s) can be used to customize the returned Cache.
func New(opts ...Option) Cache {
	return newShard(opts...)
}

// NewSharded returns a new Cache instance consisting of n shards
// and sharded by the given Hasher32.
// If no special hasher is needed, use NewDefaultHasher32 func to create a default hasher.
// Default hasher uses 32-bit FNV-1a hash function.
// It panics if n is not greater than 0 or hasher is nil.
//
// By default a returned Cache has no default expiration,
// no default sliding expiration and no eviction callback.
// Option(s) can be used to customize the returned Cache.
func NewSharded(n int, h32 Hasher32, opts ...Option) Cache {
	if n <= 0 {
		panic("imcache: number of shards must be greater than 0")
	}
	if h32 == nil {
		panic("imcache: hasher must be not nil")
	}
	shards := make([]*shard, n)
	for i := 0; i < n; i++ {
		shards[i] = newShard(opts...)
	}
	return &sharded{
		shards: shards,
		h32:    h32,
		mask:   uint32(n - 1),
	}
}

type sharded struct {
	shards []*shard
	h32    Hasher32
	mask   uint32
}

func (s *sharded) Get(key string) (interface{}, error) {
	return s.shard(key).Get(key)
}

func (s *sharded) Set(key string, val interface{}, exp Expiration) {
	s.shard(key).Set(key, val, exp)
}

func (s *sharded) Add(key string, val interface{}, exp Expiration) error {
	return s.shard(key).Add(key, val, exp)
}

func (s *sharded) Replace(key string, val interface{}, exp Expiration) error {
	return s.shard(key).Replace(key, val, exp)
}

func (s *sharded) Remove(key string) error {
	return s.shard(key).Remove(key)
}

func (s *sharded) RemoveAll() {
	for _, shard := range s.shards {
		shard.RemoveAll()
	}
}

func (s *sharded) RemoveStale() {
	for _, shard := range s.shards {
		shard.RemoveStale()
	}
}

func (s *sharded) shard(key string) *shard {
	return s.shards[s.h32.Sum32(key)&s.mask]
}

// SetInstance sets the global Cache instance.
// It's not a thread-safe func. It must be called exclusively.
func SetInstance(cache Cache) {
	c = cache
}

// Get returns the value for the given key from the global Cache instance.
// If the entry is not found, ErrNotFound is returned.
// If the entry is found but it has expired, ErrNotFound is returned and the entry is evicted.
// If the entry is found but it has a different type than the type parameter, ErrTypeAssertion is returned.
func GetConcrete[T any](key string) (T, error) {
	var val T
	any, err := c.Get(key)
	if err != nil {
		return val, err
	}
	val, ok := any.(T)
	if !ok {
		return val, ErrTypeAssertion
	}
	return val, nil
}

// Get returns the value for the given key from the global Cache instance.
// If the entry is not found, ErrNotFound is returned.
// If the entry is found but it has expired, ErrNotFound is returned and the entry is evicted.
func Get(key string) (interface{}, error) {
	return c.Get(key)
}

// Set sets the value for the given key in the global Cache instance.
// If the entry already exists, it is replaced.
// If you don't want to replace an existing entry, use Add func instead.
// If you don't want to add a new entry if it doesn't exist, use Replace func instead.
func Set(key string, val interface{}, exp Expiration) {
	c.Set(key, val, exp)
}

// Add adds the value for the given key in the global Cache instance.
// If the entry already exists, ErrAlreadyExists is returned.
// If you want to replace an existing entry, use Set func instead.
func Add(key string, val interface{}, exp Expiration) error {
	return c.Add(key, val, exp)
}

// Replace replaces the value for the given key in the global Cache instance.
// If the entry is not found, ErrNotFound is returned. If the entry is found but it has expired,
// ErrNotFound is returned and the entry is evicted.
// If you want to add a new entry if it doesn't exist, use Set func instead.
func Replace(key string, val interface{}, exp Expiration) error {
	return c.Replace(key, val, exp)
}

// Remove removes the cache entry for the given key from the global Cache instance.
// If the entry is found, it is removed and a nil error is returned,
// otherwise ErrNotFound is returned.
func Remove(key string) error {
	return c.Remove(key)
}

// RemoveAll removes all entries from the global Cache instance.
// If eviction callback is set, it is called for each removed entry.
func RemoveAll() {
	c.RemoveAll()
}

// RemoveStale removes all expired entries from the global Cache instance.
// If eviction callback is set, it is called for each removed entry.
func RemoveStale() {
	c.RemoveStale()
}
