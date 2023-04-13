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
	// EvictionReasonSizeExceeded indicates that the entry was evicted because the cache size exceeded.
	// imcache uses LRU eviction policy when the cache size exceeds the maximum size.
	EvictionReasonSizeExceeded
)

func (r EvictionReason) String() string {
	switch r {
	case EvictionReasonExpired:
		return "expired"
	case EvictionReasonRemoved:
		return "removed"
	case EvictionReasonReplaced:
		return "replaced"
	case EvictionReasonSizeExceeded:
		return "size exceeded"
	default:
		return "unknown"
	}
}

// EvictionCallback is the callback function that is called when an entry is evicted.
type EvictionCallback[K comparable, V any] func(key K, val V, reason EvictionReason)
