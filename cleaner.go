package imcache

import (
	"sync"
	"time"
)

// remover is an interface that wraps the RemoveStale method.
type remover interface {
	RemoveStale()
}

func newCleaner() *cleaner {
	return &cleaner{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

type cleaner struct {
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func (c *cleaner) start(r remover, interval time.Duration) {
	if interval <= 0 {
		panic("imcache: interval must be greater than 0")
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})
	c.mu.Unlock()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.RemoveStale()
			case <-c.stopCh:
				close(c.doneCh)
				return
			}
		}
	}()
}

func (c *cleaner) stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	// Wait for the cleaner goroutine to stop while holding the lock.
	<-c.doneCh
	c.mu.Unlock()
}
