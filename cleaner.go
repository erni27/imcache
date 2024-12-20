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
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
	running bool
}

func (c *cleaner) start(r eremover, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("imcache: interval must be greater than 0")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return errors.New("imcache: cleaner already running")
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})
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
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)
	// Wait for the cleaner goroutine to stop while holding the lock.
	<-c.doneCh
}
