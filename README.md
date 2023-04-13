# imcache

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/erni27/imcache/ci.yaml?branch=master)](https://github.com/erni27/imcache/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/erni27/imcache)](https://goreportcard.com/report/github.com/erni27/imcache)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg?style=flat-square)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/erni27/imcache)](https://pkg.go.dev/mod/github.com/erni27/imcache)
[![Coverage Status](https://codecov.io/gh/erni27/imcache/branch/master/graph/badge.svg)](https://codecov.io/gh/erni27/imcache)

`imcache` is a generic in-memory cache Go library.

It supports expiration, sliding expiration, max entries limit, eviction callbacks and sharding. It's safe for concurrent use by multiple goroutines.

```Go
import "github.com/erni27/imcache"
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/erni27/imcache"
)

func main() {
	// Zero value Cache is a valid non-sharded cache
	// with no expiration, no sliding expiration and no eviction callback.
	var c imcache.Cache[uint32, string]
	// Set a new value with no expiration time.
	c.Set(1, "one", imcache.WithNoExpiration())
	// Get the value for the key 1.
	value, ok := c.Get(1)
	if !ok {
		panic("value for the key '1' not found")
	}
	fmt.Println(value)
}
```

### Expiration

`imcache` supports no expiration, absolute expiration and sliding expiration. No expiration simply means that the entry will never expire, absolute expiration means that the entry will expire after a certain time and sliding expiration means that the entry will expire after a certain time if it hasn't been accessed. More on limiting the max number of entries in the cache can be found in the [Max entries limit](#max-entries-limit) section.

```go
// Set a new value with no expiration time.
c.Set(1, "one", imcache.WithNoExpiration())
// Set a new value with a sliding expiration time.
c.Set(2, "two", imcache.WithSlidingExpiration(time.Second))
// Set a new value with an absolute expiration time.
c.Set(3, "three", imcache.WithExpiration(time.Second))
```

If you want to use default expiration time for the given cache instance, you can use the `WithDefaultExpiration` `Expiration` option. By default the default expiration time is set to no expiration. You can set the default expiration time when creating a new `Cache` instance.

```go
// Create a new non-sharded cache instance with default absolute expiration time equal to 1 second.
c1 := imcache.New(imcache.WithDefaultExpirationOption[int32, string](time.Second))
// Set a new value with default expiration time (absolute).
c1.Set(1, "one", imcache.WithDefaultExpiration())

// Create a new non-sharded cache instance with default sliding expiration time equal to 1 second.
c2 := imcache.New(imcache.WithDefaultSlidingExpirationOption[int32, string](time.Second))
// Set a new value with default expiration time (sliding).
c2.Set(1, "one", imcache.WithDefaultExpiration())
```

### Key eviction

`imcache` follows very naive and simple eviction approach. If an expired entry is accessed by any `Cache` method, it is removed from the cache. The exception is the max entries limit. If the max entries limit is set, the cache  evicts the least recently used entry when the max entries limit is reached regardless of the expiration time.

It is possible to use the `Cleaner` to periodically remove expired entries from the cache. The `Cleaner` is a background goroutine that periodically removes expired entries from the cache. The `Cleaner` is disabled by default. You can enable it by calling the `StartCleaner` method. The `Cleaner` can be stopped by calling the `StopCleaner` method.

```go
var c imcache.Cache[string, string]
// Start Cleaner which will remove expired entries every 5 minutes.
_ = c.StartCleaner(5 * time.Minute)
defer c.StopCleaner()
```

To be notified when an entry is evicted from the cache, you can use the `EvictionCallback`. It's a function that accepts the key and value of the evicted entry along with the reason why the entry was evicted. `EvictionCallback` can be configured when creating a new `Cache` or `Sharded` instance.

```go
package main

import (
	"log"
	"time"

	"github.com/erni27/imcache"
)

func LogEvictedEntry(key string, value interface{}, reason imcache.EvictionReason) {
	log.Printf("Evicted entry: %s=%v (%s)", key, value, reason)
}

func main() {
	c := imcache.New(
		imcache.WithDefaultExpirationOption[string, interface{}](time.Second),
		imcache.WithEvictionCallbackOption(LogEvictedEntry),
	)
	c.Set("foo", "bar", imcache.WithDefaultExpiration())

	time.Sleep(time.Second)

	_, ok := c.Get("foo")
	if ok {
		panic("expected entry to be expired")
	}
}
```

### Max entries limit

`imcache` supports max entries limit. If the max entries limit is set, the cache evicts the least recently used entry when the max entries limit is reached regardless of the expiration time.

LRU eviction is implemented using a doubly linked list. The list is ordered by the time of the last access to the entry. The most recently used entry is always at the head of the list. The least recently used entry is always at the tail of the list. It means that if the max entries limit is set, `Cache` maintains another data structure in addition to the map of entries. It increases the memory usage and decreases the write performance.

The max entries limit can be configured when creating a new `Cache` instance.

```go
c := imcache.New(imcache.WithMaxEntriesOption[uint32, string](1000))
```

### Sharding

`imcache` supports sharding. It's possible to create a new cache instance with a given number of shards. Each shard is a separate `Cache` instance. A shard for a given key is selected by computing the hash of the key and taking the modulus of the number of shards. `imcache` exposes the `Hasher64` interface that wraps `Sum64` accepting a key and returning a 64-bit hash of the input key. It can be used to implement custom sharding algorithms.

Currently, `imcache` provides only one implementation of the `Hasher64` interface: `DefaultStringHasher64`. It uses the FNV-1a hash function.

A `Sharded` instance can be created by calling the `NewSharded` method. It accepts the number of shards, `Hasher64` interface and optional configuration `Option`s as arguments.

```go
c := imcache.NewSharded[string, string](4, imcache.DefaultStringHasher64{})
```

All previous examples apply to `Sharded` type as well. Note that `Option`(s) are applied to each shard not to the `Sharded` instance itself.
