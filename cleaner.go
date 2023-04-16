package imcache

import (
	"errors"
	"sync"
	"time"
)

// eremover is an interface that wraps the RemoveExpired method.
type eremover interface {
	RemoveExpired()
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

func (c *cleaner) start(r eremover, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("imcache: interval must be greater than 0")
	}
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return errors.New("imcache: cleaner already running")
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
				r.RemoveExpired()
			case <-c.stopCh:
				close(c.doneCh)
				return
			}
		}
	}()
	return nil
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
