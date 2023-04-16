package imcache

import (
	"hash/fnv"
)

// Hasher64 is the interface that wraps the Sum64 method.
type Hasher64[K comparable] interface {
	// Sum64 returns the 64-bit hash of the input key.
	Sum64(K) uint64
}

// DefaultStringHasher64 is the default hasher for string keys.
// It uses the FNV-1a hash function.
type DefaultStringHasher64 struct{}

// Sum64 returns the 64-bit hash of the input key.
func (DefaultStringHasher64) Sum64(key string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key))
	return hash.Sum64()
}
