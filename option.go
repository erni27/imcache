package imcache

import "time"

// Option is a Cache option.
type Option[K comparable, V any] interface {
	apply(*shard[K, V])
}

type optionf[K comparable, V any] func(*shard[K, V])

//lint:ignore U1000 false positive
func (f optionf[K, V]) apply(c *shard[K, V]) {
	f(c)
}

// WithEvictionCallbackOption returns an Option that sets the eviction callback.
func WithEvictionCallbackOption[K comparable, V any](f EvictionCallback[K, V]) Option[K, V] {
	return optionf[K, V](func(s *shard[K, V]) {
		s.onEviction = f
	})
}

// WithDefaultExpirationOption returns an Option that sets the default expiration.
func WithDefaultExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(s *shard[K, V]) {
		s.defaultExp = d
	})
}

// WithDefaultSlidingExpirationOption returns an Option that sets the default sliding expiration.
func WithDefaultSlidingExpirationOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(s *shard[K, V]) {
		s.defaultExp = d
		s.sliding = true
	})
}

// WithCleanerOption returns an Option that turns on the cleaner.
// The cleaner removes stale entries periodically from the cache.
// It spawns a goroutine per shard.
func WithCleanerOption[K comparable, V any](d time.Duration) Option[K, V] {
	return optionf[K, V](func(s *shard[K, V]) {
		s.cleaner = true
		s.cleanerInterval = d
		s.cleanerStop = make(chan struct{})
	})
}
