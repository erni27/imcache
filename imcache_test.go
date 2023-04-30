package imcache

import (
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type imcache[K comparable, V any] interface {
	Get(key K) (v V, present bool)
	Set(key K, val V, exp Expiration)
	GetOrSet(key K, val V, exp Expiration) (v V, present bool)
	Replace(key K, val V, exp Expiration) (present bool)
	ReplaceWithFunc(key K, f func(old V) (new V), exp Expiration) (present bool)
	Remove(key K) (present bool)
	RemoveAll()
	RemoveExpired()
	GetAll() map[K]V
	Len() int
	Close()
}

func TestImcache_Get(t *testing.T) {
	tests := []struct {
		name string
		c    func() imcache[string, string]
		key  string
		want string
		ok   bool
	}{
		{
			name: "success",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name: "not found",
			c: func() imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
		},
		{
			name: "entry expired",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
		{
			name: "success - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name: "not found - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				return c
			},
			key: "foo",
		},
		{
			name: "entry expired - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			got, ok := c.Get(tt.key)
			if ok != tt.ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("Cache.Get(%s) = %v, _ want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestImcache_Get_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_Set(t *testing.T) {
	tests := []struct {
		name string
		c    func() imcache[string, string]
		key  string
		val  string
	}{
		{
			name: "add new entry",
			c: func() imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			c.Set(tt.key, tt.val, WithNoExpiration())
			got, ok := c.Get(tt.key)
			if !ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, true", tt.key, ok)
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_GetOrSet(t *testing.T) {
	tests := []struct {
		name    string
		c       func() imcache[string, string]
		key     string
		val     string
		want    string
		present bool
	}{
		{
			name: "add new entry",
			c: func() imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "get existing entry",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			want:    "foo",
			present: true,
		},
		{
			name: "add new entry - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "get existing entry - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			want:    "foo",
			present: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			got, ok := c.GetOrSet(tt.key, tt.val, WithDefaultExpiration())
			if ok != tt.present {
				t.Errorf("Cache.GetOrSet(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if got != tt.want {
				t.Errorf("Cache.GetOrSet(%s) = %v, _, want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestImcache_GetOrSet_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			<-time.After(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_Replace(t *testing.T) {
	tests := []struct {
		name    string
		c       func() imcache[string, string]
		key     string
		val     string
		present bool
	}{
		{
			name: "success",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			present: true,
		},
		{
			name: "entry expired",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry doesn't exist",
			c: func() imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "success - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			present: true,
		},
		{
			name: "entry expired - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.Replace(tt.key, tt.val, WithDefaultExpiration()); ok != tt.present {
				t.Fatalf("Cache.Replace(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
			}
			got, ok := c.Get(tt.key)
			if ok != tt.present {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if !ok {
				return
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, _, want %v, _", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_ReplaceWithFunc(t *testing.T) {
	tests := []struct {
		name    string
		c       func() imcache[string, int32]
		key     string
		f       func(int32) int32
		val     int32
		present bool
	}{
		{
			name: "success",
			c: func() imcache[string, int32] {
				c := New[string, int32]()
				c.Set("foo", 997, WithNoExpiration())
				return c
			},
			key:     "foo",
			f:       Increment[int32],
			val:     998,
			present: true,
		},
		{
			name: "entry expired",
			c: func() imcache[string, int32] {
				c := New[string, int32]()
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "entry doesn't exist",
			c: func() imcache[string, int32] {
				c := New[string, int32]()
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "success - sharded",
			c: func() imcache[string, int32] {
				c := NewSharded[string, int32](2, DefaultStringHasher64{})
				c.Set("foo", 997, WithNoExpiration())
				return c
			},
			key:     "foo",
			f:       Decrement[int32],
			val:     996,
			present: true,
		},
		{
			name: "entry expired - sharded",
			c: func() imcache[string, int32] {
				c := NewSharded[string, int32](4, DefaultStringHasher64{})
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() imcache[string, int32] {
				c := NewSharded[string, int32](8, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.ReplaceWithFunc(tt.key, tt.f, WithDefaultExpiration()); ok != tt.present {
				t.Fatalf("Cache.Replace(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
			}
			got, ok := c.Get(tt.key)
			if ok != tt.present {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, _, want %v, _", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_Remove(t *testing.T) {
	tests := []struct {
		name    string
		c       func() imcache[string, string]
		key     string
		present bool
	}{
		{
			name: "success",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			present: true,
		},
		{
			name: "entry doesn't exist",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "bar",
		},
		{
			name: "entry expired",
			c: func() imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
		{
			name: "success - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpirationDate(time.Now().Add(time.Minute)))
				return c
			},
			key:     "foo",
			present: true,
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "bar",
		},
		{
			name: "entry expired - sharded",
			c: func() imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpirationDate(time.Now().Add(time.Nanosecond)))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.Remove(tt.key); ok != tt.present {
				t.Fatalf("Cache.Remove(%s) = %t, want %t", tt.key, ok, tt.present)
			}
			if _, ok := c.Get(tt.key); ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, false", tt.key, ok)
			}
		})
	}
}

func TestImcache_RemoveAll(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_RemoveExpired(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			c.RemoveExpired()
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_GetAll(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](4, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			got := c.GetAll()
			want := map[string]string{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_GetAll_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(500*time.Millisecond))
			<-time.After(300 * time.Millisecond)
			want := map[string]string{
				"foo": "foo",
				"bar": "bar",
			}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(300 * time.Millisecond)
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(500 * time.Millisecond)
			want = map[string]string{}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_Len(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, int]
	}{
		{
			name: "not sharded",
			c:    New[string, int](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, int](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			n := 1000 + rand.Intn(1000)
			for i := 0; i < n; i++ {
				c.Set(strconv.Itoa(i), i, WithNoExpiration())
			}
			if got := c.Len(); got != n {
				t.Errorf("Cache.Len() = %d, want %d", got, n)
			}
		})
	}
}

func TestImcache_DefaultExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New(WithDefaultExpirationOption[string, string](500 * time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithDefaultExpirationOption[string, string](500*time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithDefaultExpiration())
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestCache_DefaultExpiration_LessOrEqual0(t *testing.T) {
	c := New(WithDefaultExpirationOption[string, string](0))
	if c.defaultExp != noExp {
		t.Errorf("Cache.defaultExp = %v, want %v", c.defaultExp, noExp)
	}
}

func TestImcache_DefaultSlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New(WithDefaultSlidingExpirationOption[string, string](500 * time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithDefaultSlidingExpirationOption[string, string](500*time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithDefaultExpiration())
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestCache_DefaultSlidingExpiration_LessOrEqual0(t *testing.T) {
	c := New(WithDefaultSlidingExpirationOption[string, string](0))
	if c.defaultExp != noExp {
		t.Errorf("Cache.defaultExp = %v, want %v", c.defaultExp, noExp)
	}
	if c.sliding {
		t.Errorf("Cache.sliding = %t, want %t", c.sliding, false)
	}
}

func TestImcache_Cleaner(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](20*time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](20*time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			<-time.After(30 * time.Millisecond)
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonExpired)
			}
			if evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback not called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
			<-time.After(200 * time.Millisecond)
			if !evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_Cleaner_IntervalLessOrEqual0(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](-1)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			<-time.After(200 * time.Millisecond)
			if !evictioncMock.HasNotBeenCalled() {
				t.Error("want EvictionCallback not called")
			}
		})
	}
}

func TestImcache_Close(t *testing.T) {
	tests := []struct {
		name string
		c    imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](WithCleanerOption[string, string](time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}, WithCleanerOption[string, string](time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Close()
			c.Set("foo", "foo", WithNoExpiration())
			if _, ok := c.Get("foo"); ok {
				t.Error("imcache.Get(_) = _, ok, want _, false")
			}
			v, ok := c.GetOrSet("foo", "bar", WithNoExpiration())
			if ok {
				t.Error("imcache.GetOrSet(_, _, _) = _, ok, want _, false")
			}
			if v != "" {
				t.Errorf("imcache.GetOrSet(_, _, _) = %s, _, want %s, _", v, "")
			}
			if ok := c.Replace("foo", "bar", WithNoExpiration()); ok {
				t.Error("imcache.Replace(_, _, _) = ok, want false")
			}
			if ok := c.ReplaceWithFunc("foo", func(string) string { return "bar" }, WithNoExpiration()); ok {
				t.Error("imcache.ReplaceWithFunc(_, _, _) = ok, want false")
			}
			if ok := c.Remove("foo"); ok {
				t.Error("imcache.Remove(_) = ok, want false")
			}
			if got := c.GetAll(); got != nil {
				t.Errorf("imcache.GetAll() = %v, want nil", got)
			}
			if c.Len() != 0 {
				t.Errorf("imcache.Len() = %d, want %d", c.Len(), 0)
			}
			c.RemoveAll()
			c.RemoveExpired()
			c.Close()
		})
	}
}

type evictionCallbackCall struct {
	key    string
	val    interface{}
	reason EvictionReason
}

type evictionCallbackMock struct {
	mu    sync.Mutex
	calls []evictionCallbackCall
}

func (m *evictionCallbackMock) Callback(key string, val interface{}, reason EvictionReason) {
	m.mu.Lock()
	m.calls = append(m.calls, evictionCallbackCall{key, val, reason})
	m.mu.Unlock()
}

func (m *evictionCallbackMock) HasBeenCalledWith(key string, val interface{}, reason EvictionReason) bool {
	m.mu.Lock()
	var calls []evictionCallbackCall
	for _, c := range m.calls {
		if c.key == key {
			calls = append(calls, c)
		}
	}
	m.mu.Unlock()
	for _, c := range calls {
		if c.val == val && c.reason == reason {
			return true
		}
	}
	return false
}

func (m *evictionCallbackMock) HasNotBeenCalled() bool {
	return len(m.calls) == 0
}

func (m *evictionCallbackMock) Reset() {
	m.calls = nil
}

func TestImcache_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.Set("foo", "bar", WithNoExpiration())
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			c.Set("bar", "foo", WithNoExpiration())
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_GetOrSet_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("Cache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			<-time.After(time.Nanosecond)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("Cache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.Replace("foo", "bar", WithNoExpiration()); ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if ok := c.Replace("bar", "foo", WithNoExpiration()); !ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_ReplaceWithFunc_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", 1, WithExpiration(time.Nanosecond))
			c.Set("bar", 2, WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.ReplaceWithFunc("foo", func(interface{}) interface{} { return 997 }, WithNoExpiration()); ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", 1, EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "foo", 1, EvictionReasonExpired)
			}
			if ok := c.ReplaceWithFunc("bar", func(interface{}) interface{} { return 997 }, WithNoExpiration()); !ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", 2, EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "bar", 2, EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.Remove("foo"); ok {
				t.Errorf("Cache.Remove(%s) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if ok := c.Remove("bar"); !ok {
				t.Errorf("Cache.Remove(%s) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_RemoveAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonRemoved)
			}
		})
	}
}

func TestImcache_RemoveExpired_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveExpired()
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_GetAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithSlidingExpiration(time.Second))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			got := c.GetAll()
			want := map[string]interface{}{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonExpired)
			}
		})
	}
}

func TestNewSharded_NSmallerThan0(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(-1, _) did not panic")
		}
	}()
	NewSharded[string, string](-1, DefaultStringHasher64{})
}

func TestNewSharded_NilHasher(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(_, nil) did not panic")
		}
	}()
	_ = NewSharded[string, string](2, nil)
}

func TestCache_ZeroValue(t *testing.T) {
	var c Cache[string, string]
	c.Set("foo", "bar", WithNoExpiration())
	if v, ok := c.Get("foo"); !ok || v != "bar" {
		t.Errorf("want Cache.Get(_) = %s, true, got %s, %t", "bar", v, ok)
	}
}

func TestCache_MaxEntries(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	c := New(WithEvictionCallbackOption(evictioncMock.Callback), WithMaxEntriesOption[string, interface{}](5))
	c.Set("one", 1, WithNoExpiration())
	c.Set("two", 2, WithNoExpiration())
	c.Set("three", 3, WithNoExpiration())
	c.Set("four", 4, WithExpiration(time.Nanosecond))
	c.Set("five", 5, WithNoExpiration())
	// LRU queue: five -> four -> three -> two -> one.

	// Set should evict the last entry from the queue if the size is exceeded.
	c.Set("six", 6, WithNoExpiration())
	// LRU queue: six -> five -> four -> three -> two.
	if _, ok := c.Get("one"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("one", 1, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "one", 1, EvictionReasonMaxEntriesExceeded)
	}

	// Get should move the entry to the front of the queue.
	if _, ok := c.Get("two"); !ok {
		t.Error("want Cache.Get(_) = _, true, got _, false")
	}
	// LRU queue: two -> six -> five -> four -> three.
	c.Set("seven", 7, WithNoExpiration())
	// LRU queue: seven -> two -> six -> five -> four.
	if _, ok := c.Get("three"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("three", 3, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "three", 3, EvictionReasonMaxEntriesExceeded)
	}

	// Set should evict the last entry from the queue if the size is exceeded
	// and if the entry is expired the eviction reason should be EvictionReasonExpired.
	c.Set("eight", 8, WithNoExpiration())
	// LRU queue: eight -> seven -> two -> six -> five.
	if _, ok := c.Get("four"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("four", 4, EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "four", 4, EvictionReasonExpired)
	}

	// Replace should update the entry and move it to the front of the queue.
	if ok := c.Replace("five", 5, WithNoExpiration()); !ok {
		t.Error("want Cache.Replace(_) = true, got false")
	}
	// LRU queue: five -> eight -> seven -> two -> six.
	c.Set("nine", 9, WithNoExpiration())
	// LRU queue: nine -> five -> eight -> seven -> two.
	if _, ok := c.Get("six"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("six", 6, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "six", 6, EvictionReasonMaxEntriesExceeded)
	}

	// ReplaceWithFunc should update the entry and move it to the front of the queue.
	if ok := c.ReplaceWithFunc("two", func(interface{}) interface{} { return 2 }, WithNoExpiration()); !ok {
		t.Error("want Cache.ReplaceWithFunc(_) = true, got false")
	}
	// LRU queue: two -> nine -> five -> eight -> seven.
	c.Set("ten", 10, WithNoExpiration())
	// LRU queue: ten -> two -> nine -> five -> eight.
	if _, ok := c.Get("seven"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("seven", 7, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "seven", 7, EvictionReasonMaxEntriesExceeded)
	}

	// Set should not evict any entry if the size is not exceeded.
	c.Set("ten", 10, WithNoExpiration())
	// LRU queue: ten -> two -> nine -> five -> eight.
	if _, ok := c.Get("eight"); !ok {
		t.Error("want Cache.Get(_) = _, true, got _, false")
	}
	// LRU queue: eight -> ten -> two -> nine -> five.

	// GetAll should not change the LRU queue.
	c.GetAll()
	// LRU queue: eight -> ten -> two -> nine -> five.
	c.Set("eleven", 11, WithNoExpiration())
	// LRU queue: eleven -> eight -> ten -> two -> nine.
	if _, ok := c.Get("five"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("five", 5, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "five", 5, EvictionReasonMaxEntriesExceeded)
	}

	// Remove should not mess with the LRU queue.
	if ok := c.Remove("two"); !ok {
		t.Error("want Cache.Remove(_) = true, got false")
	}
	// LRU queue: eleven -> eight -> ten -> nine.
	c.Set("twelve", 12, WithNoExpiration())
	// LRU queue: twelve -> eleven -> eight -> ten -> nine.
	if _, ok := c.Get("nine"); !ok {
		t.Error("want Cache.Get(_) = _, true, got _, false")
	}
	// LRU queue: nine -> twelve -> eleven -> eight -> ten.
	c.Set("thirteen", 13, WithNoExpiration())
	// LRU queue: thirteen -> nine -> twelve -> eleven -> eight.
	if _, ok := c.Get("ten"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	if !evictioncMock.HasBeenCalledWith("ten", 10, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "ten", 10, EvictionReasonMaxEntriesExceeded)
	}

	// RemoveAll reset the LRU queue.
	c.RemoveAll()
	// LRU queue: empty.
	c.Set("fourteen", 14, WithNoExpiration())
	c.Set("fifteen", 15, WithNoExpiration())
	c.Set("sixteen", 16, WithExpiration(time.Nanosecond))
	c.Set("seventeen", 17, WithExpiration(100*time.Millisecond))
	c.Set("eighteen", 18, WithNoExpiration())
	// LRU queue: eighteen -> seventeen -> sixteen -> fifteen -> fourteen.
	if _, ok := c.Get("fourteen"); !ok {
		t.Error("Cache.Get(_) = _, false, want _, true")
	}
	// LRU queue: fourteen -> eighteen -> seventeen -> sixteen -> fifteen.
	c.Set("nineteen", 19, WithExpiration(time.Nanosecond))
	// LRU queue: nineteen -> fourteen -> eighteen -> seventeen -> sixteen.
	if _, ok := c.Get("fifteen"); ok {
		t.Error("Cache.Get(_) = _, true, got _, false")
	}
	if !evictioncMock.HasBeenCalledWith("fifteen", 15, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "fifteen", 15, EvictionReasonMaxEntriesExceeded)
	}

	// RemoveExpired should not mess with the LRU queue.
	c.RemoveExpired()
	// LRU queue: fourteen -> eighteen -> seventeen.
	c.Set("twenty", 20, WithNoExpiration())
	// LRU queue: twenty -> fourteen -> eighteen -> seventeen.
	if _, ok := c.Get("seventeen"); !ok {
		t.Error("Cache.Get(_) = _, false, want _, true")
	}
	// LRU queue: seventeen -> twenty -> fourteen -> eighteen.
	c.Set("twentyone", 21, WithExpiration(200*time.Millisecond))
	// LRU queue: twentyone -> seventeen -> twenty -> fourteen -> eighteen.
	if _, ok := c.Get("eighteen"); !ok {
		t.Error("Cache.Get(_) = _, false, got _, true")
	}
	// LRU queue: eighteen -> twentyone -> seventeen -> twenty -> fourteen.

	// GetOrSet should cause eviction if the size is exceeded.
	if _, ok := c.GetOrSet("twentytwo", 22, WithNoExpiration()); ok {
		t.Error("Cache.GetOrSet(_, _, _) = _, true, want _, false")
	}
	// LRU queue: twentytwo -> eighteen -> twentyone -> seventeen -> twenty.
	if _, ok := c.Get("fourteen"); ok {
		t.Error("Cache.Get(_) = _, true, got _, false")
	}
	if !evictioncMock.HasBeenCalledWith("fourteen", 14, EvictionReasonMaxEntriesExceeded) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "fourteen", 14, EvictionReasonMaxEntriesExceeded)
	}

	// GetOrSet should move the entry to the front of the LRU queue.
	if _, ok := c.GetOrSet("twenty", 20, WithNoExpiration()); !ok {
		t.Error("Cache.GetOrSet(_, _, _) = _, false, want _, true")
	}
	// LRU queue: twenty -> twentytwo -> eighteen -> twentyone -> seventeen.
	// Wait until seventeen is expired.
	<-time.After(100 * time.Millisecond)
	// seventeen is expired, but it's still in the cache.
	c.Set("twentythree", 23, WithNoExpiration())
	// LRU queue: twentythree -> twenty -> twentytwo -> eighteen -> twentyone.
	if _, ok := c.Get("seventeen"); ok {
		t.Error("Cache.Get(_) = _, true, got _, false")
	}
	// seventeen should be evicted with an expired reason instead of max entries exceeded.
	if !evictioncMock.HasBeenCalledWith("seventeen", 17, EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "seventeen", 17, EvictionReasonExpired)
	}
	// Wait until twentyone is expired.
	<-time.After(100 * time.Millisecond)
	// twentyone is expired, but it's still in the cache.
	if _, ok := c.GetOrSet("twentyfour", 24, WithNoExpiration()); ok {
		t.Error("Cache.GetOrSet(_, _, _) = _, true, want _, false")
	}
	// LRU queue: twentyfour -> twentythree -> twenty -> twentytwo -> eighteen.
	if _, ok := c.Get("twentyone"); ok {
		t.Error("Cache.Get(_) = _, true, got _, false")
	}
	// twentyone should be evicted with an expired reason instead of max entries exceeded.
	if !evictioncMock.HasBeenCalledWith("twentyone", 21, EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "twentyone", 21, EvictionReasonExpired)
	}
}

func TestCache_MaxEntries_NoEvictionCallback(t *testing.T) {
	c := New(WithMaxEntriesOption[string, int](1))
	c.Set("one", 1, WithNoExpiration())
	c.Set("two", 2, WithNoExpiration())
	if _, ok := c.Get("one"); ok {
		t.Error("Cache.Get(_) = _, true, want _, false")
	}
	if _, ok := c.GetOrSet("three", 3, WithNoExpiration()); ok {
		t.Error("Cache.GetOrSet(_, _, _) = _, true, want _, false")
	}
	if _, ok := c.Get("two"); ok {
		t.Error("Cache.Get(_) = _, true, want _, false")
	}
}

func TestCache_MaxEntries_LessOrEqual0(t *testing.T) {
	c := New(WithMaxEntriesOption[string, string](0))
	if _, ok := c.queue.(*nopq[string]); !ok {
		t.Error("Cache.queue = _, want *nopq")
	}
}
