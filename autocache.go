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

// BackSourceFunc is a function type for back-source queries.
type BackSourceFunc func(ctx context.Context, keys []string) (map[string]interface{}, error)

// Options defines the configuration options for the AutoCache.
type Options struct {
	BatchSize     int
	BatchInterval time.Duration
	BatchTimeout  time.Duration
	ChanSize      int
	Expiration    time.Duration
	CleanInterval time.Duration
}

// Option is a function type for configuring Options.
type Option func(*Options)

// AutoCache represents the cache structure.
type AutoCache struct {
	backing    BackSourceFunc
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
func NewAutoCache(backing BackSourceFunc, options ...Option) *AutoCache {
	opts := &Options{
		BatchSize:     10,
		BatchInterval: 100 * time.Millisecond,
		BatchTimeout:  3 * time.Second,
		ChanSize:      1000,
		Expiration:    time.Minute * 5,
		CleanInterval: time.Minute * 10,
	}
	for _, option := range options {
		option(opts)
	}

	cache := &AutoCache{
		opts:       opts,
		cache:      NewCache(opts.Expiration),
		loading:    make(map[string]chan struct{}),
		taskChan:   make(chan string, opts.ChanSize),
		backing:    backing,
		stopChan:   make(chan struct{}),
		loadTimer:  time.NewTimer(opts.BatchInterval),
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
			if len(batch) >= c.opts.BatchSize {
				c.loadTimer.Stop()
				c.batchLoad(batch)
				batch = nil
				c.loadTimer.Reset(c.opts.BatchInterval)
			}
		case <-c.loadTimer.C:
			if len(batch) > 0 {
				c.loadTimer.Stop()
				c.batchLoad(batch)
				batch = nil
			}
			c.loadTimer.Reset(c.opts.BatchInterval)
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
	ctx, cancel := context.WithTimeout(context.Background(), c.opts.BatchTimeout)
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

// WithBatchSize sets the batch size.
func WithBatchSize(size int) Option {
	return func(opts *Options) {
		opts.BatchSize = size
	}
}

// WithBatchInterval sets the batch interval.
func WithBatchInterval(interval time.Duration) Option {
	return func(opts *Options) {
		opts.BatchInterval = interval
	}
}

// WithBatchTimeout sets the batch timeout.
func WithBatchTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.BatchTimeout = timeout
	}
}

// WithChanSize sets the task channel size.
func WithChanSize(size int) Option {
	return func(opts *Options) {
		opts.ChanSize = size
	}
}

// WithExpiration ...
func WithExpiration(exp time.Duration) Option {
	return func(opts *Options) {
		opts.Expiration = exp
	}
}

// WithCleanInterval ...
func WithCleanInterval(itv time.Duration) Option {
	return func(opts *Options) {
		opts.CleanInterval = itv
	}
}
