package imcache

import "time"

// options holds the cache configuration.
type options[K comparable, V any] struct {
	onEviction      EvictionCallback[K, V]
	defaultExp      time.Duration
	sliding         bool
	maxEntriesLimit int
	cleanerInterval time.Duration
}

// Option configures the cache.
type Option[K comparable, V any] interface {
	apply(*options[K, V])
}

type optionf[K comparable, V any] func(*options[K, V])

//lint:ignore U1000 false positive
func (f optionf[K, V]) apply(o *options[K, V]) {
	f(o)
}

// WithEvictionCallbackOption returns an Option that sets the cache
// eviction callback.
func WithEvictionCallbackOption[K comparable, V any](f EvictionCallback[K, V]) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		o.onEviction = f
	})
}

// WithDefaultExpirationOption returns an Option that sets the cache
// default expiration.
func WithDefaultExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		if d <= 0 {
			return
		}
		o.defaultExp = d
	})
}

// WithDefaultSlidingExpirationOption returns an Option that sets the cache
// default sliding expiration.
func WithDefaultSlidingExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		if d <= 0 {
			return
		}
		o.defaultExp = d
		o.sliding = true
	})
}

// WithMaxEntriesOption returns an Option that sets the cache maximum number
// of entries. When the maximum number of entries is exceeded, the least
// recently used entry is evicted regardless of the entry's expiration time.
//
// If used with Sharded type, the maximum size is per shard,
// not the total size of all shards.
func WithMaxEntriesOption[K comparable, V any](n int) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		if n <= 0 {
			return
		}
		o.maxEntriesLimit = n
	})
}

// WithCleanerOption returns an Option that sets a cache cleaner that
// periodically removes expired entries from the cache. A cleaner
// runs in a separate goroutine.
//
// A cleaner is stopped when the cache is closed.
func WithCleanerOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		if d <= 0 {
			return
		}
		o.cleanerInterval = d
	})
}
