package autocache

import (
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	cache := NewCache(2 * time.Second)

	cache.Set("key1", "value1", DefaultExpiration)
	value, found := cache.Get("key1")
	if !found {
		t.Errorf("Expected to find key1 in the cache, but it was not found.")
	}
	if value != "value1" {
		t.Errorf("Expected value1, but got %v", value)
	}
}

func TestCache_Expiration(t *testing.T) {
	cache := NewCache(1 * time.Second)

	cache.Set("key1", "value1", DefaultExpiration)
	time.Sleep(2 * time.Second) // Wait for expiration

	_, found := cache.Get("key1")
	if found {
		t.Errorf("Expected key1 to be expired, but it was found in the cache.")
	}
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(2 * time.Second)

	cache.Set("key1", "value1", DefaultExpiration)
	cache.Delete("key1")

	_, found := cache.Get("key1")
	if found {
		t.Errorf("Expected key1 to be deleted, but it was found in the cache.")
	}
}

func TestCache_NoExpiration(t *testing.T) {
	cache := NewCache(NoExpiration)

	cache.Set("key1", "value1", NoExpiration)
	time.Sleep(2 * time.Second) // Wait for more than the default TTL

	value, found := cache.Get("key1")
	if !found {
		t.Errorf("Expected to find key1 in the cache, but it was not found.")
	}
	if value != "value1" {
		t.Errorf("Expected value1, but got %v", value)
	}
}

func TestCache_Cleanup(t *testing.T) {
	cache := NewCache(1 * time.Second)

	cache.Set("key1", "value1", DefaultExpiration)
	time.Sleep(2 * time.Second) // Allow cleanup to run
	cache.Cleanup()
	_, found := cache.Get("key1")
	if found {
		t.Errorf("Expected key1 to be expired and cleaned up, but it was found in the cache.")
	}
}
