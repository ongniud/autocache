package autocache

import (
	"time"
)

const (
	NoExpiration      time.Duration = -1
	DefaultExpiration time.Duration = 0
)

// item represents a single cache entry
type item struct {
	value      interface{}
	expiration int64 // Expiration timestamp (Unix time in nanoseconds)
}

// Cache structure using a map for storage (not concurrency-safe)
type Cache struct {
	data            map[string]item
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	cleanupTimer    *time.Timer
}

// NewCache creates a new cache instance
func NewCache(defaultExpiration time.Duration) *Cache {
	cache := &Cache{
		data:       make(map[string]item),
		defaultTTL: defaultExpiration,
	}
	return cache
}

// Set inserts a value into the cache
func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	var expireAt int64
	switch ttl {
	case NoExpiration:
		expireAt = 0
	case DefaultExpiration:
		expireAt = time.Now().Add(c.defaultTTL).UnixNano()
	default:
		expireAt = time.Now().Add(ttl).UnixNano()
	}
	c.data[key] = item{value: value, expiration: expireAt}
}

// Get retrieves a value from the cache (not concurrency-safe)
func (c *Cache) Get(key string) (interface{}, bool) {
	it, found := c.data[key]
	if !found {
		return nil, false
	}
	// Check if the item has expired
	if it.expiration > 0 && time.Now().UnixNano() > it.expiration {
		delete(c.data, key) // Remove expired entry
		return nil, false
	}
	return it.value, true
}

// Delete removes an entry from the cache
func (c *Cache) Delete(key string) {
	delete(c.data, key)
}

// Cleanup removes expired entries from the cache
func (c *Cache) Cleanup() {

	now := time.Now().UnixNano()
	for key, it := range c.data {
		if it.expiration > 0 && it.expiration <= now {
			delete(c.data, key)
		}
	}
}
