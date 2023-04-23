// Package imcache provides a generic in-memory cache.
// It supports absolute expiration, sliding expiration, max entries limit,
// eviction callbacks and sharding.
// It's safe for concurrent use by multiple goroutines.
//
// The New function creates a new in-memory non-sharded cache instance.
//
// The NewSharded function creates a new in-memory sharded cache instance.
package imcache

import (
	"sync"
	"time"
)

// New returns a new Cache instance.
//
// By default a returned Cache has no default expiration, no default sliding
// expiration, no entry limit and no eviction callback.
//
// Option(s) can be used to customize the returned Cache.
func New[K comparable, V any](opts ...Option[K, V]) *Cache[K, V] {
	s := &Cache[K, V]{
		m:          make(map[K]entry[K, V]),
		defaultExp: -1,
		cleaner:    newCleaner(),
		queue:      &nopq[K]{},
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

// Cache is a non-sharded in-memory cache.
//
// By default it has no default expiration, no default sliding expiration
// no entry limit and no eviction callback.
//
// The zero value Cache is ready to use.
//
// If you want to configure a Cache, use the New function
// and provide proper Option(s).
//
// Example:
//
//	c := imcache.New[string, interface{}](
//		imcache.WithDefaultExpirationOption[string, interface{}](time.Second),
//		imcache.WithMaxEntriesOption[string, interface{}](10000),
//		imcache.WithEvictionCallbackOption[string, interface{}](LogEvictedEntry),
//	)
type Cache[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]entry[K, V]

	queue evictionq[K]
	size  int

	defaultExp time.Duration
	sliding    bool

	onEviction EvictionCallback[K, V]

	cleaner *cleaner
}

// init initializes the Cache.
// It is not a concurrency-safe method.
func (s *Cache[K, V]) init() {
	if s.m == nil {
		s.m = make(map[K]entry[K, V])
		s.queue = &nopq[K]{}
	}
}

// Get returns the value for the given key.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()
	var empty V
	c.mu.Lock()
	current, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return empty, false
	}
	c.queue.Remove(current.node)
	if current.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			c.onEviction(key, current.val, EvictionReasonExpired)
		}
		return empty, false
	}
	if current.HasSlidingExpiration() {
		current.SlideExpiration(now)
		c.m[key] = current
	}
	c.queue.Add(current.node)
	c.mu.Unlock()
	return current.val, true
}

// Set sets the value for the given key.
// If the entry already exists, it is replaced.
//
// If it encounters an expired entry, it is evicted and a new entry is added.
//
// If you don't want to replace an existing entry, use the GetOrSet method
// instead. If you don't want to add a new entry if it doesn't exist, use
// the Replace method instead.
func (c *Cache[K, V]) Set(key K, val V, exp Expiration) {
	now := time.Now()
	new := entry[K, V]{val: val}
	exp.apply(&new.exp)
	new.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	// Make sure that the shard is initialized.
	c.init()
	new.node = c.queue.AddNew(key)
	current, ok := c.m[key]
	c.m[key] = new
	if ok {
		c.queue.Remove(current.node)
		c.mu.Unlock()
		if c.onEviction != nil {
			if current.HasExpired(now) {
				c.onEviction(key, current.val, EvictionReasonExpired)
			} else {
				c.onEviction(key, current.val, EvictionReasonReplaced)
			}
		}
		return
	}
	if c.size <= 0 || c.len() <= c.size {
		c.mu.Unlock()
		return
	}
	node := c.queue.Pop()
	if c.onEviction == nil {
		delete(c.m, node.key)
		c.mu.Unlock()
		return
	}
	toevict := c.m[node.key]
	delete(c.m, node.key)
	c.mu.Unlock()
	if toevict.HasExpired(now) {
		c.onEviction(node.key, toevict.val, EvictionReasonExpired)
	} else {
		c.onEviction(node.key, toevict.val, EvictionReasonMaxEntriesExceeded)
	}
}

