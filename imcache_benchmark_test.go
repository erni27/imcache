package imcache

import (
	"fmt"
	"sync"
	"testing"
)

type token struct {
	ID int `json:"id"`
}

func BenchmarkCache_Get(b *testing.B) {
	var c Cache[string, token]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
			_ = v
		}
	}
}

func BenchmarkSharded_Get(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
					_ = v
				}
			}
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLRU(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyLRU))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
			_ = v
		}
	}
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyLRU))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
					_ = v
				}
			}
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLFU(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyLFU))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
			_ = v
		}
	}
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyLFU))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
					_ = v
				}
			}
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyRandom(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyRandom))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
			_ = v
		}
	}
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyRandom))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
					_ = v
				}
			}
		})
	}
}

func BenchmarkMap_Get(b *testing.B) {
	m := make(map[string]token)
	var mu sync.Mutex
	for i := 0; i < b.N; i++ {
		m[fmt.Sprintf("key-%d", i)] = token{ID: i}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		v, ok := m[fmt.Sprintf("key-%d", random.Intn(b.N))]
		if ok {
			_ = (token)(v)
		}
		mu.Unlock()
	}
}

func BenchmarkCache_Get_Parallel(b *testing.B) {
	var c Cache[string, token]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
				_ = (token)(v)
			}
		}
	})
}

func BenchmarkSharded_Get_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
						_ = (token)(v)
					}
				}
			})
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyLRU))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
				_ = (token)(v)
			}
		}
	})
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLRU_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyLRU))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
						_ = (token)(v)
					}
				}
			})
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyLFU))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
				_ = (token)(v)
			}
		}
	})
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyLFU_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyLFU))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
						_ = (token)(v)
					}
				}
			})
		})
	}
}

func BenchmarkCache_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N, EvictionPolicyRandom))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
				_ = (token)(v)
			}
		}
	})
}

func BenchmarkSharded_Get_MaxEntriesLimit_EvictionPolicyRandom_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n, EvictionPolicyRandom))
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if v, ok := c.Get(fmt.Sprintf("key-%d", random.Intn(b.N))); ok {
						_ = (token)(v)
					}
				}
			})
		})
	}
}

func BenchmarkMap_Get_Parallel(b *testing.B) {
	m := make(map[string]token)
	var mu sync.Mutex
	for i := 0; i < b.N; i++ {
		m[fmt.Sprintf("key-%d", i)] = token{ID: i}
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			v, ok := m[fmt.Sprintf("key-%d", random.Intn(b.N))]
			if ok {
				_ = (token)(v)
			}
			mu.Unlock()
		}
	})
}

func BenchmarkCache_Set(b *testing.B) {
	var c Cache[string, token]

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
	}
}

func BenchmarkSharded_Set(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
			}
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLRU(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyLRU))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
	}
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyLRU))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
			}
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLFU(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyLFU))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
	}
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyLFU))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
			}
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyRandom(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyRandom))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
	}
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyRandom))

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", random.Intn(b.N)), token{ID: i}, WithNoExpiration())
			}
		})
	}
}

func BenchmarkMap_Set(b *testing.B) {
	m := make(map[string]token)
	var mu sync.Mutex

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m[fmt.Sprintf("key-%d", random.Intn(b.N))] = token{ID: i}
		mu.Unlock()
	}
}

func BenchmarkCache_Set_Parallel(b *testing.B) {
	var c Cache[string, token]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
		}
	})
}

func BenchmarkSharded_Set_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := random.Intn(b.N)
					c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyLRU))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
		}
	})
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLRU_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyLRU))

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := random.Intn(b.N)
					c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyLFU))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
		}
	})
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyLFU_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyLFU))

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := random.Intn(b.N)
					c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkCache_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel(b *testing.B) {
	c := New[string, token](WithMaxEntriesLimitOption[string, token](b.N/2, EvictionPolicyRandom))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
		}
	})
}

func BenchmarkSharded_Set_MaxEntriesLimit_EvictionPolicyRandom_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{}, WithMaxEntriesLimitOption[string, token](b.N/n/2, EvictionPolicyRandom))

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := random.Intn(b.N)
					c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkMap_Set_Parallel(b *testing.B) {
	m := make(map[string]token)
	var mu sync.Mutex

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			mu.Lock()
			m[fmt.Sprintf("key-%d", i)] = token{ID: i}
			mu.Unlock()
		}
	})
}

func BenchmarkCache_GetOrSet(b *testing.B) {
	var c Cache[string, token]

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := random.Intn(b.N)
		v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
		if ok {
			_ = (token)(v)
		}
	}
}

func BenchmarkSharded_GetOrSet(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				i := random.Intn(b.N)
				v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
				if ok {
					_ = (token)(v)
				}
			}
		})
	}
}

func BenchmarkCache_GetOrSet_Parallel(b *testing.B) {
	var c Cache[string, token]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := random.Intn(b.N)
			v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
			if ok {
				_ = (token)(v)
			}
		}
	})
}

func Benchmark_Sharded_GetOrSet_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d_Shards", n), func(b *testing.B) {
			c := NewSharded[string, token](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := random.Intn(b.N)
					v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
					if ok {
						_ = (token)(v)
					}
				}
			})
		})
	}
}

func BenchmarkCache_Replace(b *testing.B) {
	var c Cache[string, token]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := random.Intn(b.N)
		if ok := c.Replace(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration()); !ok {
			b.Fatalf("Replace(_, _, _) = %t", ok)
		}
	}
}

func BenchmarkCache_Remove(b *testing.B) {
	var c Cache[string, token]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), token{ID: i}, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ok := c.Remove(fmt.Sprintf("key-%d", i)); !ok {
			b.Fatalf("Remove(_) = %t", ok)
		}
	}
}
