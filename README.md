# imcache

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/erni27/imcache/ci.yaml?branch=master)](https://github.com/erni27/imcache/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/erni27/imcache)](https://goreportcard.com/report/github.com/erni27/imcache)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg?style=flat-square)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/erni27/imcache)](https://pkg.go.dev/mod/github.com/erni27/imcache)
[![Coverage Status](https://codecov.io/gh/erni27/imcache/branch/master/graph/badge.svg)](https://codecov.io/gh/erni27/imcache)

`imcache` is a zero-dependency generic in-memory cache Go library.

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
	c.Set(1, "one", imcache.WithNoExpiration())
	value, ok := c.Get(1)
	if !ok {
		panic("value for the key '1' not found")
	}
	fmt.Println(value)
}
```

### Expiration

`imcache` supports the following expiration options:

* `WithNoExpiration` - the entry will never expire.
* `WithExpiration` - the entry will expire after a certain time.
* `WithExpirationDate` - the entry will expire at a certain date.
* `WithSlidingExpiration` - the entry will expire after a certain time if it hasn't been accessed. The expiration time is reset every time the entry is accessed. It is slided to now + sliding expiration time when now is the time when the entry was accessed.

```go
// The entry will never expire.
c.Set(1, "one", imcache.WithNoExpiration())
// The entry will expire after 1 second.
c.Set(2, "two", imcache.WithExpiration(time.Second))
// The entry will expire at the given date.
c.Set(3, "three", imcache.WithExpirationDate(time.Now().Add(time.Second)))
// The entry will expire after 1 second if it hasn't been accessed.
// Otherwise, the expiration time will slide to the access time + 1 second.
c.Set(4, "four", imcache.WithSlidingExpiration(time.Second))
```

One can also use the `WithExpirationOption` and `WithSlidingExpirationOption` options to set the default expiration time for the given cache instance. By default, the default expiration time is set to no expiration.

```go
// Create a new cache instance with the default expiration time equal to 1 second.
c1 := imcache.New[int32, string](imcache.WithDefaultExpirationOption[int32, string](time.Second))
// The entry will expire after 1 second (the default expiration time).
c1.Set(1, "one", imcache.WithDefaultExpiration())

// Create a new cache instance with the default sliding expiration time equal to 1 second.
c2 := imcache.New[int32, string](imcache.WithDefaultSlidingExpirationOption[int32, string](time.Second))
// The entry will expire after 1 second (the default expiration time) if it hasn't been accessed.
// Otherwise, the expiration time will slide to the access time + 1 second.
c2.Set(1, "one", imcache.WithDefaultExpiration())
```

### Key eviction

`imcache` actively evicts expired entries. It removes expired entries when they are accessed by most of `Cache` methods (both read and write). `Peek`, `PeekMultiple` and `PeekAll` methods are the exception. They don't remove the expired entries and do not slide the expiration time (if the sliding expiration is set).

It is possible to use the `Cleaner` to periodically remove expired entries from the cache. The `Cleaner` is a background goroutine that periodically removes expired entries from the cache. The `Cleaner` is disabled by default. One can use the `WithCleanerOption` option to enable the `Cleaner` and set the cleaning interval.

```go
// Create a new Cache with a Cleaner which will remove expired entries every 5 minutes.
c := imcache.New[string, string](imcache.WithCleanerOption[string, string](5 * time.Minute))
// Close the Cache. This will stop the Cleaner if it is running.
defer c.Close()
```

To be notified when the entry is evicted from the cache, one can use the `EvictionCallback`. It's a function that accepts the key and the value of the evicted entry along with the reason why the entry was evicted. One can use the `WithEvictionCallbackOption` option to set the `EvictionCallback` for the given cache instance.

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

`EvictionCallback` is invoked in a separate goroutine to not block any `Cache` method.

### Max entries limit

`imcache` supports setting the max entries limit. When the max entries limit is reached, the entry is evicted according to the chosen eviction policy. `imcache` supports the following eviction policies:

* `EvictionPolicyLRU` - the least recently used entry is evicted.
* `EvictionPolicyLFU` - the least frequently used entry is evicted.
* `EvictionPolicyRandom` - a random entry is evicted.

One can use the `WithMaxEntriesLimitOption` option to set the max entries limit and the eviction policy for the given cache instance.

```go
c := imcache.New[uint32, string](imcache.WithMaxEntriesLimitOption[uint32, string](1000, imcache.EvictionPolicyLRU))
```

### Sharding

`imcache` supports sharding. Each shard is a separate `Cache` instance. A shard for a given key is selected by computing the hash of the key and taking the modulus of the number of shards. `imcache` exposes the `Hasher64` interface that wraps `Sum64` accepting a key and returning a 64-bit hash of the input key. It can be used to implement custom sharding algorithms.

A `Sharded` instance can be created by calling the `NewSharded` method.

```go
c := imcache.NewSharded[string, string](4, imcache.DefaultStringHasher64{})
```

All previous examples apply to `Sharded` type as well. Note that `Option`(s) are applied to each shard (`Cache` instance separately) not to the `Sharded` instance itself.

## Performance

`imcache` was compared to the vanilla Go map with simple locking mechanism. The benchmarks were run on an Apple M1 Pro 8-core CPU with 32 GB of RAM running macOS Ventura 13.4.1 using Go 1.21.6.

### Reads

```bash
go version
go version go1.21.6 darwin/arm64
go test -benchmem -bench "Get_|Get$|Peek_|Peek$"
goos: darwin
goarch: arm64
pkg: github.com/erni27/imcache
BenchmarkCache_Get-8                                                                 2655246	       428.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/2_Shards-8                                                      2810713	       436.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/4_Shards-8                                                      2732820	       444.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/8_Shards-8                                                      2957444	       445.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/16_Shards-8                                                     2773999	       447.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/32_Shards-8                                                     2752075	       443.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/64_Shards-8                                                     2752899	       439.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get/128_Shards-8                                                    2771691	       456.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLRU-8                               2410712	       526.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/2_Shards-8                    2346715	       543.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/4_Shards-8                    2317453	       566.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/8_Shards-8                    2293774	       556.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/16_Shards-8                   2292554	       557.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/32_Shards-8                   2262634	       542.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/64_Shards-8                   2318079	       544.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU/128_Shards-8                  2278434	       565.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLFU-8                               2482602	       528.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/2_Shards-8                    2403782	       534.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/4_Shards-8                    2286364	       548.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/8_Shards-8                    2239857	       576.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/16_Shards-8                   2198414	       556.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/32_Shards-8                   2109063	       554.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/64_Shards-8                   2251125	       550.6 ns/op	      23 B/op	       2 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU/128_Shards-8                  2172662	       551.0 ns/op	      23 B/op	       2 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyRandom-8                            2959924	       433.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/2_Shards-8                 2909020	       429.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/4_Shards-8                 2890734	       438.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/8_Shards-8                 2947471	       432.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/16_Shards-8                2937757	       456.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/32_Shards-8                2963146	       427.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/64_Shards-8                2794441	       430.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom/128_Shards-8               2965760	       466.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Peek-8                                                                3034557	       419.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/2_Shards-8                                                     2907327	       452.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/4_Shards-8                                                     2918097	       441.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/8_Shards-8                                                     2923375	       442.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/16_Shards-8                                                    2927227	       443.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/32_Shards-8                                                    2843028	       439.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/64_Shards-8                                                    2898458	       461.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek/128_Shards-8                                                   2805452	       460.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkMap_Get-8                                                                   4693522	       327.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_Parallel-8                                                        2272208	       546.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/2_Shards-8                                             3216616	       416.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/4_Shards-8                                             3898989	       327.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/8_Shards-8                                             5443684	       241.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/16_Shards-8                                            6995467	       186.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/32_Shards-8                                            8505585	       154.6 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/64_Shards-8                                            10256752	       170.9 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_Parallel/128_Shards-8                                           12003910	       113.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel-8                      1995304	       627.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/2_Shards-8           2628376	       474.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/4_Shards-8           3056476	       407.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/8_Shards-8           4281996	       304.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/16_Shards-8          5438091	       237.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/32_Shards-8          6442630	       209.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/64_Shards-8          8096386	       177.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel/128_Shards-8         9417211	       137.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel-8                      1463374	       796.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/2_Shards-8           2112945	       591.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/4_Shards-8           3008568	       412.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/8_Shards-8           4302746	       296.3 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/16_Shards-8          5671981	       231.8 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/32_Shards-8          6828380	       186.5 ns/op	      23 B/op	       2 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/64_Shards-8          7074661	       161.5 ns/op	      23 B/op	       2 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel/128_Shards-8         9294237	       139.2 ns/op	      23 B/op	       2 allocs/op
BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel-8                   1838180	       675.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/2_Shards-8        2781007	       455.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/4_Shards-8        4072056	       341.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/8_Shards-8        5529966	       227.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/16_Shards-8       7354364	       211.7 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/32_Shards-8       8624466	       138.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/64_Shards-8       11442829	       161.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel/128_Shards-8      12071902	       104.4 ns/op	      23 B/op	       1 allocs/op
BenchmarkCache_Peek_Parallel-8                                                       7957777	       155.1 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/2_Shards-8                                            7781154	       163.5 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/4_Shards-8                                           12301404	       112.0 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/8_Shards-8                                           12828216	       106.2 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/16_Shards-8                                          13837890	       94.11 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/32_Shards-8                                          14921073	       89.67 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/64_Shards-8                                          15708536	       87.81 ns/op	      23 B/op	       1 allocs/op
BenchmarkSharded_Peek_Parallel/128_Shards-8                                         16547871	       84.76 ns/op	      23 B/op	       1 allocs/op
BenchmarkMap_Get_Parallel-8                                                          3098846	       453.0 ns/op	      23 B/op	       1 allocs/op
PASS
ok  	github.com/erni27/imcache	366.377s
```

### Writes

```bash
go version
go version go1.21.6 darwin/arm64
go test -benchmem -bench "_Set"
goos: darwin
goarch: arm64
pkg: github.com/erni27/imcache
BenchmarkCache_Set-8                                                                 2942062	       461.7 ns/op	     161 B/op	       2 allocs/op
BenchmarkSharded_Set/2_Shards-8                                                      2939275	       487.5 ns/op	     162 B/op	       2 allocs/op
BenchmarkSharded_Set/4_Shards-8                                                      2827146	       497.1 ns/op	     166 B/op	       2 allocs/op
BenchmarkSharded_Set/8_Shards-8                                                      2837316	       509.1 ns/op	     165 B/op	       2 allocs/op
BenchmarkSharded_Set/16_Shards-8                                                     2818513	       495.7 ns/op	     166 B/op	       2 allocs/op
BenchmarkSharded_Set/32_Shards-8                                                     2793490	       506.3 ns/op	     167 B/op	       2 allocs/op
BenchmarkSharded_Set/64_Shards-8                                                     2815544	       499.7 ns/op	     166 B/op	       2 allocs/op
BenchmarkSharded_Set/128_Shards-8                                                    2738779	       511.2 ns/op	     170 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLRU-8                               3217950	       423.2 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/2_Shards-8                    2893335	       458.4 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/4_Shards-8                    2940762	       460.9 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/8_Shards-8                    2976267	       462.5 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/16_Shards-8                   2849967	       478.1 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/32_Shards-8                   2859116	       472.2 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/64_Shards-8                   2842370	       466.6 ns/op	      68 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU/128_Shards-8                  2863222	       481.6 ns/op	      68 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLFU-8                               3198320	       417.6 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/2_Shards-8                    2944507	       450.2 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/4_Shards-8                    2915707	       456.9 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/8_Shards-8                    2820458	       464.4 ns/op	      68 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/16_Shards-8                   2833208	       479.6 ns/op	      68 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/32_Shards-8                   2864629	       460.9 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/64_Shards-8                   2846596	       491.7 ns/op	      68 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU/128_Shards-8                  2841228	       493.6 ns/op	      68 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyRandom-8                            2988330	       459.8 ns/op	      59 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/2_Shards-8                 2713388	       495.6 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/4_Shards-8                 2756364	       493.7 ns/op	      58 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/8_Shards-8                 2691565	       484.5 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/16_Shards-8                2710840	       479.6 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/32_Shards-8                2728695	       496.0 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/64_Shards-8                2690887	       507.6 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom/128_Shards-8               2663871	       495.1 ns/op	      57 B/op	       2 allocs/op
BenchmarkMap_Set-8                                                                   5847526	       339.0 ns/op	     106 B/op	       2 allocs/op
BenchmarkCache_Set_Parallel-8                                                        2347249	       527.7 ns/op	     123 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/2_Shards-8                                             3112208	       444.3 ns/op	     156 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/4_Shards-8                                             3323001	       401.2 ns/op	     149 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/8_Shards-8                                             4130948	       291.3 ns/op	     131 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/16_Shards-8                                            5516203	       274.4 ns/op	     169 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/32_Shards-8                                            6069385	       219.0 ns/op	     158 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/64_Shards-8                                            7290878	       187.9 ns/op	     141 B/op	       2 allocs/op
BenchmarkSharded_Set_Parallel/128_Shards-8                                           8021259	       163.6 ns/op	     133 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel-8                      2402929	       555.8 ns/op	      66 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/2_Shards-8           2835458	       418.2 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/4_Shards-8           3219746	       393.8 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/8_Shards-8           4225972	       301.9 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/16_Shards-8          5186444	       246.7 ns/op	      67 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/32_Shards-8          6100933	       222.6 ns/op	      70 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/64_Shards-8          7518198	       180.7 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel/128_Shards-8         8257630	       180.1 ns/op	      65 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel-8                      2344273	       543.1 ns/op	      66 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/2_Shards-8           2973801	       442.6 ns/op	      69 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/4_Shards-8           3416180	       383.2 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/8_Shards-8           4511509	       293.7 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/16_Shards-8          5374215	       230.7 ns/op	      68 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/32_Shards-8          6602998	       205.1 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/64_Shards-8          7730086	       171.8 ns/op	      65 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel/128_Shards-8         8232051	       154.6 ns/op	      65 B/op	       2 allocs/op
BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel-8                   2186001	       592.0 ns/op	      55 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/2_Shards-8        2716564	       438.3 ns/op	      57 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/4_Shards-8        3548733	       366.0 ns/op	      55 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/8_Shards-8        4665452	       296.7 ns/op	      55 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/16_Shards-8       6088111	       240.0 ns/op	      59 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/32_Shards-8       7442355	       197.1 ns/op	      55 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/64_Shards-8       8089219	       163.0 ns/op	      55 B/op	       2 allocs/op
BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel/128_Shards-8      9815180	       144.8 ns/op	      56 B/op	       2 allocs/op
BenchmarkMap_Set_Parallel-8                                                          3541270	       407.3 ns/op	      92 B/op	       2 allocs/op
PASS
ok  	github.com/erni27/imcache	135.166s
```
