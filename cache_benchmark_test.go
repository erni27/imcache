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
	c := New[string, int]()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); !ok {
			b.Fatalf("Get(_) = _, %t", ok)
		}
	}
}

func BenchmarkGet_Concurrent(b *testing.B) {
	c := New[string, int]()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); !ok {
				b.Fatalf("Get(_) = _, %t", ok)
			}
		}
	})
}

func BenchmarkGet_Sharded(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); !ok {
					b.Fatalf("Get(_) = _, %t", ok)
				}
			}
		})
	}
}

func BenchmarkGet_Sharded_Concurrent(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})
			for i := 0; i < b.N; i++ {
				c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, ok := c.Get(fmt.Sprintf("key-%d", rand.Intn(b.N))); !ok {
						b.Fatalf("Get(_) = _, %t", ok)
					}
				}
			})
		})
	}
}

func BenchmarkSet(b *testing.B) {
	c := New[string, int]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		c.Set(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}
}

func BenchmarkSet_Concurrent(b *testing.B) {
	c := New[string, int]()

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
			c := NewSharded[string, int](n, DefaultStringHasher64{})

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

var val int
var ok bool

func BenchmarkGetOrSet(b *testing.B) {
	c := New[string, int]()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i := rand.Intn(b.N)
		val, ok = c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
	}
}

func BenchmarkGetOrSet_Concurrent(b *testing.B) {
	c := New[string, int]()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(b.N)
			val, ok = c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
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
				val, ok = c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
			}
		})
	}
}

func BenchmarkGetOrSet_Sharded_Concurrent(b *testing.B) {
	for _, n := range []int{2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		b.Run(fmt.Sprintf("%d shards", n), func(b *testing.B) {
			c := NewSharded[string, int](n, DefaultStringHasher64{})

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := rand.Intn(b.N)
					val, ok = c.GetOrSet(fmt.Sprintf("key-%d", i), i, WithNoExpiration())
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
