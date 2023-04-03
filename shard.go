package imcache

import (
	"sync"
	"time"
)

func newShard(opts ...Option) *shard {
	s := &shard{
		m:          make(map[string]entry),
		defaultExp: -1,
	}
	for _, opt := range opts {
		opt.apply(s)
	}
	if s.cleanerInterval > 0 {
		go func() {
			ticker := time.NewTicker(s.cleanerInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.RemoveStale()
				case <-s.cleanerStop:
					return
				}
			}
		}()
	}
	return s
}

type shard struct {
	mu sync.Mutex

	m map[string]entry

	defaultExp time.Duration
	sliding    bool

	onEviction EvictionCallback

	cleaner         bool
	cleanerInterval time.Duration
	cleanerStop     chan struct{}
}

func (s *shard) Get(key string) (interface{}, error) {
	now := time.Now()
	s.mu.Lock()
	entry, ok := s.m[key]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if entry.HasExpired(now) {
		delete(s.m, key)
		s.mu.Unlock()
		if s.onEviction != nil {
			s.onEviction(key, entry.val, EvictionReasonExpired)
		}
		return nil, ErrNotFound
	}
	if entry.HasSlidingExpiration() {
		entry.SlideExpiration(now)
		s.m[key] = entry
	}
	s.mu.Unlock()
	return entry.val, nil
}

func (s *shard) Set(key string, val interface{}, exp Expiration) {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
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

func (s *shard) Add(key string, val interface{}, exp Expiration) error {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
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

func (s *shard) Replace(key string, val interface{}, exp Expiration) error {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
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

func (s *shard) Remove(key string) error {
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

func (s *shard) RemoveAll() {
	s.mu.Lock()
	removed := s.m
	s.m = make(map[string]entry)
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

func (s *shard) RemoveStale() {
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
	type kv struct {
		key string
		val interface{}
	}
	var removed []kv
	for key, entry := range s.m {
		if entry.HasExpired(now) {
			removed = append(removed, kv{key, entry.val})
			delete(s.m, key)
		}
	}
	s.mu.Unlock()
	for _, kv := range removed {
		s.onEviction(kv.key, kv.val, EvictionReasonExpired)
	}
}
