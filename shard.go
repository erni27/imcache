package imcache

import (
	"sync"
	"time"
)

func newShard(opts ...Option) *shard {
	c := &shard{
		m:          make(map[string]entry),
		defaultexp: -1,
	}
	for _, opt := range opts {
		opt.apply(c)
	}
	return c
}

type shard struct {
	mu sync.Mutex

	m map[string]entry

	defaultexp time.Duration
	sliding    bool

	evictionc EvictionCallback
}

func (c *shard) Get(key string) (interface{}, error) {
	now := time.Now()
	c.mu.Lock()
	entry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return nil, ErrNotFound
	}
	if entry.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.evictionc != nil {
			c.evictionc(key, entry.val, EvictionReasonExpired)
		}
		return nil, ErrNotFound
	}
	if entry.HasSlidingExpiration() {
		entry.SlideExpiration(now)
		c.m[key] = entry
	}
	c.mu.Unlock()
	return entry.val, nil
}

func (c *shard) Set(key string, val interface{}, exp Expiration) {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
	entry.SetDefault(now, c.defaultexp, c.sliding)
	c.mu.Lock()
	current, ok := c.m[key]
	c.m[key] = entry
	c.mu.Unlock()
	if ok && c.evictionc != nil {
		if current.HasExpired(now) {
			c.evictionc(key, current.val, EvictionReasonExpired)
		} else {
			c.evictionc(key, current.val, EvictionReasonReplaced)
		}
	}
}

func (c *shard) Add(key string, val interface{}, exp Expiration) error {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
	entry.SetDefault(now, c.defaultexp, c.sliding)
	c.mu.Lock()
	current, ok := c.m[key]
	if ok && !current.HasExpired(now) {
		c.mu.Unlock()
		return ErrAlreadyExists
	}
	c.m[key] = entry
	c.mu.Unlock()
	if ok && c.evictionc != nil {
		c.evictionc(key, current.val, EvictionReasonExpired)
	}
	return nil
}

func (c *shard) Replace(key string, val interface{}, exp Expiration) error {
	now := time.Now()
	entry := entry{val: val}
	exp.apply(&entry)
	entry.SetDefault(now, c.defaultexp, c.sliding)
	c.mu.Lock()
	current, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return ErrNotFound
	}
	if current.HasExpired(now) {
		delete(c.m, key)
		c.mu.Unlock()
		if c.evictionc != nil {
			c.evictionc(key, current.val, EvictionReasonExpired)
		}
		return ErrNotFound
	}
	c.m[key] = entry
	c.mu.Unlock()
	if c.evictionc != nil {
		c.evictionc(key, current.val, EvictionReasonReplaced)
	}
	return nil
}

func (c *shard) Remove(key string) error {
	c.mu.Lock()
	entry, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return ErrNotFound
	}
	delete(c.m, key)
	c.mu.Unlock()
	if entry.HasExpired(time.Now()) {
		if c.evictionc != nil {
			c.evictionc(key, entry.val, EvictionReasonExpired)
		}
		return ErrNotFound
	}
	if c.evictionc != nil {
		c.evictionc(key, entry.val, EvictionReasonRemoved)
	}
	return nil
}

func (c *shard) RemoveAll() {
	c.mu.Lock()
	removed := c.m
	c.m = make(map[string]entry)
	c.mu.Unlock()
	if c.evictionc != nil {
		for key, entry := range removed {
			if entry.HasExpired(time.Now()) {
				c.evictionc(key, entry.val, EvictionReasonExpired)
			} else {
				c.evictionc(key, entry.val, EvictionReasonRemoved)
			}
		}
	}
}

func (c *shard) RemoveStale() {
	now := time.Now()
	c.mu.Lock()
	if c.evictionc == nil {
		for key, entry := range c.m {
			if entry.HasExpired(now) {
				delete(c.m, key)
			}
		}
		c.mu.Unlock()
		return
	}
	type kv struct {
		key string
		val interface{}
	}
	var removed []kv
	for key, entry := range c.m {
		if entry.HasExpired(now) {
			removed = append(removed, kv{key, entry.val})
			delete(c.m, key)
		}
	}
	c.mu.Unlock()
	for _, kv := range removed {
		c.evictionc(kv.key, kv.val, EvictionReasonExpired)
	}
}
