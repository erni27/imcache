package imcache

import "math/rand"

// EvictionPolicy represents the eviction policy.
type EvictionPolicy int32

const (
	// EvictionPolicyLRU is the least recently used eviction policy.
	EvictionPolicyLRU EvictionPolicy = iota + 1
	// EvictionPolicyLFU is the least frequently used eviction policy.
	EvictionPolicyLFU
	// EvictionPolicyRandom is the random eviction policy.
	// It evicts the entry randomly when the max entries limit exceeded.
	EvictionPolicyRandom
)

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

func newEvictionQueue[K comparable, V any](limit int, policy EvictionPolicy) evictionQueue[K, V] {
	if limit <= 0 {
		return nopEvictionQueue[K, V]{}
	}
	switch policy {
	case EvictionPolicyLRU:
		return &lruEvictionQueue[K, V]{}
	case EvictionPolicyLFU:
		return &lfuEvictionQueue[K, V]{freqs: make(map[int]*lfuLruEvictionQueue[K, V])}
	case EvictionPolicyRandom:
		var q randomEvictionQueue[K, V] = make([]*randomNode[K, V], 0, limit)
		return &q
	}
	return nopEvictionQueue[K, V]{}
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
	// touchall updates the node positions in the eviction queue
	// according to the eviction policy in the context of the
	// GetAll operation.
	//
	// It simplifies preserving the eviction policy when the GetAll
	// operation is used. Some eviction policies (e.g. LRU) may
	// require to not update the node position in the eviction queue
	// when the GetAll operation is used while other eviction policies
	// (e.g. LFU) may require the update.
	touchall()
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

func (n *lruNode[K, V]) setEntry(e entry[K, V]) {
	n.entr = e
}

//lint:ignore U1000 false positive
type lruEvictionQueue[K comparable, V any] struct {
	head *lruNode[K, V]
	tail *lruNode[K, V]
}

func (q *lruEvictionQueue[K, V]) add(e entry[K, V]) node[K, V] {
	n := &lruNode[K, V]{entr: e}
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
	q.remove(n)
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

func (q *lruEvictionQueue[K, V]) touchall() {}

//lint:ignore U1000 false positive
type lfuNode[K comparable, V any] struct {
	entr entry[K, V]
	next *lfuNode[K, V]
	prev *lfuNode[K, V]
	freq int
}

func (n *lfuNode[K, V]) entry() entry[K, V] {
	return n.entr
}

func (n *lfuNode[K, V]) setEntry(e entry[K, V]) {
	n.entr = e
}

//lint:ignore U1000 false positive
type lfuLruEvictionQueue[K comparable, V any] struct {
	next *lfuLruEvictionQueue[K, V]
	prev *lfuLruEvictionQueue[K, V]
	head *lfuNode[K, V]
	tail *lfuNode[K, V]
	freq int
}

func (q *lfuLruEvictionQueue[K, V]) add(n *lfuNode[K, V]) {
	if q.head == nil {
		q.tail = n
	} else {
		q.head.prev = n
		n.next = q.head
	}
	q.head = n
}

func (q *lfuLruEvictionQueue[K, V]) remove(n *lfuNode[K, V]) {
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
	n.next, n.prev = nil, nil
}

func (q *lfuLruEvictionQueue[K, V]) pop() *lfuNode[K, V] {
	n := q.tail
	q.remove(n)
	return n
}

//lint:ignore U1000 false positive
type lfuEvictionQueue[K comparable, V any] struct {
	freqs map[int]*lfuLruEvictionQueue[K, V]
	head  *lfuLruEvictionQueue[K, V]
	tail  *lfuLruEvictionQueue[K, V]
}

func (q *lfuEvictionQueue[K, V]) add(e entry[K, V]) node[K, V] {
	n := &lfuNode[K, V]{entr: e, freq: 1}
	lruq, ok := q.freqs[1]
	if !ok {
		lruq = &lfuLruEvictionQueue[K, V]{freq: 1}
		q.freqs[1] = lruq
		if q.tail != nil {
			q.tail.next = lruq
			lruq.prev = q.tail
		}
		q.tail = lruq
		if q.head == nil {
			q.head = lruq
		}
	}
	lruq.add(n)
	return n
}

func (q *lfuEvictionQueue[K, V]) pop() node[K, V] {
	return q.tail.pop()
}

func (q *lfuEvictionQueue[K, V]) remove(n node[K, V]) {
	lfun := n.(*lfuNode[K, V])
	lruq := q.freqs[lfun.freq]
	lruq.remove(lfun)
	if lruq.head != nil {
		return
	}
	q.removeq(lruq)
}

func (q *lfuEvictionQueue[K, V]) removeq(lruq *lfuLruEvictionQueue[K, V]) {
	if lruq.prev == nil {
		q.head = lruq.next
	} else {
		lruq.prev.next = lruq.next
	}
	if lruq.next == nil {
		q.tail = lruq.prev
	} else {
		lruq.next.prev = lruq.prev
	}
	delete(q.freqs, lruq.freq)
}

func (q *lfuEvictionQueue[K, V]) touch(n node[K, V]) {
	lfun := n.(*lfuNode[K, V])
	lruq := q.freqs[lfun.freq]
	lruq.remove(lfun)
	lfun.freq++
	newlruq, ok := q.freqs[lfun.freq]
	if !ok {
		newlruq = &lfuLruEvictionQueue[K, V]{freq: lfun.freq}
		q.freqs[newlruq.freq] = newlruq
		newlruq.prev = lruq.prev
		newlruq.next = lruq
		if lruq.prev == nil {
			q.head = newlruq
		} else {
			newlruq.prev.next = newlruq
		}
		lruq.prev = newlruq
	}
	if lruq.head == nil {
		q.removeq(lruq)
	}
	newlruq.add(lfun)
}

func (q *lfuEvictionQueue[K, V]) touchall() {
	for lruq := q.head; lruq != nil; lruq = lruq.next {
		for n := lruq.head; n != nil; n = n.next {
			n.freq++
		}
		delete(q.freqs, lruq.freq)
		lruq.freq++
		q.freqs[lruq.freq] = lruq
	}
}

//lint:ignore U1000 false positive
type randomNode[K comparable, V any] struct {
	entr entry[K, V]
	idx  int
}

func (n *randomNode[K, V]) entry() entry[K, V] {
	return n.entr
}

func (n *randomNode[K, V]) setEntry(e entry[K, V]) {
	n.entr = e
}

//lint:ignore U1000 false positive
type randomEvictionQueue[K comparable, V any] []*randomNode[K, V]

func (q *randomEvictionQueue[K, V]) add(e entry[K, V]) node[K, V] {
	n := &randomNode[K, V]{entr: e}
	*q = append(*q, n)
	n.idx = len(*q) - 1
	return n
}

func (q *randomEvictionQueue[K, V]) remove(n node[K, V]) {
	randn := n.(*randomNode[K, V])
	updated := *q
	updated[randn.idx], updated[len(updated)-1] = updated[len(updated)-1], updated[randn.idx]
	updated[randn.idx].idx = randn.idx
	updated = updated[:len(updated)-1]
	*q = updated
}

func (q *randomEvictionQueue[K, V]) pop() node[K, V] {
	idx := rand.Intn(len(*q))
	n := (*q)[idx]
	q.remove(n)
	return n
}

func (randomEvictionQueue[K, V]) touch(node[K, V]) {}

func (randomEvictionQueue[K, V]) touchall() {}

//lint:ignore U1000 false positive
type nopNode[K comparable, V any] entry[K, V]

func (n *nopNode[K, V]) entry() entry[K, V] {
	return (entry[K, V])(*n)
}

func (n *nopNode[K, V]) setEntry(e entry[K, V]) {
	*n = nopNode[K, V](e)
}

//lint:ignore U1000 false positive
type nopEvictionQueue[K comparable, V any] struct{}

func (nopEvictionQueue[K, V]) add(e entry[K, V]) node[K, V] {
	return (*nopNode[K, V])(&e)
}

func (nopEvictionQueue[K, V]) remove(node[K, V]) {}

func (nopEvictionQueue[K, V]) pop() node[K, V] {
	// imcache is carefully designed to never call pop on a NOP queue.
	panic("imcache: pop called on a NOP eviction queue")
}

func (nopEvictionQueue[K, V]) touch(node[K, V]) {}

func (nopEvictionQueue[K, V]) touchall() {}
