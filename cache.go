// Package imcache provides a generic in-memory cache.
// It supports expiration, sliding expiration, eviction callbacks and sharding.
// It's safe for concurrent use by multiple goroutines.
//
// The New function creates a new Cache instance.
//
// The NewSharded function creates a new sharded Cache instance.
package imcache

import (
	"sync"
	"time"
)

// Cache is the interface that wraps cache operations.
type Cache[K comparable, V any] interface {
	// Get returns the value for the given key.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	Get(key K) (v V, present bool)
	// Set sets the value for the given key.
	// If the entry already exists, it is replaced.
	//
	// If it encounters an expired entry, it is evicted and a new entry is added.
	//
	// If you don't want to replace an existing entry, use the GetOrSet method instead.
	// If you don't want to add a new entry if it doesn't exist, use the Replace method instead.
	Set(key K, val V, exp Expiration)
	// GetOrSet returns the value for the given key and true if it exists,
	// otherwise it sets the value for the given key and returns the set value and false.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	GetOrSet(key K, val V, exp Expiration) (v V, present bool)
	// Replace replaces the value for the given key.
	// It returns true if the value is present and replaced, otherwise it returns false.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	//
	// If you want to add or replace an entry, use the Set method instead.
	Replace(key K, val V, exp Expiration) (present bool)
	// Remove removes the cache entry for the given key.
	//
	// It returns true if the entry is present and removed,
	// otherwise it returns false.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	// It results in calling the eviction callback with EvictionReasonExpired,
	// not EvictionReasonRemoved. If entry is expired, it returns false.
	Remove(key K) (present bool)
	// RemoveAll removes all entries.
	//
	// If eviction callback is set, it is called for each removed entry.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	// It results in calling the eviction callback with EvictionReasonExpired,
	// not EvictionReasonRemoved.
	RemoveAll()
	// RemoveStale removes all expired entries.
	//
	// If eviction callback is set, it is called for each removed entry.
	RemoveStale()
	// GetAll returns a copy of all entries in the cache.
	//
	// If it encounters an expired entry, the expired entry is evicted.
	GetAll() map[K]V
	// Len returns the number of entries in the cache.
	Len() int
	// StartCleaner starts a cleaner that periodically removes expired entries.
	// A cleaner runs in a separate goroutine.
	// It's a NOP method if the cleaner is already running.
	// It panics if the interval is less than or equal to zero.
	//
	// The cleaner can be stopped by calling StopCleaner method.
	StartCleaner(interval time.Duration)
	// StopCleaner stops the cleaner.
	// It is a blocking method that waits for the cleaner to stop.
	// It's a NOP method if the cleaner is not running.
	StopCleaner()
}

// New returns a new non-sharded Cache instance.
//
// By default a returned Cache has no default expiration,
// no default sliding expiration and no eviction callback.
// Option(s) can be used to customize the returned Cache.
func New[K comparable, V any](opts ...Option[K, V]) Cache[K, V] {
	return newShard(opts...)
}

func newShard[K comparable, V any](opts ...Option[K, V]) *shard[K, V] {
	s := &shard[K, V]{
		m:          make(map[K]entry[V]),
		defaultExp: -1,
		cleaner:    newCleaner(),
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// shard is a non-sharded cache.
type shard[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]entry[V]

	defaultExp time.Duration
	sliding    bool

	onEviction EvictionCallback[K, V]

	cleaner *cleaner
}

func (s *shard[K, V]) Get(key K) (V, bool) {
	now := time.Now()
	var empty V
	s.mu.Lock()
	entry, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return empty, false
	}
	if entry.HasExpired(now) {
		delete(s.m, key)
		s.mu.Unlock()
		if s.onEviction != nil {
			s.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return empty, false
	}
	if entry.HasSlidingExpiration() {
		entry.SlideExpiration(now)
		s.m[key] = entry
	}
	s.mu.Unlock()
	return entry.val, true
}

func (s *shard[K, V]) Set(key K, val V, exp Expiration) {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	s.m[key] = entry
	s.mu.Unlock()
	if ok && s.onEviction != nil {
		if current.HasExpired(now) {
			s.onEviction(key, current.val, EvictionReasonExpired)
		} else {
			s.onEviction(key, current.val, EvictionReasonReplaced)
		}
	}
}

func (s *shard[K, V]) GetOrSet(key K, val V, exp Expiration) (V, bool) {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	if ok && !current.HasExpired(now) {
		s.mu.Unlock()
		return current.val, true
	}
	s.m[key] = entry
	s.mu.Unlock()
	if ok && s.onEviction != nil {
		s.onEviction(key, current.val, EvictionReasonExpired)
	}
	return entry.val, false
}

func (s *shard[K, V]) Replace(key K, val V, exp Expiration) bool {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return false
	}
	if current.HasExpired(now) {
		delete(s.m, key)
		s.mu.Unlock()
		if s.onEviction != nil {
			s.onEviction(key, current.val, EvictionReasonExpired)
		}
		return false
	}
	s.m[key] = entry
	s.mu.Unlock()
	if s.onEviction != nil {
		s.onEviction(key, current.val, EvictionReasonReplaced)
	}
	return true
}

func (s *shard[K, V]) Remove(key K) bool {
	now := time.Now()
	s.mu.Lock()
	entry, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return false
	}
	delete(s.m, key)
	s.mu.Unlock()
	if entry.HasExpired(now) {
		if s.onEviction != nil {
			s.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return false
	}
	if s.onEviction != nil {
		s.onEviction(key, entry.val, EvictionReasonRemoved)
	}
	return true
}

