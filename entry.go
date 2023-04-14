package imcache

import "time"

// entry is the value stored in the cache.
type entry[K comparable, V any] struct {
	val  V
	exp  expiration
	node *node[K]
}

// HasExpired returns true if the entry has expired.
func (e *entry[K, V]) HasExpired(now time.Time) bool {
	return !e.HasNoExpiration() && e.exp.date < now.UnixNano()
}

// HasNoExpiration returns true if the entry has no expiration.
func (e *entry[K, V]) HasNoExpiration() bool {
	return e.exp.date < 0
}

// HasDefaultExpiration returns true if the entry has a default expiration.
func (e *entry[K, V]) HasDefaultExpiration() bool {
	return e.exp.date == defaultExp
}

// HasSlidingExpiration returns true if the entry has a sliding expiration.
func (e *entry[K, V]) HasSlidingExpiration() bool {
	return e.exp.sliding > 0
}

// SetDefault sets the expiration time if the entry has a default expiration,
// otherwise it does nothing.
func (e *entry[K, V]) SetDefault(now time.Time, d time.Duration, sliding bool) {
	if e.HasDefaultExpiration() {
		if d == noExp {
			e.exp.date = noExp
			return
		}
		e.exp.date = now.Add(d).UnixNano()
		if sliding {
			e.exp.sliding = d
		}
	}
}

// SlideExpiration sets the expiration time to now + sliding.
func (e *entry[K, V]) SlideExpiration(now time.Time) {
	e.exp.date = now.Add(e.exp.sliding).UnixNano()
}
