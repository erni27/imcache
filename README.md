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
