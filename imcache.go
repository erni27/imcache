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
	o := options[K, V]{defaultExp: noExp}
	for _, opt := range opts {
		opt.apply(&o)
	}
	return newCache(o)
}

func newCache[K comparable, V any](opts options[K, V]) *Cache[K, V] {
	c := &Cache[K, V]{
		m:          make(map[K]entry[K, V]),
		onEviction: opts.onEviction,
		defaultExp: opts.defaultExp,
		sliding:    opts.sliding,
	}
	if opts.maxEntriesLimit > 0 {
		c.queue = &lruq[K]{}
		c.maxEntriesLimit = opts.maxEntriesLimit
	} else {
		c.queue = &nopq[K]{}
	}
	if opts.cleanerInterval > 0 {
		c.cleaner = newCleaner()
		if err := c.cleaner.start(c, opts.cleanerInterval); err != nil {
			// With current sanitization this should never happen.
			panic(err)
		}
	}
	return c
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
//		imcache.WithCleanerOption[string, interface{}](5*time.Minute),
//		imcache.WithMaxEntriesOption[string, interface{}](10000),
//		imcache.WithEvictionCallbackOption[string, interface{}](LogEvictedEntry),
//	)
type Cache[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]entry[K, V]

	defaultExp time.Duration
	sliding    bool

	onEviction EvictionCallback[K, V]

	queue           evictionq[K]
	maxEntriesLimit int

	cleaner *cleaner

	closed bool
}

// init initializes the Cache.
// It is not a concurrency-safe method.
func (s *Cache[K, V]) init() {
	if s.m == nil {
		s.m = make(map[K]entry[K, V])
		s.defaultExp = noExp
		s.queue = &nopq[K]{}
	}
}

// Get returns the value for the given key.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := time.Now()
	var zero V
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return zero, false
	}
	currentEntry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return zero, false
	}
	c.queue.Remove(currentEntry.node)
	if currentEntry.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return zero, false
	}
	if currentEntry.HasSlidingExpiration() {
		currentEntry.SlideExpiration(now)
		c.m[key] = currentEntry
	}
	c.queue.Add(currentEntry.node)
	c.mu.Unlock()
	return currentEntry.val, true
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
	newEntry := entry[K, V]{val: val}
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	// Make sure that the shard is initialized.
	c.init()
	newEntry.node = c.queue.AddNew(key)
	currentEntry, ok := c.m[key]
	c.m[key] = newEntry
	if ok {
		c.queue.Remove(currentEntry.node)
		c.mu.Unlock()
		if c.onEviction != nil {
			if currentEntry.HasExpired(now) {
				go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
			} else {
				go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
			}
		}
		return
	}
	if c.maxEntriesLimit <= 0 || c.len() <= c.maxEntriesLimit {
		c.mu.Unlock()
		return
	}
	lruNode := c.queue.Pop()
	if c.onEviction == nil {
		delete(c.m, lruNode.key)
		c.mu.Unlock()
		return
	}
	lruEntry := c.m[lruNode.key]
	delete(c.m, lruNode.key)
	c.mu.Unlock()
	if lruEntry.HasExpired(now) {
		go c.onEviction(lruNode.key, lruEntry.val, EvictionReasonExpired)
	} else {
		go c.onEviction(lruNode.key, lruEntry.val, EvictionReasonMaxEntriesExceeded)
	}
}

