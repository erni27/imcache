// Package imcache provides a generic in-memory cache.
// It supports expiration, sliding expiration, eviction callbacks and sharding.
// It's safe for concurrent use by multiple goroutines.
//
// The New function creates a new Cache instance.
//
// The NewSharded function creates a new sharded Cache instance.
package imcache

import (
	"errors"
)

var (
	// ErrNotFound is returned when an entry for a given key is not found in the cache.
	ErrNotFound = errors.New("imcache: not found")
	// ErrAlreadyExists is returned when an entry for a given key already exists in the cache.
	ErrAlreadyExists = errors.New("imcache: already exists")
)

// Cache is the interface that wraps the basic cache operations.
type Cache[K comparable, V any] interface {
	// Get returns the value for the given key.
	// If the entry is not found, ErrNotFound is returned.
	// If the entry is found but it has expired, ErrNotFound is returned and the entry is evicted.
	Get(key K) (V, error)
	// Set sets the value for the given key.
	// If the entry already exists, it is replaced.
	// If you don't want to replace an existing entry, use Add func instead.
	// If you don't want to add a new entry if it doesn't exist, use Replace func instead.
	Set(key K, val V, exp Expiration)
	// Add adds the value for the given key.
	// If the entry already exists, ErrAlreadyExists is returned.
	// If you want to replace an existing entry, use Set func instead.
	Add(key K, val V, exp Expiration) error
	// Replace replaces the value for the given key.
	// If the entry is not found, ErrNotFound is returned. If the entry is found but it has expired,
	// ErrNotFound is returned and the entry is evicted.
	// If you want to add a new entry if it doesn't exist, use Set func instead.
	Replace(key K, val V, exp Expiration) error
	// Remove removes the cache entry for the given key.
	// If the entry is found, it is removed and a nil error is returned,
	// otherwise ErrNotFound is returned.
	Remove(key K) error
	// RemoveAll removes all entries.
	// If eviction callback is set, it is called for each removed entry.
	RemoveAll()
	// RemoveStale removes all expired entries.
	// If eviction callback is set, it is called for each removed entry.
	RemoveStale()
}

// New returns a new non-sharded Cache instance.
//
// By default a returned Cache has no default expiration,
// no default sliding expiration and no eviction callback.
// Option(s) can be used to customize the returned Cache.
func New[K comparable, V any](opts ...Option[K, V]) Cache[K, V] {
	return newShard(opts...)
}

// NewSharded returns a new Cache instance consisting of n shards
// and sharded by the given Hasher64.
//
// By default a returned Cache has no default expiration,
// no default sliding expiration and no eviction callback.
// Option(s) can be used to customize the returned Cache.
func NewSharded[K comparable, V any](n int, hasher Hasher64[K], opts ...Option[K, V]) Cache[K, V] {
	if n <= 0 {
		panic("imcache: number of shards must be greater than 0")
	}
	if hasher == nil {
		panic("imcache: hasher must be not nil")
	}
	shards := make([]*shard[K, V], n)
	for i := 0; i < n; i++ {
		shards[i] = newShard(opts...)
	}
	return &sharded[K, V]{
		shards: shards,
		hasher: hasher,
		mask:   uint64(n - 1),
	}
}

type sharded[K comparable, V any] struct {
	shards []*shard[K, V]
	hasher Hasher64[K]
	mask   uint64
}

func (s *sharded[K, V]) Get(key K) (V, error) {
	return s.shard(key).Get(key)
}

func (s *sharded[K, V]) Set(key K, val V, exp Expiration) {
	s.shard(key).Set(key, val, exp)
}

func (s *sharded[K, V]) Add(key K, val V, exp Expiration) error {
	return s.shard(key).Add(key, val, exp)
}

func (s *sharded[K, V]) Replace(key K, val V, exp Expiration) error {
	return s.shard(key).Replace(key, val, exp)
}

func (s *sharded[K, V]) Remove(key K) error {
	return s.shard(key).Remove(key)
}

func (s *sharded[K, V]) RemoveAll() {
	for _, shard := range s.shards {
		shard.RemoveAll()
	}
}

func (s *sharded[K, V]) RemoveStale() {
	for _, shard := range s.shards {
		shard.RemoveStale()
	}
}

func (s *sharded[K, V]) shard(key K) *shard[K, V] {
	return s.shards[s.hasher.Sum64(key)&s.mask]
}
