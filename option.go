package imcache

import "time"

// Option is a Cache option.
type Option[K comparable, V any] interface {
	apply(*Cache[K, V])
}

type optionf[K comparable, V any] func(*Cache[K, V])

//lint:ignore U1000 false positive
func (f optionf[K, V]) apply(c *Cache[K, V]) {
	f(c)
}

// WithEvictionCallbackOption returns an Option that sets the Cache
// eviction callback.
func WithEvictionCallbackOption[K comparable, V any](f EvictionCallback[K, V]) Option[K, V] {
	return optionf[K, V](func(c *Cache[K, V]) {
		c.onEviction = f
	})
}

// WithDefaultExpirationOption returns an Option that sets the Cache
// default expiration.
func WithDefaultExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(c *Cache[K, V]) {
		if d <= 0 {
			return
		}
		c.defaultExp = d
	})
}

// WithDefaultSlidingExpirationOption returns an Option that sets the Cache
// default sliding expiration.
func WithDefaultSlidingExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(c *Cache[K, V]) {
		if d <= 0 {
			return
		}
		c.defaultExp = d
		c.sliding = true
	})
}

// WithMaxEntriesOption returns an Option that sets the Cache maximum number
// of entries. When the maximum number of entries is exceeded, the least
// recently used entry is evicted regardless of the entry's expiration time.
//
// If used with Sharded type, the maximum size is per shard,
// not the total size of all shards.
func WithMaxEntriesOption[K comparable, V any](n int) Option[K, V] {
	return optionf[K, V](func(c *Cache[K, V]) {
		if n <= 0 {
			return
		}
		c.size = n
		c.queue = &fifoq[K]{}
	})
}