// GetOrSet returns the value for the given key and true if it exists,
// otherwise it sets the value for the given key and returns the set value
// and false.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetOrSet(key K, val V, exp Expiration) (V, bool) {
	now := time.Now()
	newEntry := entry[K, V]{val: val}
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	// Make sure that the shard is initialized.
	c.init()
	currentEntry, ok := c.m[key]
	if !ok {
		newEntry.node = c.queue.AddNew(key)
		c.m[key] = newEntry
		if c.maxEntriesLimit <= 0 || c.len() <= c.maxEntriesLimit {
			c.mu.Unlock()
			return val, false
		}
		lruNode := c.queue.Pop()
		if c.onEviction == nil {
			delete(c.m, lruNode.key)
			c.mu.Unlock()
			return val, false
		}
		lruEntry := c.m[lruNode.key]
		delete(c.m, lruNode.key)
		c.mu.Unlock()
		if lruEntry.HasExpired(now) {
			go c.onEviction(lruNode.key, lruEntry.val, EvictionReasonExpired)
		} else {
			go c.onEviction(lruNode.key, lruEntry.val, EvictionReasonMaxEntriesExceeded)
		}
		return val, false
	}
	c.queue.Remove(currentEntry.node)
	if !currentEntry.HasExpired(now) {
		if currentEntry.HasSlidingExpiration() {
			currentEntry.SlideExpiration(now)
			c.m[key] = currentEntry
		}
		c.queue.Add(currentEntry.node)
		c.mu.Unlock()
		return currentEntry.val, true
	}
	newEntry.node = c.queue.AddNew(key)
	c.m[key] = newEntry
	c.mu.Unlock()
	if c.onEviction != nil {
		go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
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
	newEntry := entry[K, V]{val: val}
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	currentEntry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	newEntry.node = currentEntry.node
	c.queue.Remove(currentEntry.node)
	if currentEntry.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return false
	}
	c.queue.Add(newEntry.node)
	c.m[key] = newEntry
	c.mu.Unlock()
	if c.onEviction != nil {
		go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
	}
	return true
}

// ReplaceWithFunc replaces the value for the given key with the result
// of the given function that takes the current value as an argument.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to replace the value with a new value not depending on
// the current value, use the Replace method instead.
//
// imcache provides the Increment and Decrement functions that can be used as f
// to increment or decrement the current numeric type value.
//
// Example:
//
//	var c imcache.Cache[string, int32]
//	c.Set("foo", 997, imcache.WithNoExpiration())
//	_ = c.ReplaceWithFunc("foo", imcache.Increment[int32], imcache.WithNoExpiration())
func (c *Cache[K, V]) ReplaceWithFunc(key K, f func(current V) (new V), exp Expiration) bool {
	now := time.Now()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	currentEntry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	c.queue.Remove(currentEntry.node)
	if currentEntry.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return false
	}
	newEntry := entry[K, V]{val: f(currentEntry.val), node: currentEntry.node}
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	c.queue.Add(newEntry.node)
	c.m[key] = newEntry
	c.mu.Unlock()
	if c.onEviction != nil {
		go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
	}
	return true
}

// ReplaceKey replaces the given old key with the new key.
// The value remains the same. It returns true if the key is present
// and replaced, otherwise it returns false. If there is an existing
// entry for the new key, it is replaced.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) ReplaceKey(old, new K, exp Expiration) bool {
	now := time.Now()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	oldEntry, ok := c.m[old]
	if !ok {
		c.mu.Unlock()
		return false
	}
	delete(c.m, old)
	c.queue.Remove(oldEntry.node)
	if oldEntry.HasExpired(now) {
		c.mu.Unlock()
		if c.onEviction != nil {
			go c.onEviction(old, oldEntry.val, EvictionReasonExpired)
		}
		return false
	}
	currentEntry, ok := c.m[new]
	newEntry := entry[K, V]{val: oldEntry.val}
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, c.defaultExp, c.sliding)
	newEntry.node = c.queue.AddNew(new)
	c.m[new] = newEntry
	if !ok {
		c.mu.Unlock()
		if c.onEviction != nil {
			go c.onEviction(old, oldEntry.val, EvictionReasonKeyReplaced)
		}
		return true
	}
	c.queue.Remove(currentEntry.node)
	c.mu.Unlock()
	if c.onEviction != nil {
		go func() {
			c.onEviction(old, oldEntry.val, EvictionReasonKeyReplaced)
			if currentEntry.HasExpired(now) {
				c.onEviction(new, currentEntry.val, EvictionReasonExpired)
			} else {
				c.onEviction(new, currentEntry.val, EvictionReasonReplaced)
			}
		}()
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
	if c.closed {
		c.mu.Unlock()
		return false
	}
	currentEntry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	delete(c.m, key)
	c.queue.Remove(currentEntry.node)
	c.mu.Unlock()
	if c.onEviction == nil {
		return !currentEntry.HasExpired(now)
	}
	if currentEntry.HasExpired(now) {
		go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		return false
	}
	go c.onEviction(key, currentEntry.val, EvictionReasonRemoved)
	return true
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

func (c *Cache[K, V]) removeAll(now time.Time) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	removedEntries := c.m
	c.m = make(map[K]entry[K, V])
	if c.maxEntriesLimit > 0 {
		c.queue = &lruq[K]{}
	} else {
		c.queue = &nopq[K]{}
	}
	c.mu.Unlock()
	if c.onEviction != nil {
		go func() {
			for key, entry := range removedEntries {
				if entry.HasExpired(now) {
					c.onEviction(key, entry.val, EvictionReasonExpired)
				} else {
					c.onEviction(key, entry.val, EvictionReasonRemoved)
				}
			}
		}()
	}
}

