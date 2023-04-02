package imcache

import (
	"hash"
	"hash/fnv"
)

// Hasher32 is the interface that wraps the Sum32 method.
type Hasher32 interface {
	// Sum32 returns the 32-bit hash of input string.
	Sum32(string) uint32
}

// NewDefaultHasher32 returns a new Hasher32 that uses the FNV-1a algorithm.
func NewDefaultHasher32() Hasher32 {
	return hasher32{hash: fnv.New32a()}
}

type hasher32 struct {
	hash hash.Hash32
}

func (h hasher32) Sum32(s string) uint32 {
	h.hash.Reset()
	h.hash.Write([]byte(s))
	return h.hash.Sum32()
}
