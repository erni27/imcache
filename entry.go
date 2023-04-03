package imcache

import "time"

// entry is the value stored in the cache.
type entry struct {
	val        interface{}
	expiration int64
	sliding    time.Duration
}

// HasExpired returns true if the entry has expired.
func (e *entry) HasExpired(now time.Time) bool {
	return !e.HasNoExpiration() && e.expiration < now.UnixNano()
}

// HasNoExpiration returns true if the entry has no expiration.
func (e *entry) HasNoExpiration() bool {
	return e.expiration < 0
}

// HasDefaultExpiration returns true if the entry has a default expiration.
func (e *entry) HasDefaultExpiration() bool {
	return e.expiration == defaultExp
}

// HasSlidingExpiration returns true if the entry has a sliding expiration.
func (e *entry) HasSlidingExpiration() bool {
	return e.sliding > 0
}

// SetDefault sets the expiration time if the entry has a default expiration,
// otherwise it does nothing.
func (e *entry) SetDefault(now time.Time, d time.Duration, sliding bool) {
	if e.HasDefaultExpiration() {
		if d == noExp {
			e.expiration = noExp
			return
		}
		e.expiration = now.Add(d).UnixNano()
		if sliding {
			e.sliding = d
		}
	}
}

// SlideExpiration sets the expiration time to now + sliding.
func (e *entry) SlideExpiration(now time.Time) {
	e.expiration = now.Add(e.sliding).UnixNano()
}