// RemoveExpired removes all expired entries.
//
// If an eviction callback is set, it is called for each removed entry.
func (c *Cache[K, V]) RemoveExpired() {
	c.removeExpired(time.Now())
}

func (c *Cache[K, V]) removeExpired(now time.Time) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
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
	var removedEntries []kv[K, V]
	for key, entry := range c.m {
		if entry.HasExpired(now) {
			removedEntries = append(removedEntries, kv[K, V]{key: key, val: entry.val})
			delete(c.m, key)
			c.queue.Remove(entry.node)
		}
	}
	c.mu.Unlock()
	go func() {
		for _, kv := range removedEntries {
			c.onEviction(kv.key, kv.val, EvictionReasonExpired)
		}
	}()
}

// GetAll returns a copy of all entries in the cache.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetAll() map[K]V {
	return c.getAll(time.Now())
}

func (c *Cache[K, V]) getAll(now time.Time) map[K]V {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
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
	var expiredEntries []kv[K, V]
	m := make(map[K]V, len(c.m))
	for key, entry := range c.m {
		if entry.HasExpired(now) {
			expiredEntries = append(expiredEntries, kv[K, V]{key: key, val: entry.val})
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
	go func() {
		for _, kv := range expiredEntries {
			c.onEviction(kv.key, kv.val, EvictionReasonExpired)
		}
	}()
	return m
}

type kv[K comparable, V any] struct {
	key K
	val V
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0
	}
	n := c.len()
	c.mu.Unlock()
	return n
}

// len returns the number of entries in the cache.
// It is not a concurrency-safe method.
func (c *Cache[K, V]) len() int {
	return len(c.m)
}

// Close closes the cache. It purges all entries and stops the cleaner
// if it is running. After Close, all other methods are NOP returning
// zero values immediately.
//
// It is safe to call Close multiple times.
//
// It's not necessary to call Close if the cache is no longer referenced
// and there is no cleaner running. Garbage collector will collect the cache.
func (c *Cache[K, V]) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.m = nil
	c.closed = true
	c.mu.Unlock()
	// If the cleaner is running, stop it.
	// It's safe to access c.cleaner without a lock because
	// it's only set during initialization and never modified.
	if c.cleaner != nil {
		c.cleaner.stop()
	}
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
	o := options[K, V]{defaultExp: noExp}
	for _, opt := range opts {
		opt.apply(&o)
	}
	cleanerInterval := o.cleanerInterval
	// To prevent running the cleaner in each shard.
	o.cleanerInterval = 0
	shards := make([]*Cache[K, V], n)
	for i := 0; i < n; i++ {
		shards[i] = newCache(o)
	}
	s := &Sharded[K, V]{
		shards: shards,
		hasher: hasher,
		mask:   uint64(n - 1),
	}
	if cleanerInterval > 0 {
		s.cleaner = newCleaner()
		if err := s.cleaner.start(s, cleanerInterval); err != nil {
			// With current sanitization this should never happen.
			panic(err)
		}
	}
	return s
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
//		imcache.WithCleanerOption[string, interface{}](5*time.Minute),
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

// ReplaceKey replaces the given old key with the new key.
// The value remains the same. It returns true if the key is present
// and replaced, otherwise it returns false. If there is an existing
// entry for the new key, it is replaced.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) ReplaceKey(old, new K, exp Expiration) bool {
	now := time.Now()
	oldShard := s.shard(old)
	newShard := s.shard(new)
	// Check if the old and new keys are in the same shard.
	if oldShard == newShard {
		return oldShard.ReplaceKey(old, new, exp)
	}
	oldShard.mu.Lock()
	// Check if the old shard is closed.
	// If so it means that the Sharded is closed as well.
	if oldShard.closed {
		oldShard.mu.Unlock()
		return false
	}
	oldShardEntry, ok := oldShard.m[old]
	if !ok {
		oldShard.mu.Unlock()
		return false
	}
	oldShard.queue.Remove(oldShardEntry.node)
	delete(oldShard.m, old)
	if oldShardEntry.HasExpired(now) {
		oldShard.mu.Unlock()
		if oldShard.onEviction != nil {
			go oldShard.onEviction(old, oldShardEntry.val, EvictionReasonExpired)
		}
		return false
	}
	newShard.mu.Lock()
	newShardEntry, ok := newShard.m[new]
	if ok {
		newShard.queue.Remove(newShardEntry.node)
	}
	newEntry := entry[K, V]{val: oldShardEntry.val}
	newEntry.node = newShard.queue.AddNew(new)
	exp.apply(&newEntry.exp)
	newEntry.SetDefaultOrNothing(now, newShard.defaultExp, newShard.sliding)
	newShard.m[new] = newEntry
	if newShard.maxEntriesLimit <= 0 || newShard.len() <= newShard.maxEntriesLimit {
		oldShard.mu.Unlock()
		newShard.mu.Unlock()
		// Both callbacks points to the same function.
		if newShard.onEviction != nil {
			go func() {
				newShard.onEviction(old, oldShardEntry.val, EvictionReasonKeyReplaced)
				if ok {
					if newShardEntry.HasExpired(now) {
						newShard.onEviction(new, newShardEntry.val, EvictionReasonExpired)
					} else {
						newShard.onEviction(new, newShardEntry.val, EvictionReasonReplaced)
					}
				}
			}()
		}
		return true
	}
	lruNode := newShard.queue.Pop()
	if newShard.onEviction == nil {
		delete(newShard.m, lruNode.key)
		oldShard.mu.Unlock()
		newShard.mu.Unlock()
		return true
	}
	lruEntry := newShard.m[lruNode.key]
	delete(newShard.m, lruNode.key)
	oldShard.mu.Unlock()
	newShard.mu.Unlock()
	go func() {
		newShard.onEviction(old, oldShardEntry.val, EvictionReasonKeyReplaced)
		if lruEntry.HasExpired(now) {
			newShard.onEviction(lruNode.key, lruEntry.val, EvictionReasonExpired)
		} else {
			newShard.onEviction(lruNode.key, lruEntry.val, EvictionReasonMaxEntriesExceeded)
		}
	}()
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
		// If Cache.getAll returns nil, it means that the shard is closed
		// hence Sharded is closed too.
		if m == nil {
			return nil
		}
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

// Close closes the cache. It purges all entries and stops the cleaner
// if it is running. After Close, all other methods are NOP returning
// zero values immediately.
//
// It is safe to call Close multiple times.
//
// It's not necessary to call Close if the cache is no longer referenced
// and there is no cleaner running. Garbage collector will collect the cache.
func (s *Sharded[K, V]) Close() {
	for _, shard := range s.shards {
		shard.Close()
	}
	// If the cleaner is running, stop it.
	// It's safe to access s.cleaner without a lock because
	// it's only set during initialization and never modified.
	if s.cleaner != nil {
		s.cleaner.stop()
	}
}
