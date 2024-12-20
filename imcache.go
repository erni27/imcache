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

// nowf is a func returning current time. It simply calls time.Now()
// under the hood. It should be changed only for the testing purposes.
//
// This global variable is not ideal but it simplifies a lot, making
// zero value Cache simple. No need to init a field level clock, that
// is important for read-only (Peek*) methods.
var nowf = func() time.Time {
	return time.Now()
}

// New returns a new Cache instance.
//
// By default, a returned Cache has no default expiration, no default sliding
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
	var limit int
	if opts.maxEntriesLimit > 0 {
		limit = opts.maxEntriesLimit
	}
	c := &Cache[K, V]{
		m:          make(map[K]node[K, V], limit),
		onEviction: opts.onEviction,
		defaultExp: opts.defaultExp,
		sliding:    opts.sliding,
		limit:      limit,
		policy:     opts.evictionPolicy,
		queue:      newEvictionQueue[K, V](opts.maxEntriesLimit, opts.evictionPolicy),
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
// By default, it has no default expiration, no default sliding expiration
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
	queue      evictionQueue[K, V]
	m          map[K]node[K, V]
	onEviction EvictionCallback[K, V]
	cleaner    *cleaner
	defaultExp time.Duration
	policy     EvictionPolicy
	limit      int
	mu         sync.RWMutex
	sliding    bool
	closed     bool
}

// init initializes the Cache.
// It is not a concurrency-safe method.
func (c *Cache[K, V]) init() {
	if c.m == nil {
		c.m = make(map[K]node[K, V])
		c.defaultExp = noExp
		c.queue = &nopEvictionQueue[K, V]{}
	}
}

// Get returns the value for the given key.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	now := nowf()
	var zero V
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return zero, false
	}
	node, ok := c.m[key]
	if !ok {
		return zero, false
	}
	entry := node.entry()
	if entry.expired(now) {
		c.queue.remove(node)
		delete(c.m, key)
		if c.onEviction != nil {
			go c.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return zero, false
	}
	entry.slide(now)
	node.setEntry(entry)
	c.queue.touch(node)
	return entry.val, true
}

// GetMultiple returns the values for the given keys.
// If the Cache is not closed, then the returned map
// is always a non-nil one.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetMultiple(keys ...K) map[K]V {
	return c.getMultiple(nowf(), keys...)
}

func (c *Cache[K, V]) getMultiple(now time.Time, keys ...K) map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	got := make(map[K]V, len(keys))
	// To avoid copying the expired entries if there's no eviction callback.
	if c.onEviction == nil {
		for _, key := range keys {
			node, ok := c.m[key]
			if !ok {
				continue
			}
			entry := node.entry()
			if entry.expired(now) {
				c.queue.remove(node)
				delete(c.m, key)
				continue
			}
			entry.slide(now)
			node.setEntry(entry)
			c.queue.touch(node)
			got[key] = entry.val
		}
		return got
	}
	var expired []entry[K, V]
	for _, key := range keys {
		node, ok := c.m[key]
		if !ok {
			continue
		}
		entry := node.entry()
		if entry.expired(now) {
			expired = append(expired, entry)
			c.queue.remove(node)
			delete(c.m, key)
			continue
		}
		entry.slide(now)
		node.setEntry(entry)
		c.queue.touch(node)
		got[key] = entry.val
	}
	go func() {
		for _, entry := range expired {
			c.onEviction(entry.key, entry.val, EvictionReasonExpired)
		}
	}()
	return got
}

// GetAll returns a copy of all entries in the cache.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetAll() map[K]V {
	return c.getAll(nowf())
}

func (c *Cache[K, V]) getAll(now time.Time) map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	// To avoid copying the expired entries if there's no eviction callback.
	if c.onEviction == nil {
		got := make(map[K]V, len(c.m))
		for key, node := range c.m {
			entry := node.entry()
			if entry.expired(now) {
				c.queue.remove(node)
				delete(c.m, key)
				continue
			}
			entry.slide(now)
			node.setEntry(entry)
			got[key] = entry.val
		}
		c.queue.touchall()
		return got
	}
	var expired []entry[K, V]
	got := make(map[K]V, len(c.m))
	for key, node := range c.m {
		entry := node.entry()
		if entry.expired(now) {
			expired = append(expired, entry)
			delete(c.m, key)
			c.queue.remove(node)
			continue
		}
		entry.slide(now)
		node.setEntry(entry)
		got[key] = entry.val
	}
	c.queue.touchall()
	if len(expired) != 0 {
		go func() {
			for _, kv := range expired {
				c.onEviction(kv.key, kv.val, EvictionReasonExpired)
			}
		}()
	}
	return got
}

