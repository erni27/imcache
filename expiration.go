package imcache

import "time"

const (
	noExp      = -1
	defaultExp = 0
)

type expiration struct {
	date    int64
	sliding time.Duration
}

// Expiration is the expiration time of an entry.
type Expiration interface {
	apply(*expiration)
}

type expirationf func(*expiration)

func (f expirationf) apply(e *expiration) {
	f(e)
}

// WithExpiration returns an Expiration that sets the expiration time
// to now + d.
func WithExpiration(d time.Duration) Expiration {
	return expirationf(func(e *expiration) {
		e.date = time.Now().Add(d).UnixNano()
	})
}

// WithExpirationDate returns an Expiration that sets the expiration time to t.
func WithExpirationDate(t time.Time) Expiration {
	return expirationf(func(e *expiration) {
		e.date = t.UnixNano()
	})
}

// WithSlidingExpiration returns an Expiration that sets the expiration time to
// now + d and sets the sliding expiration to d.
//
// The sliding expiration is the time after which the entry is considered
// expired if it has not been accessed. If the entry has been accessed,
// the expiration time is reset to now + d where now is the time of the access.
func WithSlidingExpiration(d time.Duration) Expiration {
	return expirationf(func(e *expiration) {
		e.date = time.Now().Add(d).UnixNano()
		e.sliding = d
	})
}

// WithNoExpiration returns an Expiration that sets the expiration time
// to never expire.
func WithNoExpiration() Expiration {
	return expirationf(func(e *expiration) {
		e.date = noExp
	})
}

// WithDefaultExpiration returns an Expiration that sets the expiration time
// to the default expiration time.
func WithDefaultExpiration() Expiration {
	return expirationf(func(e *expiration) {
		e.date = defaultExp
	})
}
