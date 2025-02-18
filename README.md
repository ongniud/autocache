# AutoCache

AutoCache is a high-performance, batch-based caching library in Go, designed to optimize database query efficiency by batching requests and reducing duplicate calls.

## Features
- **Automatic Batch Loading**: Aggregates multiple cache misses into a single batch query to reduce backend load.
- **Concurrency Safe**: Uses mutexes and channels to ensure thread safety.
- **Non-blocking Query**: Provides a non-blocking method `TryGet` to check for cached values without triggering backend queries.

## Installation
To use AutoCache in your Go project, simply import it:

```go
import "github.com/ongniud/autocache"
```

## Usage

### Example

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ongniud/autocache"
)

func main() {
	// Define a backing function to simulate a database query
	backing := func(ctx context.Context, keys []string) (map[string]interface{}, error) {
		values := make(map[string]interface{})
		for _, key := range keys {
			values[key] = fmt.Sprintf("value for %s", key)
		}
		return values, nil
	}

	// Create a new cache instance
	cache := autocache.NewCache(backing,
		autocache.WithBatchSize(20),
		autocache.WithBatchInterval(200*time.Millisecond),
		autocache.WithBatchTimeout(5*time.Second),
		autocache.WithChanSize(2000),
	)
	defer cache.Close()

	// Concurrently fetch values
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id)
			val, err := cache.Get(context.Background(), key)
			if err != nil {
				fmt.Printf("Error getting key %s: %v\n", key, err)
			} else {
				fmt.Printf("Value for key %s: %v\n", key, val)
			}
		}(i)
	}

	wg.Wait()
}
```

### Configuration Options
AutoCache supports the following configuration options:
- **WithBatchSize(size int)**: Sets the maximum number of keys to batch together for a single backend query.
- **WithBatchInterval(interval time.Duration)**: Sets the maximum time interval between batch queries.
- **WithBatchTimeout(timeout time.Duration)**: Sets the timeout for backend queries.
- **WithChanSize(size int)**: Sets the size of the task channel for pending keys.

### Error Handling
The library defines several error types:
- **ErrNotExist**: Indicates that the key does not exist in the cache.
- **ErrChanFull**: Indicates that the task channel is full.
- **ErrNotFound**: Indicates that the key was not found after a back - source query.

When calling the Get or TryGet methods, you should handle these errors appropriately.


## Contribution
If you find any bugs or have suggestions for improvement, please feel free to submit an issue or a pull request on the GitHub repository.

## License
This project is licensed under the MIT License. 