// Peek returns the value for the given key without
// actively evicting the entry if it is expired and
// updating the entry's sliding expiration.
//
// If the max entries limit is set, it doesn't update
// the entry's position in the eviction queue.
func (c *Cache[K, V]) Peek(key K) (V, bool) {
	now := nowf()
	var zero V
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return zero, false
	}
	n, ok := c.m[key]
	if !ok || n.entry().expired(now) {
		return zero, false
	}
	return n.entry().val, true
}

// PeekMultiple returns the values for the given keys without
// actively evicting the encountered entry if it is expired and
// updating the entry's sliding expiration.
// If the Cache is not closed, then the returned map
// is always a non-nil one.
//
// If the max entries limit is set, it doesn't update
// the encountered entry's position in the eviction queue.
func (c *Cache[K, V]) PeekMultiple(keys ...K) map[K]V {
	return c.peekMultiple(nowf(), keys...)
}

func (c *Cache[K, V]) peekMultiple(now time.Time, keys ...K) map[K]V {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil
	}
	got := make(map[K]V, len(keys))
	for _, key := range keys {
		node, ok := c.m[key]
		if !ok || node.entry().expired(now) {
			continue
		}
		got[key] = node.entry().val
	}
	return got
}

// PeekAll returns a copy of all entries in the cache without
// actively evicting the encountered entry if it is expired and
// updating the entry's sliding expiration.
//
// If the max entries limit is set, it doesn't update
// the encountered entry's position in the eviction queue.
func (c *Cache[K, V]) PeekAll() map[K]V {
	return c.peekAll(nowf())
}

func (c *Cache[K, V]) peekAll(now time.Time) map[K]V {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil
	}
	got := make(map[K]V, len(c.m))
	for key, node := range c.m {
		if node.entry().expired(now) {
			continue
		}
		got[key] = node.entry().val
	}
	return got
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
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	// Make sure that the shard is initialized.
	c.init()
	currentNode, ok := c.m[key]
	newEntry := entry[K, V]{key: key, val: val, exp: exp.new(now, c.defaultExp, c.sliding)}
	if ok {
		currentEntry := currentNode.entry()
		if !currentEntry.expired(now) {
			currentNode.setEntry(newEntry)
			c.queue.touch(currentNode)
			if c.onEviction != nil {
				go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
			}
			return
		}
		c.m[key] = c.queue.add(newEntry)
		c.queue.remove(currentNode)
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return
	}
	var evictedNode node[K, V]
	if c.limit > 0 && c.len() == c.limit {
		evictedNode = c.queue.pop()
		delete(c.m, evictedNode.entry().key)
	}
	c.m[key] = c.queue.add(entry[K, V]{key: key, val: val, exp: exp.new(now, c.defaultExp, c.sliding)})
	if c.onEviction == nil || evictedNode == nil {
		return
	}
	evictedEntry := evictedNode.entry()
	if evictedEntry.expired(now) {
		go c.onEviction(evictedEntry.key, evictedEntry.val, EvictionReasonExpired)
	} else {
		go c.onEviction(evictedEntry.key, evictedEntry.val, EvictionReasonMaxEntriesExceeded)
	}
}

