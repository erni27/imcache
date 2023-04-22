# imcache

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/erni27/imcache/ci.yaml?branch=master)](https://github.com/erni27/imcache/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/erni27/imcache)](https://goreportcard.com/report/github.com/erni27/imcache)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg?style=flat-square)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/erni27/imcache)](https://pkg.go.dev/mod/github.com/erni27/imcache)
[![Coverage Status](https://codecov.io/gh/erni27/imcache/branch/master/graph/badge.svg)](https://codecov.io/gh/erni27/imcache)

`imcache` is a generic in-memory cache Go library.

It supports absolute expiration, sliding expiration, max entries limit, eviction callbacks and sharding. It's safe for concurrent use by multiple goroutines.

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
	// with no expiration, no sliding expiration,
	// no entry limit and no eviction callback.
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

`imcache` supports no expiration, absolute expiration and sliding expiration.

* No expiration means that the entry will never expire.
* Absolute expiration means that the entry will expire after a certain time.
* Sliding expiration means that the entry will expire after a certain time if it hasn't been accessed. The expiration time is reset every time the entry is accessed.

```go
// Set a new value with no expiration time.
// The entry will never expire.
c.Set(1, "one", imcache.WithNoExpiration())
// Set a new value with a sliding expiration time.
// If the entry hasn't been accessed in the last 1 second, it will be evicted,
// otherwise the expiration time will be extended by 1 second from the last access time.
c.Set(2, "two", imcache.WithSlidingExpiration(time.Second))
// Set a new value with an absolute expiration time set to 1 second from now.
// The entry will expire after 1 second.
c.Set(3, "three", imcache.WithExpiration(time.Second))
// Set a new value with an absolute expiration time set to a specific date.
// The entry will expire at the given date.
c.Set(4, "four", imcache.WithExpirationDate(time.Now().Add(time.Second)))
```

If you want to use default expiration time for the given cache instance, you can use the `WithDefaultExpiration` `Expiration` option. By default the default expiration time is set to no expiration. You can set the default expiration time when creating a new `Cache` or a `Sharded` instance. More on sharding can be found in the [Sharding](#sharding) section.

```go
// Create a new non-sharded cache instance with default absolute expiration time equal to 1 second.
c1 := imcache.New[int32, string](imcache.WithDefaultExpirationOption[int32, string](time.Second))
// Set a new value with default expiration time (absolute).
c1.Set(1, "one", imcache.WithDefaultExpiration())

// Create a new non-sharded cache instance with default sliding expiration time equal to 1 second.
c2 := imcache.New[int32, string](imcache.WithDefaultSlidingExpirationOption[int32, string](time.Second))
// Set a new value with default expiration time (sliding).
c2.Set(1, "one", imcache.WithDefaultExpiration())
```

### Key eviction

`imcache` follows very naive and simple eviction approach. If an expired entry is accessed by any `Cache` method, it is removed from the cache. The exception is the max entries limit. If the max entries limit is set, the cache  evicts the least recently used entry when the max entries limit is reached regardless of its expiration time. More on limiting the max number of entries in the cache can be found in the [Max entries limit](#max-entries-limit) section.

It is possible to use the `Cleaner` to periodically remove expired entries from the cache. The `Cleaner` is a background goroutine that periodically removes expired entries from the cache. The `Cleaner` is disabled by default. You can enable it by calling the `StartCleaner` method. The `Cleaner` can be stopped by calling the `StopCleaner` method.

```go
var c imcache.Cache[string, string]
// Start Cleaner which will remove expired entries every 5 minutes.
_ = c.StartCleaner(5 * time.Minute)
defer c.StopCleaner()
```

To be notified when an entry is evicted from the cache, you can use the `EvictionCallback`. It's a function that accepts the key and value of the evicted entry along with the reason why the entry was evicted. `EvictionCallback` can be configured when creating a new `Cache` or a `Sharded` instance.

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
	c := imcache.New[string, interface{}](
		imcache.WithDefaultExpirationOption[string, interface{}](time.Second),
		imcache.WithEvictionCallbackOption[string, interface{}](LogEvictedEntry),
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

`imcache` supports max entries limit. If the max entries limit is set, the cache evicts the least recently used entry when the max entries limit is reached. The least recently used entry is evicted regardless of the entry's expiration time. This allows `imcache` to remain simple and efficient.

LRU eviction is implemented using a doubly linked list. The list is ordered by the time of the last access to the entry. The most recently used entry is always at the head of the list. The least recently used entry is always at the tail of the list. It means that if the max entries limit is set, `Cache` maintains another data structure in addition to the map of entries. As a result, it increases memory usage and slightly decreases performance.

The max entries limit can be configured when creating a new `Cache` instance.

```go
c := imcache.New[uint32, string](imcache.WithMaxEntriesOption[uint32, string](1000))
```

### Sharding

`imcache` supports sharding. It's possible to create a new cache instance with a given number of shards. Each shard is a separate `Cache` instance. A shard for a given key is selected by computing the hash of the key and taking the modulus of the number of shards. `imcache` exposes the `Hasher64` interface that wraps `Sum64` accepting a key and returning a 64-bit hash of the input key. It can be used to implement custom sharding algorithms.

Currently, `imcache` provides only one implementation of the `Hasher64` interface: `DefaultStringHasher64`. It uses the FNV-1a hash function.

A `Sharded` instance can be created by calling the `NewSharded` method. It accepts the number of shards, `Hasher64` interface and optional configuration `Option`s as arguments.

```go
c := imcache.NewSharded[string, string](4, imcache.DefaultStringHasher64{})
```

All previous examples apply to `Sharded` type as well. Note that `Option`(s) are applied to each shard (`Cache` instance) not to the `Sharded` instance itself.

## Performance

`imcache` is designed to be simple and efficient. It uses a vanilla Go map to store entries and a simple and efficient implementation of double linked list to maintain LRU order. The latter is used to evict the least recently used entry when the max entries limit is reached hence if the max entries limit is not set, `imcache` doesn't maintain any additional data structures.

`imcache` was compared to the vanilla Go map with simple locking mechanism. The benchmarks were run on an Apple M1 Pro 8-core CPU with 32 GB of RAM running macOS Ventura 13.1 using Go 1.20.3.

### Reads

```bash
go version
go version go1.20.3 darwin/arm64
go test -benchmem -bench "Get_|Get$"
goos: darwin
goarch: arm64
pkg: github.com/erni27/imcache
BenchmarkCache_Get-8                                             	 3569514	       429.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/2_Shards-8                                  	 3595566	       412.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/4_Shards-8                                  	 3435393	       408.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/8_Shards-8                                  	 3601080	       414.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/16_Shards-8                                 	 3626385	       398.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/32_Shards-8                                 	 3587340	       408.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/64_Shards-8                                 	 3617484	       400.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/128_Shards-8                                	 3606388	       404.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit-8                             	 2587023	       518.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/2_Shards-8                  	 2506747	       525.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/4_Shards-8                  	 2459122	       531.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/8_Shards-8                  	 2349974	       528.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/16_Shards-8                 	 2454192	       536.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/32_Shards-8                 	 2363572	       535.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/64_Shards-8                 	 2399238	       535.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit/128_Shards-8                	 2287570	       533.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkMap_Get-8                                               	 4760186	       333.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_Parallel-8                                    	 2670980	       498.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/2_Shards-8                         	 3999897	       326.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/4_Shards-8                         	 2844760	       434.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/8_Shards-8                         	 2945050	       431.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/16_Shards-8                        	 2936168	       428.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/32_Shards-8                        	 2960804	       431.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/64_Shards-8                        	 2910768	       428.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/128_Shards-8                       	 2946024	       429.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_Parallel-8                    	 1980928	       633.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/2_Shards-8         	 2657145	       490.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/4_Shards-8         	 2472285	       516.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/8_Shards-8         	 2453889	       485.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/16_Shards-8        	 2566749	       492.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/32_Shards-8        	 2542867	       471.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/64_Shards-8        	 2599514	       486.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_Parallel/128_Shards-8       	 2509952	       470.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkMap_Get_Parallel-8                                      	 3271418	       447.2 ns/op	      23 B/op	       1 allocs/op
PASS
ok  	github.com/erni27/imcache	133.111s
```

The results are rather predictable. If data is accessed by a single goroutine, the vanilla Go map with simple locking mechanism is the fastest. `Sharded` is the fastest when data is accessed by multiple goroutines. Both `Cache` and `Sharded` are slightly slower when max entries limit is set (maintaining LRU order steals a few nanoseconds).

### Writes

```bash
go version
go version go1.20.3 darwin/arm64
go test -benchmem -bench "_Set"
goos: darwin
goarch: arm64
pkg: github.com/erni27/imcache
BenchmarkCache_Set-8                                             	 3612012	       417.0 ns/op	     188 B/op	       3 allocs/op
BenchmarkSharded_Set/2_Shards-8                                  	 3257109	       456.1 ns/op	     202 B/op	       3 allocs/op
BenchmarkSharded_Set/4_Shards-8                                  	 3197056	       457.8 ns/op	     205 B/op	       3 allocs/op
BenchmarkSharded_Set/8_Shards-8                                  	 3229351	       459.8 ns/op	     203 B/op	       3 allocs/op
BenchmarkSharded_Set/16_Shards-8                                 	 3210788	       464.8 ns/op	     204 B/op	       3 allocs/op
BenchmarkSharded_Set/32_Shards-8                                 	 3144094	       468.0 ns/op	     207 B/op	       3 allocs/op
BenchmarkSharded_Set/64_Shards-8                                 	 3139846	       468.4 ns/op	     208 B/op	       3 allocs/op
BenchmarkSharded_Set/128_Shards-8                                	 3078704	       476.0 ns/op	     211 B/op	       3 allocs/op
BenchmarkCache_Set_MaxEntriesLimit-8                             	 2845030	       469.1 ns/op	     176 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/2_Shards-8                  	 2561269	       517.0 ns/op	     183 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/4_Shards-8                  	 2495008	       527.5 ns/op	     185 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/8_Shards-8                  	 2446089	       533.3 ns/op	     187 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/16_Shards-8                 	 2399400	       542.0 ns/op	     188 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/32_Shards-8                 	 2358630	       541.0 ns/op	     190 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/64_Shards-8                 	 2346480	       551.0 ns/op	     190 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit/128_Shards-8                	 2277868	       554.1 ns/op	     193 B/op	       4 allocs/op
BenchmarkMap_Set-8                                               	 5529367	       342.1 ns/op	     113 B/op	       2 allocs/op
BenchmarkCache_Set_Parallel-8                                    	 2852869	       523.2 ns/op	     223 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/2_Shards-8                         	 2758494	       472.4 ns/op	     229 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/4_Shards-8                         	 2703622	       494.1 ns/op	     232 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/8_Shards-8                         	 2742208	       480.2 ns/op	     230 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/16_Shards-8                        	 2785494	       463.6 ns/op	     227 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/32_Shards-8                        	 2797771	       466.0 ns/op	     226 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/64_Shards-8                        	 2800551	       460.8 ns/op	     226 B/op	       3 allocs/op
BenchmarkSharded_Set_Parallel/128_Shards-8                       	 2796956	       462.2 ns/op	     226 B/op	       3 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_Parallel-8                    	 2172498	       588.4 ns/op	     197 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/2_Shards-8         	 2495745	       498.0 ns/op	     185 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/4_Shards-8         	 2388216	       527.8 ns/op	     189 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/8_Shards-8         	 2466673	       509.2 ns/op	     186 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/16_Shards-8        	 2486941	       501.3 ns/op	     185 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/32_Shards-8        	 2479155	       498.1 ns/op	     186 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/64_Shards-8        	 2478316	       495.2 ns/op	     186 B/op	       4 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_Parallel/128_Shards-8       	 2469722	       493.1 ns/op	     186 B/op	       4 allocs/op
BenchmarkMap_Set_Parallel-8                                      	 3236552	       434.0 ns/op	     100 B/op	       2 allocs/op
PASS
ok  	github.com/erni27/imcache	74.508s
```

When it comes to writes, the vanilla Go map is the fastest even if accessed by multiple goroutines. The advantage is around 30 ns/op when compared to `Sharded` and 60 ns/op when compared to `Sharded` with max entries limit set. It is caused by the fact, internally `Cache` does a few checks before writing the value into the internal map to ensure safety and proper eviction. Again both `Cache` and `Sharded` are slightly slower when max entries limit is set.