func (s *shard[K, V]) removeAll(now time.Time) {
	s.mu.Lock()
	removed := s.m
	s.m = make(map[K]entry[V])
	s.mu.Unlock()
	if s.onEviction != nil {
		for key, entry := range removed {
			if entry.HasExpired(now) {
				s.onEviction(key, entry.val, EvictionReasonExpired)
			} else {
				s.onEviction(key, entry.val, EvictionReasonRemoved)
			}
		}
	}
}

func (s *shard[K, V]) RemoveAll() {
	s.removeAll(time.Now())
}

func (s *shard[K, V]) removeStale(now time.Time) {
	s.mu.Lock()
	// To avoid copying the expired entries if there's no eviction callback.
	if s.onEviction == nil {
		for key, entry := range s.m {
			if entry.HasExpired(now) {
				delete(s.m, key)
			}
		}
		s.mu.Unlock()
		return
	}
	var removed []kv[K, V]
	for key, entry := range s.m {
		if entry.HasExpired(now) {
			removed = append(removed, kv[K, V]{key, entry.val})
			delete(s.m, key)
		}
	}
	s.mu.Unlock()
	for _, kv := range removed {
		s.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
}

func (s *shard[K, V]) RemoveStale() {
	s.removeStale(time.Now())
}

type kv[K comparable, V any] struct {
	key K
	val V
}

func (s *shard[K, V]) getAll(now time.Time) map[K]V {
	s.mu.Lock()
	// To avoid copying the expired entries if there's no eviction callback.
	if s.onEviction == nil {
		m := make(map[K]V, len(s.m))
		for key, entry := range s.m {
			if entry.HasExpired(now) {
				delete(s.m, key)
			} else {
				if entry.HasSlidingExpiration() {
					entry.SlideExpiration(now)
					s.m[key] = entry
				}
				m[key] = entry.val
			}
		}
		s.mu.Unlock()
		return m
	}
	var expired []kv[K, V]
	m := make(map[K]V, len(s.m))
	for key, entry := range s.m {
		if entry.HasExpired(now) {
			expired = append(expired, kv[K, V]{key: key, val: entry.val})
			delete(s.m, key)
		} else {
			if entry.HasSlidingExpiration() {
				entry.SlideExpiration(now)
				s.m[key] = entry
			}
			m[key] = entry.val
		}
	}
	s.mu.Unlock()
	for _, kv := range expired {
		s.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
	return m
}

func (s *shard[K, V]) GetAll() map[K]V {
	return s.getAll(time.Now())
}

func (s *shard[K, V]) Len() int {
	s.mu.Lock()
	n := len(s.m)
	s.mu.Unlock()
	return n
}

func (s *shard[K, V]) StartCleaner(interval time.Duration) {
	s.cleaner.start(s, interval)
}

func (s *shard[K, V]) StopCleaner() {
	s.cleaner.stop()
}

// NewSharded returns a new Cache instance consisting of n shards
// and sharded by the given Hasher64.
// It panics if n is not greater than 0 or hasher is nil.
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
		shards:  shards,
		hasher:  hasher,
		mask:    uint64(n - 1),
		cleaner: newCleaner(),
	}
}

// sharded is a sharded cache.
type sharded[K comparable, V any] struct {
	shards []*shard[K, V]
	hasher Hasher64[K]
	mask   uint64

	cleaner *cleaner
}

func (s *sharded[K, V]) shard(key K) Cache[K, V] {
	return s.shards[s.hasher.Sum64(key)&s.mask]
}

func (s *sharded[K, V]) Get(key K) (V, bool) {
	return s.shard(key).Get(key)
}

func (s *sharded[K, V]) Set(key K, val V, exp Expiration) {
	s.shard(key).Set(key, val, exp)
}

func (s *sharded[K, V]) GetOrSet(key K, val V, exp Expiration) (v V, present bool) {
	return s.shard(key).GetOrSet(key, val, exp)
}

func (s *sharded[K, V]) Replace(key K, val V, exp Expiration) bool {
	return s.shard(key).Replace(key, val, exp)
}

func (s *sharded[K, V]) Remove(key K) bool {
	return s.shard(key).Remove(key)
}

func (s *sharded[K, V]) RemoveAll() {
	now := time.Now()
	for _, shard := range s.shards {
		shard.removeAll(now)
	}
}

func (s *sharded[K, V]) RemoveStale() {
	now := time.Now()
	for _, shard := range s.shards {
		shard.removeStale(now)
	}
}

func (s *sharded[K, V]) GetAll() map[K]V {
	now := time.Now()
	var n int
	ms := make([]map[K]V, 0, len(s.shards))
	for _, shard := range s.shards {
		m := shard.getAll(now)
		n += len(m)
		ms = append(ms, m)
	}
	all := make(map[K]V, n)
	for _, m := range ms {
		for key, val := range m {
			all[key] = val
		}
	}
	return all
}

func (s *sharded[K, V]) Len() int {
	var n int
	for _, shard := range s.shards {
		n += shard.Len()
	}
	return n
}

func (s *sharded[K, V]) StartCleaner(interval time.Duration) {
	s.cleaner.start(s, interval)
}

func (s *sharded[K, V]) StopCleaner() {
	s.cleaner.stop()
}
