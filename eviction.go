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
	// EvictionReasonKeyReplaced indicates that the entry was evicted
	// because the key was replaced.
	EvictionReasonKeyReplaced
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
	case EvictionReasonKeyReplaced:
		return "key replaced"
	default:
		return "unknown"
	}
}

// EvictionCallback is the callback function that is called when an entry
// is evicted.
type EvictionCallback[K comparable, V any] func(key K, val V, reason EvictionReason)

// node is the eviction queue node interface.
type node[K comparable, V any] interface {
	// entry returns the imcache entry associated with the node.
	entry() entry[K, V]
	// setEntry sets the imcache entry associated with the node.
	setEntry(entry[K, V])
}

// evictionQueue is the eviction queue interface.
// It is used to implement different eviction policies.
type evictionQueue[K comparable, V any] interface {
	// add adds a new node to the eviction queue and returns it.
	add(entry[K, V]) node[K, V]
	// remove removes the node from the eviction queue.
	remove(node[K, V])
	// pop removes and returns the node from the eviction queue
	// according to the eviction policy.
	pop() node[K, V]
	// touch updates the node position in the eviction queue
	// according to the eviction policy.
	touch(node[K, V])
}

//lint:ignore U1000 false positive
type lruNode[K comparable, V any] struct {
	entr entry[K, V]
	prev *lruNode[K, V]
	next *lruNode[K, V]
}

func (n *lruNode[K, V]) entry() entry[K, V] {
	return n.entr
}

func (n *lruNode[K, V]) setEntry(entr entry[K, V]) {
	n.entr = entr
}

//lint:ignore U1000 false positive
type lruEvictionQueue[K comparable, V any] struct {
	head *lruNode[K, V]
	tail *lruNode[K, V]
}

func (q *lruEvictionQueue[K, V]) add(entr entry[K, V]) node[K, V] {
	n := &lruNode[K, V]{entr: entr}
	if q.head == nil {
		q.tail = n
	} else {
		q.head.prev = n
		n.next = q.head
	}
	q.head = n
	return n
}

func (q *lruEvictionQueue[K, V]) remove(n node[K, V]) {
	lrun := n.(*lruNode[K, V])
	if lrun.prev == nil {
		q.head = lrun.next
	} else {
		lrun.prev.next = lrun.next
	}
	if lrun.next == nil {
		q.tail = lrun.prev
	} else {
		lrun.next.prev = lrun.prev
	}
}

func (q *lruEvictionQueue[K, V]) pop() node[K, V] {
	n := q.tail
	if n != nil {
		q.remove(n)
	}
	return n
}

func (q *lruEvictionQueue[K, V]) touch(n node[K, V]) {
	lrun := n.(*lruNode[K, V])
	if lrun.prev == nil {
		return
	}
	if lrun.next == nil {
		q.tail = lrun.prev
	} else {
		lrun.next.prev = lrun.prev
	}
	lrun.prev.next = lrun.next
	q.head.prev = lrun
	lrun.next = q.head
	lrun.prev = nil
	q.head = lrun
}

//lint:ignore U1000 false positive
type nopNode[K comparable, V any] entry[K, V]

func (n *nopNode[K, V]) entry() entry[K, V] {
	return (entry[K, V])(*n)
}

func (n *nopNode[K, V]) setEntry(entr entry[K, V]) {
	*n = nopNode[K, V](entr)
}

//lint:ignore U1000 false positive
type nopEvictionQueue[K comparable, V any] struct{}

func (nopEvictionQueue[K, V]) add(entr entry[K, V]) node[K, V] {
	return (*nopNode[K, V])(&entr)
}

func (nopEvictionQueue[K, V]) remove(node[K, V]) {}

func (nopEvictionQueue[K, V]) pop() node[K, V] {
	// imcache is carefully designed to never call pop on a NOP queue.
	panic("imcache: pop called on a NOP eviction queue")
}

func (nopEvictionQueue[K, V]) touch(node[K, V]) {}