// GetOrSet returns the value for the given key and true if it exists,
// otherwise it sets the value for the given key and returns the set value
// and false.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetOrSet(key K, val V, exp Expiration) (V, bool) {
	now := time.Now()
	new := entry[K, V]{val: val}
	exp.apply(&new.exp)
	new.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	// Make sure that the shard is initialized.
	c.init()
	current, ok := c.m[key]
	if !ok {
		new.node = c.queue.AddNew(key)
		c.m[key] = new
		if c.size <= 0 || c.len() <= c.size {
			c.mu.Unlock()
			return val, false
		}
		node := c.queue.Pop()
		if c.onEviction == nil {
			delete(c.m, node.key)
			c.mu.Unlock()
			return val, false
		}
		toevict := c.m[node.key]
		delete(c.m, node.key)
		c.mu.Unlock()
		if toevict.HasExpired(now) {
			c.onEviction(node.key, toevict.val, EvictionReasonExpired)
		} else {
			c.onEviction(node.key, toevict.val, EvictionReasonMaxEntriesExceeded)
		}
		return val, false
	}
	c.queue.Remove(current.node)
	if !current.HasExpired(now) {
		if current.HasSlidingExpiration() {
			current.SlideExpiration(now)
			c.m[key] = current
		}
		c.queue.Add(current.node)
		c.mu.Unlock()
		return current.val, true
	}
	new.node = c.queue.AddNew(key)
	c.m[key] = new
	c.mu.Unlock()
	if c.onEviction != nil {
		c.onEviction(key, current.val, EvictionReasonExpired)
	}
	return val, false
}

// Replace replaces the value for the given key.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to add or replace an entry, use the Set method instead.
func (c *Cache[K, V]) Replace(key K, val V, exp Expiration) bool {
	now := time.Now()
	new := entry[K, V]{val: val}
	exp.apply(&new.exp)
	new.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	current, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	new.node = current.node
	c.queue.Remove(current.node)
	if current.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			c.onEviction(key, current.val, EvictionReasonExpired)
		}
		return false
	}
	c.queue.Add(new.node)
	c.m[key] = new
	c.mu.Unlock()
	if c.onEviction != nil {
		c.onEviction(key, current.val, EvictionReasonReplaced)
	}
	return true
}

// ReplaceWithFunc replaces the value for the given key with the result
// of the given function that takes the old value as an argument.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to replace the value with a new value not depending on the old
// value, use the Replace method instead.
//
// imcache provides the Increment and Decrement functions that can be used as f
// to increment or decrement the old numeric type value.
//
// Example:
//
//	var c imcache.Cache[string, int32]
//	c.Set("foo", 997, imcache.WithNoExpiration())
//	_ = c.ReplaceWithFunc("foo", imcache.Increment[int32], imcache.WithNoExpiration())
func (c *Cache[K, V]) ReplaceWithFunc(key K, f func(V) V, exp Expiration) bool {
	now := time.Now()
	c.mu.Lock()
	current, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	c.queue.Remove(current.node)
	if current.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			c.onEviction(key, current.val, EvictionReasonExpired)
		}
		return false
	}
	new := entry[K, V]{val: f(current.val), node: current.node}
	exp.apply(&new.exp)
	new.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.queue.Add(new.node)
	c.m[key] = new
	c.mu.Unlock()
	if c.onEviction != nil {
		c.onEviction(key, current.val, EvictionReasonReplaced)
	}
	return true
}

// Remove removes the cache entry for the given key.
//
// It returns true if the entry is present and removed,
// otherwise it returns false.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved. If entry is expired, it returns false.
func (c *Cache[K, V]) Remove(key K) bool {
	now := time.Now()
	c.mu.Lock()
	current, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	delete(c.m, key)
	c.queue.Remove(current.node)
	c.mu.Unlock()
	if c.onEviction == nil {
		return !current.HasExpired(now)
	}
	if current.HasExpired(now) {
		c.onEviction(key, current.val, EvictionReasonExpired)
		return false
	}
	c.onEviction(key, current.val, EvictionReasonRemoved)
	return true
}

func (c *Cache[K, V]) removeAll(now time.Time) {
	c.mu.Lock()
	removed := c.m
	c.m = make(map[K]entry[K, V])
	if c.size > 0 {
		c.queue = &lruq[K]{}
	} else {
		c.queue = &nopq[K]{}
	}
	c.mu.Unlock()
	if c.onEviction != nil {
		for key, entry := range removed {
			if entry.HasExpired(now) {
				c.onEviction(key, entry.val, EvictionReasonExpired)
			} else {
				c.onEviction(key, entry.val, EvictionReasonRemoved)
			}
		}
	}
}

