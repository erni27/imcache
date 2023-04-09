# imcache

[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/erni27/imcache/ci.yaml?branch=master)](https://github.com/erni27/imcache/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/erni27/imcache)](https://goreportcard.com/report/github.com/erni27/imcache)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg?style=flat-square)
[![GoDoc](https://pkg.go.dev/badge/mod/github.com/erni27/imcache)](https://pkg.go.dev/mod/github.com/erni27/imcache)
[![Coverage Status](https://codecov.io/gh/erni27/imcache/branch/master/graph/badge.svg)](https://codecov.io/gh/erni27/imcache)

`imcache` is a generic in-memory cache Go library.

It supports expiration, sliding expiration, eviction callbacks and sharding. It's safe for concurrent use by multiple goroutines.

```Go
import "github.com/erni27/imcache"
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/erni27/imcache"
)

func main() {
	c := imcache.New[int32, string]()
	// Add entries.
	c.Set(1, "one", imcache.WithNoExpiration())
	c.Set(2, "two", imcache.WithExpiration(2*time.Second))
	c.Set(3, "three", imcache.WithSlidingExpiration(2*time.Second))
	// Get the entries.
	got, ok := c.Get(1)
	if !ok {
		panic("key 1 not found")
	}
	fmt.Println(got)
	got, ok = c.Get(2)
	if !ok {
		panic("key 2 not found")
	}
	fmt.Println(got)
	got, ok = c.Get(3)
	if !ok {
		panic("key 3 not found")
	}
	fmt.Println(got)

	time.Sleep(1 * time.Second)
	// Get the entry with key 3 to slide its expiration.
	got, ok = c.Get(3)
	if !ok {
		panic("key 3 not found")
	}
	fmt.Println(got)
	time.Sleep(1 * time.Second)

    // Entry with key 2 should expire.
	got, ok = c.Get(1)
	if !ok {
		panic("key 1 not found")
	}
	fmt.Println(got)
	_, ok = c.Get(2)
	if ok {
		panic("key 2 found")
	}
	fmt.Println("entry with key 2 expired")
	got, ok = c.Get(3)
	if !ok {
		panic("key 3 not found")
	}
	fmt.Println(got)
}
```
