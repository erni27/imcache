package imcache

import "time"

const (
	noExp      = -1
	defaultExp = 0
)

// Expiration is the expiration time of an entry.
type Expiration interface {
	apply(*entry)
}

type expirationf func(*entry)

func (f expirationf) apply(e *entry) {
	f(e)
}

// WithExpiration returns an Expiration that sets the expiration time to now + d.
func WithExpiration(d time.Duration) Expiration {
	return expirationf(func(e *entry) {
		e.expiration = time.Now().Add(d).UnixNano()
	})
}

// WithSlidingExpiration returns an Expiration that sets the expiration time to
// now + d and sets the sliding expiration to d.
func WithSlidingExpiration(d time.Duration) Expiration {
	return expirationf(func(e *entry) {
		e.expiration = time.Now().Add(d).UnixNano()
		e.sliding = d
	})
}

// WithNoExpiration returns an Expiration that sets the expiration time to never expire.
func WithNoExpiration() Expiration {
	return expirationf(func(e *entry) {
		e.expiration = noExp
	})
}

// WithDefaultExpiration returns an Expiration that sets the expiration time to the
// default expiration time.
func WithDefaultExpiration() Expiration {
	return expirationf(func(e *entry) {
		e.expiration = defaultExp
	})
}
