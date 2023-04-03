package imcache

import "time"

// Option is a Cache option.
type Option interface {
	apply(*shard)
}

type optionf func(*shard)

func (f optionf) apply(c *shard) {
	f(c)
}

// WithEvictionCallbackOption returns an Option that sets the eviction callback.
func WithEvictionCallbackOption(f EvictionCallback) Option {
	return optionf(func(s *shard) {
		s.onEviction = f
	})
}

// WithDefaultExpirationOption returns an Option that sets the default expiration.
func WithDefaultExpirationOption(d time.Duration) Option {
	return optionf(func(s *shard) {
		s.defaultExp = d
	})
}

// WithDefaultSlidingExpirationOption returns an Option that sets the default sliding expiration.
func WithDefaultSlidingExpirationOption(d time.Duration) Option {
	return optionf(func(s *shard) {
		s.defaultExp = d
		s.sliding = true
	})
}

// WithCleanerOption returns an Option that turns on the cleaner.
// The cleaner removes stale entries periodically from the cache.
// It spawns a goroutine per shard.
func WithCleanerOption(d time.Duration) Option {
	return optionf(func(s *shard) {
		s.cleaner = true
		s.cleanerInterval = d
		s.cleanerStop = make(chan struct{})
	})
}