// RemoveAll removes all entries.
//
// If an eviction callback is set, it is called for each removed entry.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved.
func (c *Cache[K, V]) RemoveAll() {
	c.removeAll(time.Now())
}

func (c *Cache[K, V]) removeExpired(now time.Time) {
	c.mu.Lock()
	// To avoid copying the expired entries if there's no eviction callback.
	if c.onEviction == nil {
		for key, entry := range c.m {
			if entry.HasExpired(now) {
				delete(c.m, key)
			}
		}
		c.mu.Unlock()
		return
	}
	var removed []kv[K, V]
	for key, entry := range c.m {
		if entry.HasExpired(now) {
			removed = append(removed, kv[K, V]{key, entry.val})
			delete(c.m, key)
			c.queue.Remove(entry.node)
		}
	}
	c.mu.Unlock()
	for _, kv := range removed {
		c.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
}

// RemoveExpired removes all expired entries.
//
// If an eviction callback is set, it is called for each removed entry.
func (c *Cache[K, V]) RemoveExpired() {
	c.removeExpired(time.Now())
}

type kv[K comparable, V any] struct {
	key K
	val V
}

func (c *Cache[K, V]) getAll(now time.Time) map[K]V {
	c.mu.Lock()
	// To avoid copying the expired entries if there's no eviction callback.
	if c.onEviction == nil {
		m := make(map[K]V, len(c.m))
		for key, entry := range c.m {
			if entry.HasExpired(now) {
				delete(c.m, key)
			} else {
				if entry.HasSlidingExpiration() {
					entry.SlideExpiration(now)
					c.m[key] = entry
				}
				m[key] = entry.val
			}
		}
		c.mu.Unlock()
		return m
	}
	var expired []kv[K, V]
	m := make(map[K]V, len(c.m))
	for key, entry := range c.m {
		if entry.HasExpired(now) {
			expired = append(expired, kv[K, V]{key: key, val: entry.val})
			delete(c.m, key)
			c.queue.Remove(entry.node)
		} else {
			if entry.HasSlidingExpiration() {
				entry.SlideExpiration(now)
				c.m[key] = entry
			}
			m[key] = entry.val
		}
	}
	c.mu.Unlock()
	for _, kv := range expired {
		c.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
	return m
}

// GetAll returns a copy of all entries in the cache.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetAll() map[K]V {
	return c.getAll(time.Now())
}

// len returns the number of entries in the cache.
// It is not a concurrency-safe method.
func (c *Cache[K, V]) len() int {
	return len(c.m)
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	n := c.len()
	c.mu.Unlock()
	return n
}

// StartCleaner starts a cleaner that periodically removes expired entries.
// A cleaner runs in a separate goroutine.
// It returns an error if the cleaner is already running
// or if the interval is less than or equal to zero.
//
// The cleaner can be stopped by calling StopCleaner method.
func (c *Cache[K, V]) StartCleaner(interval time.Duration) error {
	return c.cleaner.start(c, interval)
}

// StopCleaner stops the cleaner.
// It is a blocking method that waits for the cleaner to stop.
// It's a NOP method if the cleaner is not running.
func (c *Cache[K, V]) StopCleaner() {
	c.cleaner.stop()
}

// NewSharded returns a new Sharded instance.
// It panics if n is not greater than 0 or hasher is nil.
//
// By default a returned Sharded has no default expiration,
// no default sliding expiration, no entry limit and
// no eviction callback.
//
// Option(s) can be used to customize the returned Sharded.
// Note that Option(s) are applied to each shard (Cache instance)
// not to the Sharded instance itself.
func NewSharded[K comparable, V any](n int, hasher Hasher64[K], opts ...Option[K, V]) *Sharded[K, V] {
	if n <= 0 {
		panic("imcache: number of shards must be greater than 0")
	}
	if hasher == nil {
		panic("imcache: hasher must be not nil")
	}
	shards := make([]*Cache[K, V], n)
	for i := 0; i < n; i++ {
		shards[i] = New(opts...)
	}
	return &Sharded[K, V]{
		shards:  shards,
		hasher:  hasher,
		mask:    uint64(n - 1),
		cleaner: newCleaner(),
	}
}

// Sharded is a sharded in-memory cache.
// It is a cache consisting of n shards
// and sharded by the given Hasher64.
//
// Each shard is a separate Cache instance.
//
// By default it has no default expiration, no default sliding expiration
// no entry limit and no eviction callback.
//
// The zero value Sharded is NOT ready to use.
// The NewSharded function must be used to create a new Sharded.
//
// Example:
//
//	c := imcache.NewSharded[string, interface{}](8, imcache.DefaultStringHasher64{},
//		imcache.WithDefaultExpirationOption[string, interface{}](time.Second),
//		imcache.WithMaxEntriesOption[string, interface{}](10000),
//		imcache.WithEvictionCallbackOption[string, interface{}](LogEvictedEntry),
//	)
type Sharded[K comparable, V any] struct {
	shards []*Cache[K, V]
	hasher Hasher64[K]
	mask   uint64

	cleaner *cleaner
}

// shard returns the shard for the given key.
func (s *Sharded[K, V]) shard(key K) *Cache[K, V] {
	return s.shards[s.hasher.Sum64(key)&s.mask]
}

// Get returns the value for the given key.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) Get(key K) (V, bool) {
	return s.shard(key).Get(key)
}