// GetOrSet returns the value for the given key and true if it exists,
// otherwise it sets the value for the given key and returns the set value
// and false.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) GetOrSet(key K, val V, exp Expiration) (value V, present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		var zero V
		return zero, false
	}
	// Make sure that the shard is initialized.
	c.init()
	currentNode, ok := c.m[key]
	if ok && !currentNode.entry().expired(now) {
		currentEntry := currentNode.entry()
		currentEntry.slide(now)
		currentNode.setEntry(currentEntry)
		c.queue.touch(currentNode)
		return currentEntry.val, true
	}
	if ok {
		c.m[key] = c.queue.add(entry[K, V]{key: key, val: val, exp: exp.new(now, c.defaultExp, c.sliding)})
		c.queue.remove(currentNode)
		if c.onEviction != nil {
			go c.onEviction(key, currentNode.entry().val, EvictionReasonExpired)
		}
		return val, false
	}
	var evictedNode node[K, V]
	if c.limit > 0 && c.len() == c.limit {
		evictedNode = c.queue.pop()
		delete(c.m, evictedNode.entry().key)
	}
	c.m[key] = c.queue.add(entry[K, V]{key: key, val: val, exp: exp.new(now, c.defaultExp, c.sliding)})
	if c.onEviction == nil || evictedNode == nil {
		return val, false
	}
	var evictionReason EvictionReason
	evictedEntry := evictedNode.entry()
	if evictedEntry.expired(now) {
		evictionReason = EvictionReasonExpired
	} else {
		evictionReason = EvictionReasonMaxEntriesExceeded
	}
	go c.onEviction(evictedEntry.key, evictedEntry.val, evictionReason)
	return val, false
}

// Replace replaces the value for the given key.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to add or replace an entry, use the Set method instead.
func (c *Cache[K, V]) Replace(key K, val V, exp Expiration) (present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	currentNode, ok := c.m[key]
	if !ok {
		return false
	}
	currentEntry := currentNode.entry()
	if currentEntry.expired(now) {
		c.queue.remove(currentNode)
		delete(c.m, key)
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return false
	}
	currentNode.setEntry(entry[K, V]{key: key, val: val, exp: exp.new(now, c.defaultExp, c.sliding)})
	c.queue.touch(currentNode)
	if c.onEviction != nil {
		go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
	}
	return true
}

