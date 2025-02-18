package autocache

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	// ErrNotExist indicates that the key does not exist in the cache.
	ErrNotExist = errors.New("not exist")
	// ErrChanFull indicates that the task channel is full.
	ErrChanFull = errors.New("chan full")
	// ErrNotFound indicates that the key was not found after a back-source query.
	ErrNotFound = errors.New("not found")
)

// BackingFunc is a function type for back-source queries.
type BackingFunc func(ctx context.Context, keys []string) (map[string]interface{}, error)

// AutoCache represents the cache structure.
type AutoCache struct {
	backing    BackingFunc
	opts       *Options
	mu         sync.Mutex
	cache      *Cache
	loading    map[string]chan struct{}
	taskChan   chan string
	stopChan   chan struct{}
	loadTimer  *time.Timer
	cleanTimer *time.Timer
}

// NewAutoCache creates a new cache instance.
func NewAutoCache(backing BackingFunc, options ...Option) *AutoCache {
	opts := NewDefaultOptions()
	for _, option := range options {
		option(opts)
	}

	cache := &AutoCache{
		opts:       opts,
		cache:      NewCache(opts.Expiration),
		loading:    make(map[string]chan struct{}),
		taskChan:   make(chan string, opts.LoadBuffSize),
		backing:    backing,
		stopChan:   make(chan struct{}),
		loadTimer:  time.NewTimer(opts.LoadInterval),
		cleanTimer: time.NewTimer(opts.CleanInterval),
	}
	go cache.load()
	go cache.cleanup()
	return cache
}

// Get retrieves a value from the cache.
func (c *AutoCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	if val, ok := c.cache.Get(key); ok {
		c.mu.Unlock()
		if err, oke := val.(error); oke {
			return nil, err
		}
		return val, nil
	}

	ch, ok := c.loading[key]
	if !ok {
		select {
		case c.taskChan <- key:
			ch = make(chan struct{})
			c.loading[key] = ch
		default:
			c.mu.Unlock()
			return nil, ErrChanFull
		}
	}
	c.mu.Unlock()

	select {
	case <-ch:
		c.mu.Lock()
		val, ok := c.cache.Get(key)
		c.mu.Unlock()
		if !ok {
			return nil, ErrNotFound
		}
		return val, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TryGet is a non-blocking version of Get.
func (c *AutoCache) TryGet(key string) (interface{}, error) {
	c.mu.Lock()
	if val, ok := c.cache.Get(key); ok {
		c.mu.Unlock()
		if err, oke := val.(error); oke {
			return nil, err
		}
		return val, nil
	}
	ch, ok := c.loading[key]
	if !ok {
		select {
		case c.taskChan <- key:
			ch = make(chan struct{})
			c.loading[key] = ch
		default:
			c.mu.Unlock()
			return nil, ErrChanFull
		}
	}
	c.mu.Unlock()
	return nil, ErrNotExist
}

// load is a background goroutine for batch back-source loading.
func (c *AutoCache) load() {
	var batch []string
	for {
		select {
		case key := <-c.taskChan:
			batch = append(batch, key)
			if len(batch) >= c.opts.LoadBatchSize {
				c.loadTimer.Stop()
				c.batchLoad(batch)
				batch = nil
				c.loadTimer.Reset(c.opts.LoadInterval)
			}
		case <-c.loadTimer.C:
			if len(batch) > 0 {
				c.loadTimer.Stop()
				c.batchLoad(batch)
				batch = nil
			}
			c.loadTimer.Reset(c.opts.LoadInterval)
		case <-c.stopChan:
			if c.loadTimer.Stop() {
				select {
				case <-c.loadTimer.C:
				default:
				}
			}
			return
		}
	}
}

func (c *AutoCache) cleanup() {
	for {
		select {
		case <-c.cleanTimer.C:
			c.cleanTimer.Stop()
			c.mu.Lock()
			c.cache.Cleanup()
			c.mu.Unlock()
			c.cleanTimer.Reset(c.opts.CleanInterval)
		case <-c.stopChan:
			if c.cleanTimer.Stop() {
				select {
				case <-c.cleanTimer.C:
				default:
				}
			}
			return
		}
	}
}

// batchLoad processes batch back-source queries.
func (c *AutoCache) batchLoad(keys []string) {
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.LoadTimeout)
	defer cancel()
	values, err := c.backing(ctx, keys)
	if err != nil {
		log.Printf("Error loading keys: %v\n", err)
		c.mu.Lock()
		for _, key := range keys {
			if ch, ok := c.loading[key]; ok {
				close(ch)
				delete(c.loading, key)
			}
		}
		c.mu.Unlock()
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range keys {
		if val, ok := values[key]; ok {
			c.cache.Set(key, val, c.opts.Expiration)
		} else {
			c.cache.Set(key, ErrNotExist, c.opts.Expiration)
		}
		if ch, ok := c.loading[key]; ok {
			close(ch)
			delete(c.loading, key)
		}
	}
}

// Close closes the cache and stops the background goroutine.
func (c *AutoCache) Close() {
	close(c.stopChan)
}
