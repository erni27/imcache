package imcache

import (
	"sync"
	"time"
)

func newShard[K comparable, V any](opts ...Option[K, V]) *shard[K, V] {
	s := &shard[K, V]{
		m:          make(map[K]entry[V]),
		defaultExp: -1,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	return s
}

type shard[K comparable, V any] struct {
	mu sync.Mutex

	m map[K]entry[V]

	defaultExp time.Duration
	sliding    bool

	onEviction EvictionCallback[K, V]
}

func (s *shard[K, V]) Get(key K) (V, error) {
	var empty V
	now := time.Now()
	s.mu.Lock()
	entry, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return empty, ErrNotFound
	}
	if entry.HasExpired(now) {
		delete(s.m, key)
		s.mu.Unlock()
		if s.onEviction != nil {
			s.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return empty, ErrNotFound
	}
	if entry.HasSlidingExpiration() {
		entry.SlideExpiration(now)
		s.m[key] = entry
	}
	s.mu.Unlock()
	return entry.val, nil
}

func (s *shard[K, V]) Set(key K, val V, exp Expiration) {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	s.m[key] = entry
	s.mu.Unlock()
	if ok && s.onEviction != nil {
		if current.HasExpired(now) {
			s.onEviction(key, current.val, EvictionReasonExpired)
		} else {
			s.onEviction(key, current.val, EvictionReasonReplaced)
		}
	}
}

func (s *shard[K, V]) Add(key K, val V, exp Expiration) error {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	if ok && !current.HasExpired(now) {
		s.mu.Unlock()
		return ErrAlreadyExists
	}
	s.m[key] = entry
	s.mu.Unlock()
	if ok && s.onEviction != nil {
		s.onEviction(key, current.val, EvictionReasonExpired)
	}
	return nil
}

func (s *shard[K, V]) Replace(key K, val V, exp Expiration) error {
	now := time.Now()
	entry := entry[V]{val: val}
	exp.apply(&entry.exp)
	entry.SetDefault(now, s.defaultExp, s.sliding)
	s.mu.Lock()
	current, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	if current.HasExpired(now) {
		delete(s.m, key)
		s.mu.Unlock()
		if s.onEviction != nil {
			s.onEviction(key, current.val, EvictionReasonExpired)
		}
		return ErrNotFound
	}
	s.m[key] = entry
	s.mu.Unlock()
	if s.onEviction != nil {
		s.onEviction(key, current.val, EvictionReasonReplaced)
	}
	return nil
}

func (s *shard[K, V]) Remove(key K) error {
	s.mu.Lock()
	entry, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.m, key)
	s.mu.Unlock()
	if entry.HasExpired(time.Now()) {
		if s.onEviction != nil {
			s.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return ErrNotFound
	}
	if s.onEviction != nil {
		s.onEviction(key, entry.val, EvictionReasonRemoved)
	}
	return nil
}

func (s *shard[K, V]) RemoveAll() {
	s.mu.Lock()
	removed := s.m
	s.m = make(map[K]entry[V])
	s.mu.Unlock()
	if s.onEviction != nil {
		for key, entry := range removed {
			if entry.HasExpired(time.Now()) {
				s.onEviction(key, entry.val, EvictionReasonExpired)
			} else {
				s.onEviction(key, entry.val, EvictionReasonRemoved)
			}
		}
	}
}

type kv[K comparable, V any] struct {
	key K
	val V
}

func (s *shard[K, V]) RemoveStale() {
	now := time.Now()
	s.mu.Lock()
	if s.onEviction == nil {
		for key, entry := range s.m {
			if entry.HasExpired(now) {
				delete(s.m, key)
			}
		}
		s.mu.Unlock()
		return
	}
	var removed []kv[K, V]
	for key, entry := range s.m {
		if entry.HasExpired(now) {
			removed = append(removed, kv[K, V]{key, entry.val})
			delete(s.m, key)
		}
	}
	s.mu.Unlock()
	for _, kv := range removed {
		s.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
}
