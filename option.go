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
	return optionf(func(c *shard) {
		c.evictionc = f
	})
}

// WithDefaultExpirationOption returns an Option that sets the default expiration.
func WithDefaultExpirationOption(d time.Duration) Option {
	return optionf(func(c *shard) {
		c.defaultexp = d
	})
}

// WithDefaultSlidingExpirationOption returns an Option that sets the default sliding expiration.
func WithDefaultSlidingExpirationOption(d time.Duration) Option {
	return optionf(func(c *shard) {
		c.defaultexp = d
		c.sliding = true
	})
}
