package imcache

type node[K comparable] struct {
	prev *node[K]
	next *node[K]
	key  K
}

type queue[K comparable] interface {
	AddNew(K) *node[K]
	Add(*node[K])
	Remove(*node[K])
	Pop() *node[K]
}

type fifoq[K comparable] struct {
	head *node[K]
	tail *node[K]
}

func (q *fifoq[K]) AddNew(key K) *node[K] {
	n := &node[K]{key: key}
	q.Add(n)
	return n
}

func (q *fifoq[K]) Add(n *node[K]) {
	if q.head == nil {
		q.tail = n
	} else {
		q.head.prev = n
		n.next = q.head
	}
	q.head = n
}

func (q *fifoq[K]) Remove(n *node[K]) {
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

func (q *fifoq[K]) Pop() *node[K] {
	n := q.tail
	if n != nil {
		q.Remove(n)
	}
	return n
}

type nopq[K comparable] struct{}

func (*nopq[K]) AddNew(K) *node[K] { return nil }

func (*nopq[K]) Add(*node[K]) {}

func (*nopq[K]) Remove(*node[K]) {}

func (*nopq[K]) Pop() *node[K] { return nil }
