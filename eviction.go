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

// node is an eviction queue node.
type node[K comparable] struct {
	prev *node[K]
	next *node[K]
	key  K
}

// evictionq is the eviction queue interface.
type evictionq[K comparable] interface {
	// AddNew adds a new node to the eviction queue and returns it.
	AddNew(K) *node[K]
	// Add adds an existing node to the eviction queue.
	Add(*node[K])
	// Remove removes a node from the eviction queue.
	Remove(*node[K])
	// Pop removes and returns node from the eviction queue.
	Pop() *node[K]
}

// lruq is a simple LRU eviction queue.
type lruq[K comparable] struct {
	head *node[K]
	tail *node[K]
}

func (q *lruq[K]) AddNew(key K) *node[K] {
	n := &node[K]{key: key}
	q.Add(n)
	return n
}

func (q *lruq[K]) Add(n *node[K]) {
	if q.head == nil {
		q.tail = n
	} else {
		q.head.prev = n
		n.next = q.head
	}
	q.head = n
}

func (q *lruq[K]) Remove(n *node[K]) {
	if n.prev == nil {
		q.head = n.next
	} else {
		n.prev.next = n.next
	}
	if n.next == nil {
		q.tail = n.prev
	} else {
		n.next.prev = n.prev
	}
}

func (q *lruq[K]) Pop() *node[K] {
	n := q.tail
	if n != nil {
		q.Remove(n)
	}
	return n
}

// nopq is a no-op eviction queue.
// It is used when the cache does not have a max entries limit.
type nopq[K comparable] struct{}

func (*nopq[K]) AddNew(K) *node[K] { return nil }

func (*nopq[K]) Add(*node[K]) {}

func (*nopq[K]) Remove(*node[K]) {}

func (*nopq[K]) Pop() *node[K] {
	// imcache is carefully designed to never call Pop on a NOP queue.
	panic("imcache: Pop called on a NOP queue")
}
