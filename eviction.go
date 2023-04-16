package imcache

// EvictionReason is the reason why an entry was evicted.
type EvictionReason int32

const (
	// EvictionReasonExpired indicates that the entry was evicted
	// because it expired.
	EvictionReasonExpired EvictionReason = iota + 1
	// EvictionReasonRemoved indicates that the entry was evicted
	// because it was removed.
	EvictionReasonRemoved
	// EvictionReasonReplaced indicates that the entry was evicted
	// because it was replaced.
	EvictionReasonReplaced
	// EvictionReasonMaxEntriesExceeded indicates that the entry was evicted
	// because the max entries limit was exceeded.
	//
	// imcache uses LRU eviction policy when the cache exceeds the max entries
	// limit. The least recently used entry is evicted regardless of the
	// entry's expiration time.
	EvictionReasonMaxEntriesExceeded
)

func (r EvictionReason) String() string {
	switch r {
	case EvictionReasonExpired:
		return "expired"
	case EvictionReasonRemoved:
		return "removed"
	case EvictionReasonReplaced:
		return "replaced"
	case EvictionReasonMaxEntriesExceeded:
		return "max entries limit exceeded"
	default:
		return "unknown"
	}
}

// EvictionCallback is the callback function that is called when an entry
// is evicted.
type EvictionCallback[K comparable, V any] func(key K, val V, reason EvictionReason)
