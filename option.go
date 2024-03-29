package imcache

import "time"

// options holds the cache configuration.
type options[K comparable, V any] struct {
	onEviction      EvictionCallback[K, V]
	defaultExp      time.Duration
	sliding         bool
	maxEntriesLimit int
	evictionPolicy  EvictionPolicy
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

// WithMaxEntriesLimitOption returns an Option that sets the cache maximum
// number of entries. When the limit is exceeded, the entry is evicted
// according to the eviction policy.
//
// If used with Sharded type, the maximum size is per shard,
// not the total size of all shards.
func WithMaxEntriesLimitOption[K comparable, V any](limit int, policy EvictionPolicy) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		o.maxEntriesLimit = limit
		o.evictionPolicy = policy
	})
}

// WithMaxEntriesOption returns an Option that sets the cache maximum number
// of entries. When the maximum number of entries is exceeded, the entry is evicted
// according to the LRU eviction policy.
//
// If used with Sharded type, the maximum size is per shard,
// not the total size of all shards.
//
// Deprecated: Use WithMaxEntriesLimitOption instead.
func WithMaxEntriesOption[K comparable, V any](n int) Option[K, V] {
	return WithMaxEntriesLimitOption[K, V](n, EvictionPolicyLRU)
}

// WithCleanerOption returns an Option that sets a cache cleaner that
// periodically removes expired entries from the cache.
//
// A cleaner runs in a separate goroutine. It removes expired entries
// every interval. If the interval is less than or equal to zero,
// the cleaner is disabled.
//
// A cleaner is stopped when the cache is closed.
func WithCleanerOption[K comparable, V any](interval time.Duration) Option[K, V] {
	return optionf[K, V](func(o *options[K, V]) {
		if interval <= 0 {
			return
		}
		o.cleanerInterval = interval
	})
}
