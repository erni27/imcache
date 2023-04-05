package imcache

// EvictionReason is the reason why an entry was evicted.
type EvictionReason int32

const (
	// EvictionReasonExpired indicates that the entry was evicted because it expired.
	EvictionReasonExpired EvictionReason = iota + 1
	// EvictionReasonRemoved indicates that the entry was evicted because it was removed.
	EvictionReasonRemoved
	// EvictionReasonReplaced indicates that the entry was evicted because it was replaced.
	EvictionReasonReplaced
)

// EvictionCallback is the callback function that is called when an entry is evicted.
type EvictionCallback[K comparable, V any] func(key K, val V, reason EvictionReason)