// Number is a constraint that permits any numeric type except complex ones.
//
// Deprecated: Number constraint is deprecated. It is easy to write your own
// constraint. imcache's goal is to be simple. Creating artificial types
// or functions that are not even needed conflicts with this goal.
type Number interface {
	~float32 | ~float64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Increment increments the given number by one.
//
// Deprecated: Increment function is deprecated. It is easy to write your own
// function. imcache's goal is to be simple. Creating artificial types
// or functions that are not even needed conflicts with this goal.
func Increment[V Number](old V) V {
	return old + 1
}

// Decrement decrements the given number by one.
//
// Deprecated: Decrement function is deprecated. It is easy to write your own
// function. imcache's goal is to be simple. Creating artificial types
// or functions that are not even needed conflicts with this goal.
func Decrement[V Number](old V) V {
	return old - 1
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
// Example showing how to increment the value by 1 using ReplaceWithFunc:
//
//	var c imcache.Cache[string, int32]
//	c.Set("foo", 997, imcache.WithNoExpiration())
//	_ = c.ReplaceWithFunc(
//		"foo",
//		func(current int32) int32 { return current + 1 },
//		imcache.WithNoExpiration(),
//	)
func (c *Cache[K, V]) ReplaceWithFunc(key K, f func(current V) (new V), exp Expiration) (present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	currentNode, ok := c.m[key]
	if !ok {
		return false
	}
	currentEntry := currentNode.entry()
	if currentEntry.expired(now) {
		c.queue.remove(currentNode)
		delete(c.m, key)
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return false
	}
	currentNode.setEntry(entry[K, V]{key: key, val: f(currentEntry.val), exp: exp.new(now, c.defaultExp, c.sliding)})
	c.queue.touch(currentNode)
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
func (c *Cache[K, V]) ReplaceKey(old, new K, exp Expiration) (present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	oldNode, ok := c.m[old]
	if !ok {
		return false
	}
	oldEntry := oldNode.entry()
	delete(c.m, old)
	c.queue.remove(oldNode)
	if oldEntry.expired(now) {
		if c.onEviction != nil {
			go c.onEviction(old, oldEntry.val, EvictionReasonExpired)
		}
		return false
	}
	newEntry := entry[K, V]{key: new, val: oldEntry.val, exp: exp.new(now, c.defaultExp, c.sliding)}
	currentNode, ok := c.m[new]
	if !ok {
		c.m[new] = c.queue.add(newEntry)
		if c.onEviction != nil {
			go c.onEviction(old, oldEntry.val, EvictionReasonKeyReplaced)
		}
		return true
	}
	currentEntry := currentNode.entry()
	currentNode.setEntry(newEntry)
	c.queue.touch(currentNode)
	if c.onEviction != nil {
		go func() {
			c.onEviction(old, oldEntry.val, EvictionReasonKeyReplaced)
			if currentEntry.expired(now) {
				c.onEviction(new, currentEntry.val, EvictionReasonExpired)
			} else {
				c.onEviction(new, currentEntry.val, EvictionReasonReplaced)
			}
		}()
	}
	return true
}

// CompareAndSwap replaces the value for the given key if the current value
// is equal to the expected value.
//
// Equality is defined by the given compare function.
//
// If it encounters an expired entry, the expired entry is evicted.
func (c *Cache[K, V]) CompareAndSwap(key K, expected, new V, compare func(V, V) bool, exp Expiration) (swapped, present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false, false
	}
	currentNode, ok := c.m[key]
	if !ok {
		return false, false
	}
	currentEntry := currentNode.entry()
	if currentEntry.expired(now) {
		c.queue.remove(currentNode)
		delete(c.m, key)
		if c.onEviction != nil {
			go c.onEviction(key, currentEntry.val, EvictionReasonExpired)
		}
		return false, false
	}
	if !compare(currentEntry.val, expected) {
		c.queue.touch(currentNode)
		return false, true
	}
	currentNode.setEntry(entry[K, V]{key: key, val: new, exp: exp.new(now, c.defaultExp, c.sliding)})
	c.queue.touch(currentNode)
	if c.onEviction != nil {
		go c.onEviction(key, currentEntry.val, EvictionReasonReplaced)
	}
	return true, true
}

// Remove removes the cache entry for the given key.
//
// It returns true if the entry is present and removed,
// otherwise it returns false.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved. If entry is expired, it returns false.
func (c *Cache[K, V]) Remove(key K) (present bool) {
	now := nowf()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	currentNode, ok := c.m[key]
	if !ok {
		return false
	}
	c.queue.remove(currentNode)
	delete(c.m, key)
	currentEntry := currentNode.entry()
	if c.onEviction == nil {
		return !currentEntry.expired(now)
	}
	if currentEntry.expired(now) {
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
	c.removeAll(nowf())
}

func (c *Cache[K, V]) removeAll(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	removed := c.m
	c.m = make(map[K]node[K, V], c.limit)
	c.queue = newEvictionQueue[K, V](c.limit, c.policy)
	if c.onEviction != nil && len(removed) != 0 {
		go func() {
			for key, node := range removed {
				entry := node.entry()
				if entry.expired(now) {
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
	c.removeExpired(nowf())
}

func (c *Cache[K, V]) removeExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	// To avoid copying the expired entries if there's no eviction callback.
	if c.onEviction == nil {
		for key, node := range c.m {
			entry := node.entry()
			if entry.expired(now) {
				c.queue.remove(node)
				delete(c.m, key)
			}
		}
		return
	}
	var removed []entry[K, V]
	for key, node := range c.m {
		entry := node.entry()
		if entry.expired(now) {
			removed = append(removed, entry)
			delete(c.m, key)
			c.queue.remove(node)
		}
	}
	if len(removed) != 0 {
		go func() {
			for _, entry := range removed {
				c.onEviction(entry.key, entry.val, EvictionReasonExpired)
			}
		}()
	}
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0
	}
	n := c.len()
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
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.m = nil
	c.closed = true
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
// By default, a returned Sharded has no default expiration,
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
// By default, it has no default expiration, no default sliding expiration
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
	hasher  Hasher64[K]
	cleaner *cleaner
	shards  []*Cache[K, V]
	mask    uint64
}

// shard returns the shard for the given key.
func (s *Sharded[K, V]) shard(key K) *Cache[K, V] {
	return s.shards[s.shardIndex(key)]
}

// shardIndex returns the shard index for the given key.
func (s *Sharded[K, V]) shardIndex(key K) int {
	return int(s.hasher.Sum64(key) & s.mask)
}

// Get returns the value for the given key.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) Get(key K) (value V, present bool) {
	return s.shard(key).Get(key)
}

// GetMultiple returns the values for the given keys.
// If the Sharded is not closed, then the returned map
// is always a non-nil one.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) GetMultiple(keys ...K) map[K]V {
	now := nowf()
	keysByShard := make(map[int][]K)
	for _, key := range keys {
		idx := s.shardIndex(key)
		keysByShard[idx] = append(keysByShard[idx], key)
	}
	ms := make([]map[K]V, len(keysByShard))
	var n int
	for idx, keys := range keysByShard {
		m := s.shards[idx].getMultiple(now, keys...)
		if m == nil {
			// getMultiple returns nil if the shard is closed.
			// If the shard is closed, then the Sharded is closed too.
			return nil
		}
		ms = append(ms, m)
		n += len(m)
	}
	result := make(map[K]V, n)
	for _, m := range ms {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// GetAll returns a copy of all entries in the cache.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) GetAll() map[K]V {
	now := nowf()
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

// Peek returns the value for the given key without
// actively evicting the entry if it is expired and
// updating the entry's sliding expiration.
//
// If the max entries limit is set, it doesn't update
// the entry's position in the eviction queue.
func (s *Sharded[K, V]) Peek(key K) (V, bool) {
	return s.shard(key).Peek(key)
}

// PeekMultiple returns the values for the given keys without
// actively evicting the encountered entry if it is expired and
// updating the entry's sliding expiration.
// If the Sharded is not closed, then the returned map
// is always a non-nil one.
//
// If the max entries limit is set, it doesn't update
// the encountered entry's position in the eviction queue.
func (s *Sharded[K, V]) PeekMultiple(keys ...K) map[K]V {
	now := nowf()
	keysByShard := make(map[int][]K)
	for _, key := range keys {
		idx := s.shardIndex(key)
		keysByShard[idx] = append(keysByShard[idx], key)
	}
	ms := make([]map[K]V, len(keysByShard))
	var n int
	for idx, keys := range keysByShard {
		m := s.shards[idx].peekMultiple(now, keys...)
		if m == nil {
			// peekMultiple returns nil if the shard is closed.
			// If the shard is closed, then the Sharded is closed too.
			return nil
		}
		ms = append(ms, m)
		n += len(m)
	}
	result := make(map[K]V, n)
	for _, m := range ms {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// PeekAll returns a copy of all entries in the cache without
// actively evicting the encountered entry if it is expired and
// updating the entry's sliding expiration.
//
// If the max entries limit is set, it doesn't update
// the encountered entry's position in the eviction queue.
func (s *Sharded[K, V]) PeekAll() map[K]V {
	now := nowf()
	var n int
	ms := make([]map[K]V, 0, len(s.shards))
	for _, shard := range s.shards {
		m := shard.peekAll(now)
		// If Cache.peekAll returns nil, it means that the shard is closed
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
func (s *Sharded[K, V]) GetOrSet(key K, val V, exp Expiration) (value V, present bool) {
	return s.shard(key).GetOrSet(key, val, exp)
}

// Replace replaces the value for the given key.
// It returns true if the value is present and replaced, otherwise it returns
// false.
//
// If it encounters an expired entry, the expired entry is evicted.
//
// If you want to add or replace an entry, use the Set method instead.
func (s *Sharded[K, V]) Replace(key K, val V, exp Expiration) (present bool) {
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
func (s *Sharded[K, V]) ReplaceWithFunc(key K, fn func(V) V, exp Expiration) (present bool) {
	return s.shard(key).ReplaceWithFunc(key, fn, exp)
}

// ReplaceKey replaces the given old key with the new key.
// The value remains the same. It returns true if the key is present
// and replaced, otherwise it returns false. If there is an existing
// entry for the new key, it is replaced.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) ReplaceKey(old, new K, exp Expiration) (present bool) {
	now := nowf()
	oldShard := s.shard(old)
	newShard := s.shard(new)
	// Check if the old and new keys are in the same shard.
	if oldShard == newShard {
		return oldShard.ReplaceKey(old, new, exp)
	}
	oldShard.mu.Lock()
	defer oldShard.mu.Unlock()
	// Check if the old shard is closed.
	// If so it means that the Sharded is closed as well.
	if oldShard.closed {
		return false
	}
	oldShardNode, ok := oldShard.m[old]
	if !ok {
		return false
	}
	oldShard.queue.remove(oldShardNode)
	delete(oldShard.m, old)
	oldShardEntry := oldShardNode.entry()
	if oldShardEntry.expired(now) {
		if oldShard.onEviction != nil {
			go oldShard.onEviction(old, oldShardEntry.val, EvictionReasonExpired)
		}
		return false
	}
	newEntry := entry[K, V]{key: new, val: oldShardEntry.val, exp: exp.new(now, oldShard.defaultExp, oldShard.sliding)}
	newShard.mu.Lock()
	defer newShard.mu.Unlock()
	newShardNode, ok := newShard.m[new]
	if ok {
		newShardEntry := newShardNode.entry()
		var evictionReason EvictionReason
		if !newShardEntry.expired(now) {
			newShardNode.setEntry(newEntry)
			newShard.queue.touch(newShardNode)
			evictionReason = EvictionReasonReplaced
		} else {
			newShard.m[new] = newShard.queue.add(newEntry)
			newShard.queue.remove(newShardNode)
			evictionReason = EvictionReasonExpired
		}
		// Both eviction callbacks point to the same function.
		if oldShard.onEviction != nil {
			go func() {
				oldShard.onEviction(old, oldShardEntry.val, EvictionReasonKeyReplaced)
				oldShard.onEviction(new, newShardEntry.val, evictionReason)
			}()
		}
		return true
	}
	if newShard.limit == 0 || newShard.len() < newShard.limit {
		newShard.m[new] = newShard.queue.add(newEntry)
		if oldShard.onEviction != nil {
			go oldShard.onEviction(old, oldShardEntry.val, EvictionReasonKeyReplaced)
		}
		return true
	}
	evictedNode := newShard.queue.pop()
	evictedEntry := evictedNode.entry()
	delete(newShard.m, evictedEntry.key)
	newShard.m[new] = newShard.queue.add(newEntry)
	if oldShard.onEviction == nil {
		return true
	}
	var evictionReason EvictionReason
	if evictedEntry.expired(now) {
		evictionReason = EvictionReasonExpired
	} else {
		evictionReason = EvictionReasonMaxEntriesExceeded
	}
	go func() {
		oldShard.onEviction(old, oldShardEntry.val, EvictionReasonKeyReplaced)
		oldShard.onEviction(evictedEntry.key, evictedEntry.val, evictionReason)
	}()
	return true
}

// CompareAndSwap replaces the value for the given key if the current value
// is equal to the expected value.
//
// Equality is defined by the given compare function.
//
// If it encounters an expired entry, the expired entry is evicted.
func (s *Sharded[K, V]) CompareAndSwap(key K, expected, new V, compare func(V, V) bool, exp Expiration) (swapped, present bool) {
	return s.shard(key).CompareAndSwap(key, expected, new, compare, exp)
}

// Remove removes the cache entry for the given key.
//
// It returns true if the entry is present and removed,
// otherwise it returns false.
//
// If it encounters an expired entry, the expired entry is evicted.
// It results in calling the eviction callback with EvictionReasonExpired,
// not EvictionReasonRemoved. If entry is expired, it returns false.
func (s *Sharded[K, V]) Remove(key K) (present bool) {
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
	now := nowf()
	for _, shard := range s.shards {
		shard.removeAll(now)
	}
}

// RemoveExpired removes all expired entries.
//
// If an eviction callback is set, it is called for each removed entry.
func (s *Sharded[K, V]) RemoveExpired() {
	now := nowf()
	for _, shard := range s.shards {
		shard.removeExpired(now)
	}
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

// entry is the value stored in the cache.
type entry[K comparable, V any] struct {
	key K
	val V
	exp expiration
}

// expired returns true if the entry has expired.
func (e entry[K, V]) expired(now time.Time) bool {
	return e.exp.date > 0 && e.exp.date < now.UnixNano()
}

// slide sets the expiration time to now + sliding.
func (e *entry[K, V]) slide(now time.Time) {
	if e.exp.sliding == 0 {
		return
	}
	e.exp.date = now.Add(e.exp.sliding).UnixNano()
}
