package autocache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestCache_Get tests cache retrieval and batch loading.
func TestCache_Get(t *testing.T) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(5),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(1*time.Second),
		WithLoadBuffSize(100),
	)
	defer cache.Close()

	key := "testKey"
	val, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "value_testKey" {
		t.Errorf("expected value_testKey, got %v", val)
	}
}

// TestCache_TryGet tests non-blocking retrieval.
func TestCache_TryGet(t *testing.T) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(5),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(1*time.Second),
		WithLoadBuffSize(100),
	)
	defer cache.Close()

	_, err := cache.TryGet("missingKey")
	if err != ErrNotExist {
		t.Errorf("expected ErrNotExist, got %v", err)
	}

	time.Sleep(time.Millisecond * 200)
	val, _ := cache.TryGet("missingKey")
	if val != "value_missingKey" {
		t.Errorf("expected value_testKey, got %v", val)
	}
}

// TestCache_Concurrency ensures the cache handles concurrent access correctly.
func TestCache_Concurrency(t *testing.T) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(10),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(1*time.Second),
		WithLoadBuffSize(100),
	)
	defer cache.Close()

	var wg sync.WaitGroup
	n := 20
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		keys[i] = "key" + strconv.Itoa(i)
	}

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			val, err := cache.Get(context.Background(), k)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if val != "value_"+k {
				t.Errorf("expected value_%s, got %v", k, val)
			}
		}(key)
	}

	wg.Wait()
}

// TestCache_Timeout ensures the cache correctly handles batch timeouts.
func TestCache_Timeout(t *testing.T) {
	cache := NewAutoCache(
		func(ctx context.Context, keys []string) (map[string]interface{}, error) {
			time.Sleep(2 * time.Second) // Simulate long backend response
			return nil, errors.New("timeout")
		},
		WithLoadBatchSize(5),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(500*time.Millisecond), // Shorter timeout
		WithLoadBuffSize(100),
	)
	defer cache.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := cache.Get(ctx, "slowKey")
	if err == nil || err.Error() != "context deadline exceeded" {
		t.Errorf("expected timeout error, got %v", err)
	}
}

// TestCache_ChanFull ensures cache handles full task channel correctly.
func TestCache_ChanFull(t *testing.T) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(5),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(1*time.Second),
		WithLoadBuffSize(1), // Very small channel size
	)
	defer cache.Close()

	cache.buffer <- "preloadedKey"
	_, err := cache.Get(context.Background(), "extraKey")
	if err != ErrChanFull {
		t.Errorf("expected ErrChanFull, got %v", err)
	}
}

// BenchmarkCache_SingleThread tests the cache performance with a single-threaded access pattern
func BenchmarkCache_SingleThread(b *testing.B) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(10),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(3*time.Second),
		WithLoadBuffSize(100),
	)
	defer cache.Close()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%10) // Reuse keys to test cache hits
		_, _ = cache.Get(ctx, key)
	}
}

// BenchmarkCache_Concurrent tests the cache performance under concurrent load
func BenchmarkCache_Concurrent(b *testing.B) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(10),
		WithLoadInterval(100*time.Millisecond),
		WithLoadTimeout(3*time.Second),
		WithLoadBuffSize(1000),
	)
	defer cache.Close()

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			key := fmt.Sprintf("key%d", time.Now().UnixNano()%100) // Randomized keys
			_, _ = cache.Get(ctx, key)
		}
	})
}

// BenchmarkCache_StressTest simulates a high-load environment
func BenchmarkCache_StressTest(b *testing.B) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(50),
		WithLoadInterval(50*time.Millisecond),
		WithLoadTimeout(2*time.Second),
		WithLoadBuffSize(5000),
	)
	defer cache.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ { // 100 concurrent goroutines
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			for j := 0; j < b.N/100; j++ {
				key := fmt.Sprintf("key%d", j%500) // 500 different keys
				_, _ = cache.Get(ctx, key)
			}
		}(i)
	}
	wg.Wait()
}

// BenchmarkCache_BatchEfficiency measures batch processing effectiveness
func BenchmarkCache_BatchEfficiency(b *testing.B) {
	cache := NewAutoCache(MockBackingFunc,
		WithLoadBatchSize(10),
		WithLoadInterval(10*time.Millisecond),
		WithLoadTimeout(1*time.Second),
		WithLoadBuffSize(1000),
	)
	defer cache.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(context.Background(), fmt.Sprintf("key%d", i%10)) // Reuse 10 keys to maximize batching
	}
}

// MockBackingFunc simulates a backend data source.
func MockBackingFunc(ctx context.Context, keys []string) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	for _, key := range keys {
		values[key] = "value_" + key
	}
	return values, nil
}