// Set sets the value for the given key.
// If the entry already exists, it is replaced.
//
// If it encounters an expired entry, it is evicted and a new entry is added.
//
// If you don't want to replace an existing entry, use the GetOrSet method
// instead. If you don't want to add a new entry if it doesn't exist, use
// the Replace method instead.
func (s *Sharded[K, V]) Set(key K, val V, exp Expiration) {
	s.shard(key).Set(key, val, exp)
}

// GetOrSet returns the value for the given key and true if it exists,
// otherwise it sets the value for the given key and returns the set value
// and false.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) GetOrSet(key K, val V, exp Expiration) (v V, present bool) {
	return s.shard(key).GetOrSet(key, val, exp)
}

// Replace replaces the value for the given key.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to add or replace an entry, use the Set method instead.
func (s *Sharded[K, V]) Replace(key K, val V, exp Expiration) bool {
	return s.shard(key).Replace(key, val, exp)
}

// ReplaceWithFunc replaces the value for the given key with the result
// of the given function that takes the old value as an argument.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to replace the value with a new value not depending on the old
// value, use the Replace method instead.
//
// imcache provides the Increment and Decrement functions that can be used as f
// to increment or decrement the old numeric type value.
//
// Example:
//
//	c := imcache.NewSharded[string, int32](4, imcache.DefaultStringHasher64{})
//	c.Set("foo", 997, imcache.WithNoExpiration())
//	_ = c.ReplaceWithFunc("foo", imcache.Increment[int32], imcache.WithNoExpiration())
func (s *Sharded[K, V]) ReplaceWithFunc(key K, fn func(V) V, exp Expiration) bool {
	return s.shard(key).ReplaceWithFunc(key, fn, exp)
}

// Remove removes the cache entry for the given key.
//
// It returns true if the entry is present and removed,
// otherwise it returns false.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved. If entry is expired, it returns false.
func (s *Sharded[K, V]) Remove(key K) bool {
	return s.shard(key).Remove(key)
}

// RemoveAll removes all entries.
//
// If an eviction callback is set, it is called for each removed entry.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved.
func (s *Sharded[K, V]) RemoveAll() {
	now := time.Now()
	for _, shard := range s.shards {
		shard.removeAll(now)
	}
}

// RemoveExpired removes all expired entries.
//
// If an eviction callback is set, it is called for each removed entry.
func (s *Sharded[K, V]) RemoveExpired() {
	now := time.Now()
	for _, shard := range s.shards {
		shard.removeExpired(now)
	}
}

// GetAll returns a copy of all entries in the cache.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) GetAll() map[K]V {
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

// Len returns the number of entries in the cache.
func (s *Sharded[K, V]) Len() int {
	var n int
	for _, shard := range s.shards {
		n += shard.Len()
	}
	return n
}

// StartCleaner starts a cleaner that periodically removes expired entries.
// A cleaner runs in a separate goroutine.
// It returns an error if the cleaner is already running
// or if the interval is less than or equal to zero.
//
// The cleaner can be stopped by calling StopCleaner method.
func (s *Sharded[K, V]) StartCleaner(interval time.Duration) error {
	return s.cleaner.start(s, interval)
}

// StopCleaner stops the cleaner.
// It is a blocking method that waits for the cleaner to stop.
// It's a NOP method if the cleaner is not running.
func (s *Sharded[K, V]) StopCleaner() {
	s.cleaner.stop()
}
