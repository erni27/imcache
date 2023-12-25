package imcache

import "time"

const noExp = -1

type expiration struct {
	date    int64
	sliding time.Duration
}

// Expiration is the expiration time of an entry.
type Expiration interface {
	new(now time.Time, defaultExp time.Duration, sliding bool) expiration
}

type expirationf func(time.Time, time.Duration, bool) expiration

func (f expirationf) new(now time.Time, defaultExp time.Duration, sliding bool) expiration {
	return f(now, defaultExp, sliding)
}

// WithExpiration returns an Expiration that sets the expiration time
// to now + d.
func WithExpiration(d time.Duration) Expiration {
	return expirationf(func(now time.Time, _ time.Duration, _ bool) expiration {
		return expiration{date: now.Add(d).UnixNano()}
	})
}

// WithExpirationDate returns an Expiration that sets the expiration time to t.
func WithExpirationDate(t time.Time) Expiration {
	return expirationf(func(_ time.Time, _ time.Duration, _ bool) expiration {
		return expiration{date: t.UnixNano()}
	})
}

// WithSlidingExpiration returns an Expiration that sets the expiration time to
// now + d and sets the sliding expiration to d.
//
// The sliding expiration is the time after which the entry is considered
// expired if it has not been accessed. If the entry has been accessed,
// the expiration time is reset to now + d where now is the time of the access.
func WithSlidingExpiration(d time.Duration) Expiration {
	return expirationf(func(now time.Time, _ time.Duration, _ bool) expiration {
		return expiration{date: now.Add(d).UnixNano(), sliding: d}
	})
}

// WithNoExpiration returns an Expiration that sets the expiration time
// to never expire.
func WithNoExpiration() Expiration {
	return expirationf(func(_ time.Time, _ time.Duration, _ bool) expiration {
		return expiration{date: noExp}
	})
}

// WithDefaultExpiration returns an Expiration that sets the expiration time
// to the default expiration time.
func WithDefaultExpiration() Expiration {
	return expirationf(func(now time.Time, defaultExp time.Duration, sliding bool) expiration {
		if defaultExp == noExp {
			return expiration{date: noExp}
		}
		var slidingExp time.Duration
		if sliding {
			slidingExp = defaultExp
		}
		return expiration{date: now.Add(defaultExp).UnixNano(), sliding: slidingExp}
	})
}
