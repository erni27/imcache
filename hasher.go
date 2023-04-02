package imcache

import (
	"hash/fnv"
)

// Hasher32 is the interface that wraps the Sum32 method.
type Hasher32 interface {
	// Sum32 returns the 32-bit hash of input string.
	Sum32(string) uint32
}

// NewDefaultHasher32 returns a new Hasher32 that uses the FNV-1a algorithm.
func NewDefaultHasher32() Hasher32 {
	return fnv32a{}
}

type fnv32a struct{}

func (fnv32a) Sum32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
