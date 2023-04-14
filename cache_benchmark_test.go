package imcache

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func BenchmarkGet(b *testing.B) {
	var c Cache[string, int]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
			_ = (int)(v)
		}
	}
}

func BenchmarkGet_Parallel(b *testing.B) {
	var c Cache[string, int]
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
				_ = (int)(v)
			}
		}
	})
}

func BenchmarkGet_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
					_ = (int)(v)
				}
			}
		})
	}
}

func BenchmarkGet_Sharded_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
						_ = (int)(v)
					}
				}
			})
		})
	}
}

func BenchmarkGet_MaxEntriesLimit(b *testing.B) {
	c := New(WithMaxEntriesOption[string, int](b.N))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
			_ = (int)(v)
		}
	}
}

func BenchmarkGet_MaxEntriesLimit_Parallel(b *testing.B) {
	c := New(WithMaxEntriesOption[string, int](b.N))
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if v, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); ok {
				_ = (int)(v)
			}
		}
	})
}

func BenchmarkSet(b *testing.B) {
	var c Cache[string, int]

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", rand.Intn(b.N)), i, WithNoExpiration())
	}
}

func BenchmarkSet_Parallel(b *testing.B) {
	var c Cache[string, int]

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
		}
	})
}

func BenchmarkSet_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", rand.Intn(b.N)), i, WithNoExpiration())
			}
		})
	}
}

func BenchmarkSet_Sharded_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := rand.Intn(b.N)
					c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkSet_MaxEntriesLimit(b *testing.B) {
	c := New(WithMaxEntriesOption[string, int](b.N / 2))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", rand.Intn(b.N)), i, WithNoExpiration())
	}
}

func BenchmarkSet_MaxEntriesLimit_Parallel(b *testing.B) {
	c := New(WithMaxEntriesOption[string, int](b.N / 2))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(b.N)
			c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
		}
	})
}

func BenchmarkGetOrSet(b *testing.B) {
	c := New[string, int]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
		if ok {
			_ = (int)(v)
		}
	}
}

func BenchmarkGetOrSet_Parallel(b *testing.B) {
	c := New[string, int]()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(b.N)
			v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			if ok {
				_ = (int)(v)
			}
		}
	})
}

func BenchmarkGetOrSet_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				i := rand.Intn(b.N)
				v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
				if ok {
					_ = (int)(v)
				}
			}
		})
	}
}

func BenchmarkGetOrSet_Sharded_Parallel(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := rand.Intn(b.N)
					v, ok := c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
					if ok {
						_ = (int)(v)
					}
				}
			})
		})
	}
}

func BenchmarkReplace(b *testing.B) {
	c := New[string, int]()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		if ok := c.Replace(fmt.Sprintf("key-%d", i), i, WithNoExpiration()); !ok {
			b.Fatalf("Replace(_, _, _) = %t", ok)
		}
	}
}

func BenchmarkRemove(b *testing.B) {
	c := New[string, int]()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if ok := c.Remove(fmt.Sprintf("key-%d", i)); !ok {
			b.Fatalf("Remove(_) = %t", ok)
		}
	}
}
