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
	c := New()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); err != nil {
			b.Fatalf("Get(_) = %v", err)
		}
	}
}

func BenchmarkGet_Concurrent(b *testing.B) {
	c := New()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); err != nil {
				b.Fatalf("Get(_) = %v", err)
			}
		}
	})
}

func BenchmarkGet_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); err != nil {
					b.Fatalf("Get(_) = %v", err)
				}
			}
		})
	}
}

func BenchmarkGet_Sharded_Concurrent(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); err != nil {
						b.Fatalf("Get(_) = %v", err)
					}
				}
			})
		})
	}
}

func BenchmarkSet(b *testing.B) {
	c := New()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}
}

func BenchmarkSet_Concurrent(b *testing.B) {
	c := New()

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
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				i := rand.Intn(b.N)
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}
		})
	}
}

func BenchmarkSet_Sharded_Concurrent(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())

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

func BenchmarkAdd(b *testing.B) {
	c := New()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		_ = c.Add(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}
}

func BenchmarkAdd_Concurrent(b *testing.B) {
	c := New()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(b.N)
			_ = c.Add(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
		}
	})
}

func BenchmarkAdd_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				i := rand.Intn(b.N)
				_ = c.Add(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}
		})
	}
}

func BenchmarkAdd_Sharded_Concurrent(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded(n, NewDefaultHasher32())

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := rand.Intn(b.N)
					_ = c.Add(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
				}
			})
		})
	}
}

func BenchmarkReplace(b *testing.B) {
	c := New()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		if err := c.Replace(fmt.Sprintf("key-%d", i), i, WithNoExpiration()); err != nil {
			b.Fatalf("Replace(_, _, _) = %v", err)
		}
	}
}

func BenchmarkRemove(b *testing.B) {
	c := New()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.Remove(fmt.Sprintf("key-%d", i)); err != nil {
			b.Fatalf("Remove(_) = %v", err)
		}
	}
}
