package imcache

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

type imcache[K comparable, V any] interface {
	Get(key K) (v V, present bool)
	GetMultiple(keys ...K) map[K]V
	GetAll() map[K]V
	Peek(key K) (v V, present bool)
	PeekMultiple(keys ...K) map[K]V
	PeekAll() map[K]V
	Set(key K, val V, exp Expiration)
	GetOrSet(key K, val V, exp Expiration) (v V, present bool)
	Replace(key K, val V, exp Expiration) (present bool)
	ReplaceWithFunc(key K, f func(old V) (new V), exp Expiration) (present bool)
	ReplaceKey(oldKey, newKey K, exp Expiration) (present bool)
	CompareAndSwap(key K, expected, new V, compare func(V, V) bool, exp Expiration) (swapped, present bool)
	Remove(key K) (present bool)
	RemoveAll()
	RemoveExpired()
	Len() int
	Close()
}

// caches is a list of string-string caches with default configuration to test.
// If a test needs different type of cache or configured one, it should be
// created within the test.
var caches = []struct {
	create func() imcache[string, string]
	name   string
}{
	{
		name: "Cache",
		create: func() imcache[string, string] {
			// Randomly test both zero value and initialized Cache.
			if rand.Intn(2) == 1 {
				return New[string, string]()
			}
			return &Cache[string, string]{}
		},
	},
	{
		name: "Sharded",
		create: func() imcache[string, string] {
			// Randomly test different number of shards.
			shards := rand.Intn(10) + 1
			return NewSharded[string, string](shards, DefaultStringHasher64{})
		},
	},
}

func TestImcache_Get(t *testing.T) {
	tests := []struct {
		name  string
		setup func(imcache[string, string])
		key   string
		want  string
		ok    bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name:  "not found",
			setup: func(_ imcache[string, string]) {},
			key:   "foo",
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got, ok := c.Get(tt.key); ok != tt.ok || got != tt.want {
					t.Errorf("imcache.Get(%s) = %v, %t want %v, %t", tt.key, got, ok, tt.want, tt.ok)
				}
			})
		}
	}
}

func TestImcache_Get_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_GetMultiple(t *testing.T) {
	tests := []struct {
		name  string
		setup func(imcache[string, string])
		keys  []string
		want  map[string]string
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foobar", "foobar", WithNoExpiration())
				c.Set("barfoo", "barfoo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			keys: []string{"foo", "bar", "foobar", "barfoo"},
			want: map[string]string{
				"foo":    "bar",
				"bar":    "foo",
				"foobar": "foobar",
			},
		},
		{
			name: "not found",
			setup: func(c imcache[string, string]) {
				c.Set("foobar", "foobar", WithNoExpiration())
			},
			keys: []string{"foo", "bar"},
			want: map[string]string{},
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				c.Set("bar", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			keys: []string{"foo", "bar"},
			want: map[string]string{},
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got := c.GetMultiple(tt.keys...); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("imcache.GetMultiple(%v) = %v want %v", tt.keys, got, tt.want)
				}
			})
		}
	}
}

func TestImcache_GetMultiple_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(500*time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(500*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			want := map[string]string{
				"foo":    "foo",
				"bar":    "bar",
				"foobar": "foobar",
			}
			if got := c.GetMultiple("foo", "bar", "foobar"); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetMultiple(_) = %v, want %v", got, want)
			}
			time.Sleep(300 * time.Millisecond)
			want = map[string]string{
				"foo": "foo",
				"bar": "bar",
			}
			if got := c.GetMultiple("foo", "bar", "foobar"); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetMultiple(_) = %v, want %v", got, want)
			}
			time.Sleep(500 * time.Millisecond)
			if got := c.GetMultiple("foo", "bar", "foobar"); got == nil || len(got) != 0 {
				t.Errorf("imcache.GetMultiple(_) = %v, want empty", got)
			}
		})
	}
}

func TestImcache_Peek(t *testing.T) {
	tests := []struct {
		name  string
		setup func(imcache[string, string])
		key   string
		want  string
		ok    bool
	}{
		{
			name: "exists",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name:  "not found",
			setup: func(_ imcache[string, string]) {},
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got, ok := c.Peek(tt.key); ok != tt.ok || got != tt.want {
					t.Errorf("imcache.Peek(%s) = %v, %t want %v, %t", tt.key, got, ok, tt.want, tt.ok)
				}
			})
		}
	}
}

func TestImcache_Peek_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Peek("foo"); !ok {
				t.Fatal("got imcache.Peek(_) = _, false, want _, true")
			}
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Peek("foo"); ok {
				t.Fatal("got imcache.Peek(_) = _, true, want _, false")
			}
		})
	}
}

func TestImcache_PeekMultiple(t *testing.T) {
	tests := []struct {
		name  string
		setup func(imcache[string, string])
		keys  []string
		want  map[string]string
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foobar", "foobar", WithNoExpiration())
				c.Set("barfoo", "barfoo", WithNoExpiration())
			},
			keys: []string{"foo", "bar", "foobar", "barfoo"},
			want: map[string]string{
				"foo":    "bar",
				"bar":    "foo",
				"foobar": "foobar",
				"barfoo": "barfoo",
			},
		},
		{
			name: "not found",
			setup: func(c imcache[string, string]) {
				c.Set("foobar", "foobar", WithNoExpiration())
			},
			keys: []string{"foo", "bar"},
			want: map[string]string{},
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				c.Set("bar", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			keys: []string{"foo", "bar"},
			want: map[string]string{},
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got := c.PeekMultiple(tt.keys...); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("got imcache.PeekMultiple(%v) = %v want %v", tt.keys, got, tt.want)
				}
			})
		}
	}
}

func TestImcache_PeekMultiple_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(400*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			want := map[string]string{"foo": "foo", "bar": "bar"}
			if got := c.PeekMultiple("foo", "bar"); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekMultiple(_) = %v, want %v", got, want)
			}
			time.Sleep(300 * time.Millisecond)
			want = make(map[string]string)
			if got := c.PeekMultiple("foo", "bar"); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekMultiple(_) = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_PeekAll(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithNoExpiration())
			c.Set("barfoo", "barfoo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			want := map[string]string{
				"foo":    "foo",
				"foobar": "foobar",
				"barfoo": "barfoo",
			}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("got imcache.PeekAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_PeekAll_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(400*time.Millisecond))
			c.Set("foobar", "foobar", WithNoExpiration())
			time.Sleep(300 * time.Millisecond)
			want := map[string]string{"foo": "foo", "bar": "bar", "foobar": "foobar"}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekAll() = %v, want %v", got, want)
			}
			time.Sleep(300 * time.Millisecond)
			want = map[string]string{"foobar": "foobar"}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_Set(t *testing.T) {
	tests := []struct {
		name  string
		setup func(imcache[string, string])
		key   string
		val   string
	}{
		{
			name:  "add new entry",
			setup: func(imcache[string, string]) {},
			key:   "foo",
			val:   "bar",
		},
		{
			name: "replace existing entry",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpirationDate(time.Now().Add(time.Nanosecond)))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
			val: "bar",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				c.Set(tt.key, tt.val, WithNoExpiration())
				if got, ok := c.Get(tt.key); !ok || got != tt.val {
					t.Errorf("imcache.Get(%s) = %v, %t want %v, true", tt.key, got, ok, tt.val)
				}
			})
		}
	}
}

func TestImcache_GetOrSet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(imcache[string, string])
		key     string
		val     string
		want    string
		present bool
	}{
		{
			name:  "add new entry",
			setup: func(c imcache[string, string]) {},
			key:   "foo",
			val:   "bar",
			want:  "bar",
		},
		{
			name: "add new entry if old expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "get existing entry",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key:     "foo",
			val:     "bar",
			want:    "foo",
			present: true,
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got, ok := c.GetOrSet(tt.key, tt.val, WithDefaultExpiration()); ok != tt.present || got != tt.want {
					t.Errorf("imcache.GetOrSet(%s, %s, _) = %v, %t want %v, %t", tt.key, tt.val, got, ok, tt.want, tt.present)
				}
			})
		}
	}
}

func TestImcache_GetOrSet_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(500 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_Replace(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(imcache[string, string])
		key     string
		val     string
		want    string
		present bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key:     "foo",
			val:     "bar",
			want:    "bar",
			present: true,
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
			val: "bar",
		},
		{
			name:  "entry doesn't exist",
			setup: func(c imcache[string, string]) {},
			key:   "foo",
			val:   "bar",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if ok := c.Replace(tt.key, tt.val, WithDefaultExpiration()); ok != tt.present {
					t.Fatalf("imcache.Replace(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
				}

				if got, ok := c.Get(tt.key); ok != tt.present || got != tt.want {
					t.Errorf("imcache.Get(%s) = %v, %t, want %v, %t", tt.key, got, ok, tt.want, tt.present)
				}
			})
		}
	}
}

func TestImcache_ReplaceWithFunc(t *testing.T) {
	increment := func(old int32) int32 {
		return old + 1
	}
	caches := []struct {
		create func() imcache[string, int32]
		name   string
	}{
		{
			name:   "Cache",
			create: func() imcache[string, int32] { return &Cache[string, int32]{} },
		},
		{
			name:   "Sharded",
			create: func() imcache[string, int32] { return NewSharded[string, int32](8, DefaultStringHasher64{}) },
		},
	}
	tests := []struct {
		setup   func(imcache[string, int32])
		f       func(int32) int32
		name    string
		key     string
		val     int32
		present bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, int32]) {
				c.Set("foo", 997, WithNoExpiration())
			},
			key:     "foo",
			f:       increment,
			val:     998,
			present: true,
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, int32]) {
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
			f:   increment,
		},
		{
			name:  "entry doesn't exist",
			setup: func(c imcache[string, int32]) {},
			key:   "foo",
			f:     increment,
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if ok := c.ReplaceWithFunc(tt.key, tt.f, WithDefaultExpiration()); ok != tt.present {
					t.Fatalf("imcache.ReplaceWithFunc(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
				}

				if got, ok := c.Get(tt.key); ok != tt.present || got != tt.val {
					t.Errorf("imcache.Get(%s) = %v, %t, want %v, %t", tt.key, got, ok, tt.val, tt.present)
				}
			})
		}
	}
}

func TestImcache_ReplaceKey(t *testing.T) {
	tests := []struct {
		setup func(imcache[string, string])
		name  string
		old   string
		new   string
		val   string
		want  bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			old:  "foo",
			new:  "bar",
			want: true,
			val:  "foo",
		},
		{
			name:  "key doesn't exist",
			setup: func(_ imcache[string, string]) {},
			old:   "foo",
			new:   "bar",
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			old: "foo",
			new: "bar",
		},
		{
			name: "new key already exists",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
				c.Set("bar", "bar", WithNoExpiration())
			},
			old:  "foo",
			new:  "bar",
			want: true,
			val:  "foo",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if got := c.ReplaceKey(tt.old, tt.new, WithNoExpiration()); got != tt.want {
					t.Errorf("imcache.ReplaceKey(%s, %s, _) = %t, want %t", tt.old, tt.new, got, tt.want)
				}
				if !tt.want {
					return
				}
				if _, ok := c.Get(tt.old); ok {
					t.Errorf("imcache.Get(%s) = _, %t, want _, %t", tt.old, ok, false)
				}
				if got, ok := c.Get(tt.new); !ok || got != tt.val {
					t.Errorf("imcache.Get(%s) = %v, %t, want %v, %t", tt.new, got, ok, tt.val, true)
				}
			})
		}
	}
}

func TestImcache_CompareAndSwap(t *testing.T) {
	tests := []struct {
		setup    func(imcache[string, string])
		name     string
		key      string
		expected string
		new      string
		compare  func(string, string) bool
		swapped  bool
		present  bool
	}{
		{
			name: "swapped",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key:      "foo",
			expected: "foo",
			new:      "bar",
			compare:  func(x, y string) bool { return x == y },
			swapped:  true,
			present:  true,
		},
		{
			name: "not swapped",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key:      "foo",
			expected: "foo",
			new:      "bar",
			compare:  func(_, _ string) bool { return false },
			present:  true,
		},
		{
			name:     "key doesn't exist",
			setup:    func(_ imcache[string, string]) {},
			key:      "foo",
			expected: "foo",
			new:      "bar",
			compare:  func(_, _ string) bool { return true },
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key:      "foo",
			expected: "foo",
			new:      "bar",
			compare:  func(_, _ string) bool { return true },
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if swapped, present := c.CompareAndSwap(tt.key, tt.expected, tt.new, tt.compare, WithNoExpiration()); swapped != tt.swapped || present != tt.present {
					t.Errorf("imcache.CompareAndSwap(%s, %s, %s, _, _) = %t, %t, want %t, %t", tt.key, tt.expected, tt.new, swapped, present, tt.swapped, tt.present)
				}
				if !tt.swapped {
					return
				}
				val, ok := c.Get(tt.key)
				if !ok || val != tt.new {
					t.Errorf("imcache.Get(%s) = %s, %t, want %s, %t", tt.key, val, ok, tt.new, true)
				}
			})
		}
	}
}

func TestSharded_ReplaceKey_SameShard(t *testing.T) {
	s := NewSharded[string, string](2, DefaultStringHasher64{})
	s.Set("1", "foo", WithNoExpiration())
	if ok := s.ReplaceKey("1", "3", WithNoExpiration()); !ok {
		t.Errorf("Sharded.ReplaceKey(1, 3, _) = %t, want %t", ok, true)
	}
	if _, ok := s.Get("1"); ok {
		t.Errorf("Sharded.Get(1) = _, %t, want _, %t", ok, false)
	}
	if got, ok := s.Get("3"); !ok || got != "foo" {
		t.Errorf("Sharded.Get(3) = %v, %t, want %v, %t", got, ok, "foo", true)
	}
}

func TestImcache_Remove(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(imcache[string, string])
		key     string
		present bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key:     "foo",
			present: true,
		},
		{
			name: "entry doesn't exist",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithNoExpiration())
			},
			key: "bar",
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, string]) {
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				time.Sleep(time.Nanosecond)
			},
			key: "foo",
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				c := cache.create()
				tt.setup(c)
				if ok := c.Remove(tt.key); ok != tt.present {
					t.Fatalf("imcache.Remove(%s) = %t, want %t", tt.key, ok, tt.present)
				}
				if _, ok := c.Get(tt.key); ok {
					t.Fatalf("imcache.Get(%s) = _, %t, want _, false", tt.key, ok)
				}
			})
		}
	}
}

func TestImcache_RemoveAll(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			c.RemoveAll()
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_RemoveExpired(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			c.RemoveExpired()
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_GetAll(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			got := c.GetAll()
			want := map[string]string{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_GetAll_SlidingExpiration(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(500*time.Millisecond))
			time.Sleep(300 * time.Millisecond)
			want := map[string]string{
				"foo": "foo",
				"bar": "bar",
			}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
			time.Sleep(300 * time.Millisecond)
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
			time.Sleep(500 * time.Millisecond)
			want = map[string]string{}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_Len(t *testing.T) {
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			n := 1000 + rand.Intn(1000)
			for i := 0; i < n; i++ {
				c.Set(strconv.Itoa(i), fmt.Sprintf("test-%d", i), WithNoExpiration())
			}
			if got := c.Len(); got != n {
				t.Errorf("imcache.Len() = %d, want %d", got, n)
			}
		})
	}
}

func TestImcache_DefaultExpiration(t *testing.T) {
	caches := []struct {
		create func() imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func() imcache[string, string] {
				return New(WithDefaultExpirationOption[string, string](500 * time.Millisecond))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, string] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithDefaultExpirationOption[string, string](500*time.Millisecond))
			},
		},
	}
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithDefaultExpiration())
			time.Sleep(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
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
	caches := []struct {
		create func() imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func() imcache[string, string] {
				return New(WithDefaultSlidingExpirationOption[string, string](500 * time.Millisecond))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, string] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithDefaultSlidingExpirationOption[string, string](500*time.Millisecond))
			},
		},
	}
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Set("foo", "foo", WithDefaultExpiration())
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			time.Sleep(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
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

type evictionCallbackCall[K comparable, V comparable] struct {
	key    K
	val    V
	reason EvictionReason
}

type evictionCallbackMock[K comparable, V comparable] struct {
	calls []evictionCallbackCall[K, V]
	mu    sync.Mutex
}

func (m *evictionCallbackMock[K, V]) Callback(key K, val V, reason EvictionReason) {
	m.mu.Lock()
	m.calls = append(m.calls, evictionCallbackCall[K, V]{key: key, val: val, reason: reason})
	m.mu.Unlock()
}

func (m *evictionCallbackMock[K, V]) HasEventuallyBeenCalledWith(t *testing.T, key K, val V, reason EvictionReason) {
	t.Helper()
	backoff := 20 * time.Millisecond
	backoffCoefficient := 2
	var lastIndex int
	for i := 0; i < 5; i++ {
		m.mu.Lock()
		for i := lastIndex; i < len(m.calls); i++ {
			if m.calls[i].key == key && m.calls[i].val == val && m.calls[i].reason == reason {
				m.mu.Unlock()
				return
			}
		}
		lastIndex = len(m.calls)
		m.mu.Unlock()
		time.Sleep(backoff)
		backoff *= time.Duration(backoffCoefficient)
	}
	t.Fatalf("want EvictionCallback called with key=%v, val=%v, reason=%s", key, val, reason)
}

func (m *evictionCallbackMock[K, V]) HasNotBeenCalledWith(t *testing.T, key K, val V, reason EvictionReason) {
	t.Helper()
	m.mu.Lock()
	calls := make([]evictionCallbackCall[K, V], 0, len(m.calls))
	for _, c := range m.calls {
		if c.key == key {
			calls = append(calls, c)
		}
	}
	m.mu.Unlock()
	for _, c := range calls {
		if c.val == val && c.reason == reason {
			t.Fatalf("want EvictionCallback not called with key=%v, val=%v, reason=%s", key, val, reason)
		}
	}
}

func (m *evictionCallbackMock[K, V]) HasNotBeenCalled(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) != 0 {
		t.Fatalf("want EvictionCallback not called, got %d calls", len(m.calls))
	}
}

func (m *evictionCallbackMock[K, V]) HasEventuallyBeenCalledTimes(t *testing.T, times int) {
	t.Helper()
	backoff := 20 * time.Millisecond
	backoffCoefficient := 2
	var got int
	for i := 0; i < 5; i++ {
		m.mu.Lock()
		got = len(m.calls)
		m.mu.Unlock()
		if got == times {
			return
		}
		time.Sleep(backoff)
		backoff *= time.Duration(backoffCoefficient)
	}
	t.Fatalf("want EvictionCallback called %d times, got %d", times, got)
}

func (m *evictionCallbackMock[K, V]) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

func TestImcache_Cleaner(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	caches := []struct {
		create func() imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func() imcache[string, string] {
				return New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, string](20*time.Millisecond))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, string] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, string](20*time.Millisecond))
			},
		},
	}
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create()
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			time.Sleep(30 * time.Millisecond)
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
			evictioncMock.HasNotBeenCalledWith(t, "foobar", "foobar", EvictionReasonExpired)
			time.Sleep(200 * time.Millisecond)
			evictioncMock.HasEventuallyBeenCalledWith(t, "foobar", "foobar", EvictionReasonExpired)
		})
	}
}

func TestImcache_Cleaner_IntervalLessOrEqual0(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	caches := []struct {
		create func() imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func() imcache[string, string] {
				return New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, string](0))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, string] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, string](-1))
			},
		},
	}
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create()
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			time.Sleep(200 * time.Millisecond)
			evictioncMock.HasNotBeenCalled(t)
		})
	}
}

func TestImcache_Close(t *testing.T) {
	caches := []struct {
		create func() imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func() imcache[string, string] {
				return New[string, string](WithCleanerOption[string, string](time.Millisecond))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, string] {
				return NewSharded[string, string](8, DefaultStringHasher64{}, WithCleanerOption[string, string](time.Millisecond))
			},
		},
	}
	for _, cache := range caches {
		t.Run(cache.name, func(t *testing.T) {
			c := cache.create()
			c.Close()
			c.Set("foo", "foo", WithNoExpiration())
			if _, ok := c.Get("foo"); ok {
				t.Error("imcache.Get(_) = _, ok, want _, false")
			}
			if got := c.GetMultiple("foo", "bar"); got != nil {
				t.Errorf("imcache.GetMultiple(_) = %v, want %v", got, nil)
			}
			if _, ok := c.Peek("foo"); ok {
				t.Error("imcache.Peek(_) = _, ok, want _, false")
			}
			if got := c.PeekMultiple("foo", "bar"); got != nil {
				t.Errorf("imcache.PeekMultiple(_) = %v, want %v", got, nil)
			}
			if got := c.PeekAll(); got != nil {
				t.Errorf("imcache.PeekAll() = %v, want %v", got, nil)
			}
			v, ok := c.GetOrSet("foo", "bar", WithNoExpiration())
			if ok {
				t.Error("imcache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			if v != "" {
				t.Errorf("imcache.GetOrSet(_, _, _) = %s, _, want %s, _", v, "")
			}
			if ok := c.Replace("foo", "bar", WithNoExpiration()); ok {
				t.Error("imcache.Replace(_, _, _) = true, want false")
			}
			if ok := c.ReplaceWithFunc("foo", func(string) string { return "bar" }, WithNoExpiration()); ok {
				t.Error("imcache.ReplaceWithFunc(_, _, _) = true, want false")
			}
			if ok := c.ReplaceKey("foo", "bar", WithNoExpiration()); ok {
				t.Error("imcache.ReplaceKey(_, _, _) = true, want false", ok)
			}
			if swapped, present := c.CompareAndSwap("foo", "bar", "foobar", func(_, _ string) bool { return true }, WithNoExpiration()); present || swapped {
				t.Errorf("imcache.CompareAndSwap(_, _, _, _, _) = %t, %t, want false, false", swapped, present)
			}
			if ok := c.Remove("foo"); ok {
				t.Error("imcache.Remove(_) = true, want false")
			}
			if got := c.GetAll(); got != nil {
				t.Errorf("imcache.GetAll() = %v, want nil", got)
			}
			if len := c.Len(); len != 0 {
				t.Errorf("imcache.Len() = %d, want %d", len, 0)
			}
			c.RemoveAll()
			c.RemoveExpired()
			c.Close()
		})
	}
}

// cachesWithEvictionCallback is a list of [string, string]
// caches with eviction callback to test.
// If a test needs different type of cache or one with more sophisticated
// configuration, it should be created within the test.
var cachesWithEvictionCallback = []struct {
	create func(EvictionCallback[string, string]) imcache[string, string]
	name   string
}{
	{
		name: "Cache",
		create: func(f EvictionCallback[string, string]) imcache[string, string] {
			return New[string, string](WithEvictionCallbackOption(f))
		},
	},
	{
		name: "Sharded",
		create: func(f EvictionCallback[string, string]) imcache[string, string] {
			return NewSharded[string, string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(f))
		},
	},
}

func TestImcache_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_GetMultiple_EvictionCallback(t *testing.T) {
	evictionMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictionMock.Reset()
			c := cache.create(evictionMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			c.Set("foobar", "foobar", WithSlidingExpiration(100*time.Millisecond))
			time.Sleep(time.Nanosecond)
			want := map[string]string{
				"foobar": "foobar",
			}
			if got := c.GetMultiple("foo", "bar", "foobar", "barfoo"); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetMultiple(_) = %v, want %v", got, want)
			}
			evictionMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictionMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
			time.Sleep(100 * time.Millisecond)
			if got := c.GetMultiple("foo", "bar", "foobar"); got == nil || len(got) != 0 {
				t.Errorf("imcache.GetMultiple(_) = %v, want empty", got)
			}
			evictionMock.HasEventuallyBeenCalledWith(t, "foobar", "foobar", EvictionReasonExpired)
		})
	}
}

func TestImcache_Peek_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			if _, ok := c.Peek("foo"); ok {
				t.Fatal("got imcache.Peek(_) = _, true, want _, false")
			}
			evictioncMock.HasNotBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledTimes(t, 0)
		})
	}
}

func TestImcache_PeekMultiple_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			want := make(map[string]string)
			if got := c.PeekMultiple("foo"); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekMultiple(_) = %v, want %v", got, want)
			}
			evictioncMock.HasNotBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasNotBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledTimes(t, 0)
		})
	}
}

func TestImcache_PeekAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			c.Set("foobar", "foobar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			want := map[string]string{"foobar": "foobar"}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got imcache.PeekAll() = %v, want %v", got, want)
			}
			evictioncMock.HasNotBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasNotBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledTimes(t, 0)
		})
	}
}

func TestImcache_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			c.Set("foo", "bar", WithNoExpiration())
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			c.Set("bar", "foo", WithNoExpiration())
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonReplaced)
		})
	}
}

func TestImcache_GetOrSet_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			time.Sleep(time.Nanosecond)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			if ok := c.Replace("foo", "bar", WithNoExpiration()); ok {
				t.Errorf("imcache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			if ok := c.Replace("bar", "foo", WithNoExpiration()); !ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonReplaced)
		})
	}
}

func TestImcache_ReplaceWithFunc_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "1", WithExpiration(time.Nanosecond))
			c.Set("bar", "2", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			if ok := c.ReplaceWithFunc("foo", func(string) string { return "997" }, WithNoExpiration()); ok {
				t.Errorf("imcache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "1", EvictionReasonExpired)
			if ok := c.ReplaceWithFunc("bar", func(string) string { return "997" }, WithNoExpiration()); !ok {
				t.Errorf("imcache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "2", EvictionReasonReplaced)
		})
	}
}

func TestImcache_CompareAndSwap_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "1", WithExpiration(time.Nanosecond))
			c.Set("bar", "2", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			if swapped, present := c.CompareAndSwap("foo", "1", "997", func(_, _ string) bool { return true }, WithNoExpiration()); swapped || present {
				t.Errorf("imcache.CompareAndSwap(%s, _, _, _, _) = %t, %t, want false, false", "foo", swapped, present)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "1", EvictionReasonExpired)
			if swapped, present := c.CompareAndSwap("bar", "2", "997", func(_, _ string) bool { return true }, WithNoExpiration()); !swapped || !present {
				t.Errorf("imcache.CompareAndSwap(%s, _, _, _, _) = %t, %t want true, true", "bar", swapped, present)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "2", EvictionReasonReplaced)
		})
	}
}

func TestImcache_ReplaceKey_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			if ok := c.ReplaceKey("foo", "bar", WithNoExpiration()); ok {
				t.Errorf("imcache.ReplaceKey(%s, _, _) = %t, want false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			c.Set("foo", "foo", WithNoExpiration())
			if ok := c.ReplaceKey("foo", "bar", WithNoExpiration()); !ok {
				t.Errorf("imcache.ReplaceKey(%s, _, _) = %t, want true", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonKeyReplaced)
			c.Set("foobar", "foobar", WithNoExpiration())
			if ok := c.ReplaceKey("bar", "foobar", WithNoExpiration()); !ok {
				t.Errorf("imcache.ReplaceKey(%s, _, _) = %t, want true", "bar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "foo", EvictionReasonKeyReplaced)
			evictioncMock.HasEventuallyBeenCalledWith(t, "foobar", "foobar", EvictionReasonReplaced)
			c.Set("barbar", "barbar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			if ok := c.ReplaceKey("foobar", "barbar", WithNoExpiration()); !ok {
				t.Errorf("imcache.ReplaceKey(%s, _, _) = %t, want true", "foobar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "barbar", "barbar", EvictionReasonExpired)
		})
	}
}

func TestImcache_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			if ok := c.Remove("foo"); ok {
				t.Errorf("imcache.Remove(%s) = %t, want false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			if ok := c.Remove("bar"); !ok {
				t.Errorf("imcache.Remove(%s) = %t, want true", "bar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonRemoved)
		})
	}
}

func TestImcache_RemoveAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, tt := range cachesWithEvictionCallback {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			c.RemoveAll()
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonRemoved)
		})
	}
}

func TestImcache_RemoveExpired_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			time.Sleep(time.Nanosecond)
			c.RemoveExpired()
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_GetAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock[string, string]{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithSlidingExpiration(time.Second))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			time.Sleep(time.Nanosecond)
			got := c.GetAll()
			want := map[string]string{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
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

func AssertEntryEvictedFunc[K comparable, V comparable](c imcache[K, V], mcallback *evictionCallbackMock[K, V]) func(*testing.T, K, V, EvictionReason) {
	return func(t *testing.T, key K, val V, reason EvictionReason) {
		t.Helper()
		if got, ok := c.Get(key); ok {
			t.Fatalf("want Cache.Get(%v) = _, false, got %v, true", key, got)
		}
		if mcallback != nil {
			mcallback.HasEventuallyBeenCalledWith(t, key, val, reason)
		}
	}
}

func TestCache_MaxEntriesLimit_EvictionPolicyLRU(t *testing.T) {
	tests := []struct {
		name  string
		setup func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason))
	}{
		{
			name: "no eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason)) {
				c := New(WithMaxEntriesLimitOption[string, int](5, EvictionPolicyLRU))
				return c, AssertEntryEvictedFunc[string, int](c, nil)
			},
		},
		{
			name: "eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason)) {
				mcallback := &evictionCallbackMock[string, int]{}
				c := New(WithEvictionCallbackOption(mcallback.Callback), WithMaxEntriesLimitOption[string, int](5, EvictionPolicyLRU))
				return c, AssertEntryEvictedFunc[string, int](c, mcallback)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, evicted := tt.setup()

			c.Set("one", 1, WithNoExpiration())
			c.Set("two", 2, WithNoExpiration())
			c.Set("three", 3, WithNoExpiration())
			c.Set("four", 4, WithExpiration(time.Nanosecond))
			c.Set("five", 5, WithNoExpiration())
			// LRU queue: five -> four -> three -> two -> one.

			// Set should evict the last entry from the queue if the size is exceeded.
			c.Set("six", 6, WithNoExpiration())
			// LRU queue: six -> five -> four -> three -> two.
			evicted(t, "one", 1, EvictionReasonMaxEntriesExceeded)

			// Get should move the entry to the front of the queue.
			if _, ok := c.Get("two"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// Peek shouldn't move the entry to the front of the queue.
			if _, ok := c.Peek("three"); !ok {
				t.Fatal("got Cache.Peek(_) = _, false, want _, true")
			}
			// LRU queue: two -> six -> five -> four -> three.
			c.Set("seven", 7, WithNoExpiration())
			// LRU queue: seven -> two -> six -> five -> four.
			evicted(t, "three", 3, EvictionReasonMaxEntriesExceeded)

			// Set should evict the last entry from the queue if the size is exceeded
			// and if the entry is expired the eviction reason should be EvictionReasonExpired.
			c.Set("eight", 8, WithNoExpiration())
			// LRU queue: eight -> seven -> two -> six -> five.
			evicted(t, "four", 4, EvictionReasonExpired)

			// Replace should update the entry and move it to the front of the queue.
			if ok := c.Replace("five", 5, WithNoExpiration()); !ok {
				t.Fatal("got Cache.Replace(_) = false, want true")
			}
			// LRU queue: five -> eight -> seven -> two -> six.
			c.Set("nine", 9, WithNoExpiration())
			// LRU queue: nine -> five -> eight -> seven -> two.
			evicted(t, "six", 6, EvictionReasonMaxEntriesExceeded)

			// ReplaceWithFunc should update the entry and move it to the front of the queue.
			if ok := c.ReplaceWithFunc("two", func(int) int { return 2 }, WithNoExpiration()); !ok {
				t.Fatal("got Cache.ReplaceWithFunc(_) = false, want true")
			}
			// LRU queue: two -> nine -> five -> eight -> seven.
			c.Set("ten", 10, WithNoExpiration())
			// LRU queue: ten -> two -> nine -> five -> eight.
			evicted(t, "seven", 7, EvictionReasonMaxEntriesExceeded)

			// Set should not evict any entry if the size is not exceeded.
			c.Set("ten", 10, WithNoExpiration())
			// LRU queue: ten -> two -> nine -> five -> eight.
			if _, ok := c.Get("eight"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// LRU queue: eight -> ten -> two -> nine -> five.

			// GetAll should not change the LRU queue.
			c.GetAll()
			// LRU queue: eight -> ten -> two -> nine -> five.
			c.Set("eleven", 11, WithNoExpiration())
			// LRU queue: eleven -> eight -> ten -> two -> nine.
			evicted(t, "five", 5, EvictionReasonMaxEntriesExceeded)

			// Remove should not mess with the LRU queue.
			if ok := c.Remove("two"); !ok {
				t.Fatal("got Cache.Remove(_) = false, want true")
			}
			// LRU queue: eleven -> eight -> ten -> nine.
			c.Set("twelve", 12, WithNoExpiration())
			// LRU queue: twelve -> eleven -> eight -> ten -> nine.
			if _, ok := c.Get("nine"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// PeekAll should not change the LRU queue.
			want := map[string]int{"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got Cache.PeekAll() = %v, want %v", got, want)
			}
			// LRU queue: nine -> twelve -> eleven -> eight -> ten.
			c.Set("thirteen", 13, WithNoExpiration())
			// LRU queue: thirteen -> nine -> twelve -> eleven -> eight.
			evicted(t, "ten", 10, EvictionReasonMaxEntriesExceeded)

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
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// PeekMultiple shouldn't move the entires to the front of the queue.
			want = map[string]int{"fourteen": 14, "fifteen": 15}
			if got := c.PeekMultiple("fourteen", "fifteen"); !reflect.DeepEqual(got, want) {
				t.Fatalf("got Cache.PeekMultiple(_) = %v, want %v", got, want)
			}
			// LRU queue: fourteen -> eighteen -> seventeen -> sixteen -> fifteen.
			c.Set("nineteen", 19, WithExpiration(time.Nanosecond))
			// LRU queue: nineteen -> fourteen -> eighteen -> seventeen -> sixteen.
			evicted(t, "fifteen", 15, EvictionReasonMaxEntriesExceeded)
			// To make sure nineteen expired.
			time.Sleep(time.Nanosecond)

			// RemoveExpired should not mess with the LRU queue.
			c.RemoveExpired()
			// LRU queue: fourteen -> eighteen -> seventeen.
			c.Set("twenty", 20, WithNoExpiration())
			// LRU queue: twenty -> fourteen -> eighteen -> seventeen.
			if _, ok := c.Get("seventeen"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// LRU queue: seventeen -> twenty -> fourteen -> eighteen.
			c.Set("twentyone", 21, WithExpiration(200*time.Millisecond))
			// LRU queue: twentyone -> seventeen -> twenty -> fourteen -> eighteen.
			if _, ok := c.Get("eighteen"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// LRU queue: eighteen -> twentyone -> seventeen -> twenty -> fourteen.

			// GetOrSet should cause eviction if the size is exceeded.
			if _, ok := c.GetOrSet("twentytwo", 22, WithNoExpiration()); ok {
				t.Fatal("got Cache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			// LRU queue: twentytwo -> eighteen -> twentyone -> seventeen -> twenty.
			evicted(t, "fourteen", 14, EvictionReasonMaxEntriesExceeded)

			// GetOrSet should move the entry to the front of the LRU queue.
			if _, ok := c.GetOrSet("twenty", 20, WithNoExpiration()); !ok {
				t.Fatal("got Cache.GetOrSet(_, _, _) = _, false, want _, true")
			}
			// LRU queue: twenty -> twentytwo -> eighteen -> twentyone -> seventeen.
			// Wait until seventeen is expired.
			time.Sleep(100 * time.Millisecond)
			// seventeen is expired, but it's still in the cache.
			c.Set("twentythree", 23, WithNoExpiration())
			// LRU queue: twentythree -> twenty -> twentytwo -> eighteen -> twentyone.
			// seventeen should be evicted with an expired reason instead of max entries exceeded.
			evicted(t, "seventeen", 17, EvictionReasonExpired)
			// Wait until twentyone is expired.
			time.Sleep(100 * time.Millisecond)
			// twentyone is expired, but it's still in the cache.
			if _, ok := c.GetOrSet("twentyfour", 24, WithNoExpiration()); ok {
				t.Fatal("got Cache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			// LRU queue: twentyfour -> twentythree -> twenty -> twentytwo -> eighteen.
			// twentyone should be evicted with an expired reason instead of max entries exceeded.
			evicted(t, "twentyone", 21, EvictionReasonExpired)

			// GetMultiple should move the entries to the front of the LRU queue.
			if got := c.GetMultiple("eighteen", "twentytwo"); len(got) == 0 {
				t.Fatalf("got len(Cache.GetMultiple(_)) = %d, want 2", len(got))
			}
			// LRU queue: twentytwo -> eighteen -> twentyfour -> twentythree -> twenty.
			c.Set("twentyfive", 25, WithNoExpiration())
			// LRU queue: twentyfive -> twentytwo -> eighteen -> twentyfour -> twentythree.
			evicted(t, "twenty", 20, EvictionReasonMaxEntriesExceeded)

			// CompareAndSwap should move the entry to the front of the LRU queue if swapped.
			if swapped, present := c.CompareAndSwap("twentythree", 23, 2323, func(_, _ int) bool { return true }, WithNoExpiration()); !present || !swapped {
				t.Fatalf("got Cache.CompareAndSwap(_, _, _, _, _) = %t, %t, want true, true", swapped, present)
			}
			// LRU queue: twentythree -> twentyfive -> twentytwo -> eighteen -> twentyfour.
			c.Set("twentysix", 26, WithNoExpiration())
			// LRU queue: twentysix -> twentythree -> twentyfive -> twentytwo -> eighteen.
			evicted(t, "twentyfour", 24, EvictionReasonMaxEntriesExceeded)

			// CompareAndSwap should move the entry to the front of the LRU queue if not swapped.
			if swapped, present := c.CompareAndSwap("eighteen", 18, 1818, func(_, _ int) bool { return false }, WithNoExpiration()); swapped || !present {
				t.Fatalf("got Cache.CompareAndSwap(_, _, _, _, _) = %t, %t, want false, true", swapped, present)
			}
			// LRU queue: eighteen -> twentysix -> twentythree -> twentyfive -> twentytwo.
			c.Set("twentyseven", 27, WithNoExpiration())
			// LRU queue: twentyseven -> eighteen -> twentysix -> twentythree -> twentyfive.
			evicted(t, "twentytwo", 22, EvictionReasonMaxEntriesExceeded)
		})
	}
}

type hasher struct{}

func (hasher) Sum64(key int) uint64 {
	return uint64(key)
}

func TestSharded_ReplaceKey_MaxEntriesLimit_EvictionPolicyLRU(t *testing.T) {
	tests := []struct {
		name  string
		setup func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason))
	}{
		{
			name: "no eviction callback",
			setup: func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason)) {
				s := NewSharded[int, int](2, hasher{}, WithMaxEntriesLimitOption[int, int](2, EvictionPolicyLRU))
				return s, AssertEntryEvictedFunc[int, int](s, nil)
			},
		},
		{
			name: "eviction callback",
			setup: func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason)) {
				mcallback := &evictionCallbackMock[int, int]{}
				s := NewSharded[int, int](2, hasher{}, WithMaxEntriesLimitOption[int, int](2, EvictionPolicyLRU), WithEvictionCallbackOption(mcallback.Callback))
				return s, AssertEntryEvictedFunc[int, int](s, mcallback)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, evicted := tt.setup()

			s.Set(1, 1, WithNoExpiration())
			s.Set(2, 2, WithNoExpiration())
			s.Set(3, 3, WithNoExpiration())
			s.Set(4, 4, WithNoExpiration())
			if ok := s.ReplaceKey(4, 5, WithExpiration(time.Nanosecond)); !ok {
				t.Fatal("Sharded.ReplaceKey(_, _, _) = false, want true")
			}
			evicted(t, 4, 4, EvictionReasonKeyReplaced)
			evicted(t, 1, 1, EvictionReasonMaxEntriesExceeded)
			if _, ok := s.Get(3); !ok {
				t.Fatal("Sharded.Get(_) = _, false, want _, true")
			}
			time.Sleep(time.Nanosecond)
			if ok := s.ReplaceKey(2, 7, WithNoExpiration()); !ok {
				t.Fatal("Sharded.ReplaceKey(_, _, _) = false, want true")
			}
			evicted(t, 2, 2, EvictionReasonKeyReplaced)
			evicted(t, 5, 4, EvictionReasonExpired)
		})
	}
}

func TestCache_MaxEntriesLimit_EvictionPolicyLFU(t *testing.T) {
	tests := []struct {
		name  string
		setup func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason))
	}{
		{
			name: "no eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason)) {
				c := New(WithMaxEntriesLimitOption[string, int](5, EvictionPolicyLFU))
				return c, AssertEntryEvictedFunc[string, int](c, nil)
			},
		},
		{
			name: "eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, string, int, EvictionReason)) {
				mcallback := &evictionCallbackMock[string, int]{}
				c := New(WithEvictionCallbackOption(mcallback.Callback), WithMaxEntriesLimitOption[string, int](5, EvictionPolicyLFU))
				return c, AssertEntryEvictedFunc[string, int](c, mcallback)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, evicted := tt.setup()

			c.Set("one", 1, WithNoExpiration())
			c.Set("two", 2, WithNoExpiration())
			c.Set("three", 3, WithNoExpiration())
			c.Set("four", 4, WithNoExpiration())
			c.Set("five", 5, WithNoExpiration())
			// LFU queue: five -> four -> three -> two -> one.

			// Set should evict the last entry from the queue if the size is exceeded.
			// All entries have the same frequency hence the LRU policy applied.
			c.Set("six", 6, WithNoExpiration())
			// LFU queue: six -> five -> four -> three -> two.
			evicted(t, "one", 1, EvictionReasonMaxEntriesExceeded)

			// Get should update the entry frequency.
			if _, ok := c.Get("two"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			if _, ok := c.Get("two"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			if _, ok := c.Get("three"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			if _, ok := c.Get("four"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			if _, ok := c.Get("five"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			if _, ok := c.Get("six"); !ok {
				t.Fatal("got Cache.Get(_) = _, false, want _, true")
			}
			// LFU queue: two -> six -> five -> four -> three.
			// Peek shouldn't update the entry frequency.
			if _, ok := c.Peek("three"); !ok {
				t.Fatal("got Cache.Peek(_) = _, false, want _, true")
			}
			c.Set("seven", 7, WithExpiration(time.Nanosecond))
			// LFU queue: two -> six -> five -> four -> seven.
			evicted(t, "three", 3, EvictionReasonMaxEntriesExceeded)

			// Set should evict the last entry from the queue if the size is exceeded
			// and if the entry is expired the eviction reason should be EvictionReasonExpired.
			c.Set("eight", 8, WithNoExpiration())
			// LFU queue: two -> six -> five -> four -> eight.
			evicted(t, "seven", 7, EvictionReasonExpired)

			// Replace should update the frequency of the entry if the entry already exists.
			c.Set("eight", 8, WithNoExpiration())
			// LFU queue: two -> eight -> six -> five -> four.
			// PeekMultiple shouldn't update the frequencies.
			want := map[string]int{"four": 4, "five": 5}
			if got := c.PeekMultiple("four", "five"); !reflect.DeepEqual(got, want) {
				t.Fatalf("got Cache.PeekMultiple(_) = %v, want %v", got, want)
			}
			c.Set("nine", 9, WithNoExpiration())
			// LFU queue: two -> eight -> six -> five -> nine.
			evicted(t, "four", 4, EvictionReasonMaxEntriesExceeded)

			// Replace should update the entry frequency.
			if ok := c.Replace("nine", 9, WithNoExpiration()); !ok {
				t.Fatal("got Cache.Replace(_) = false, want true")
			}
			// LFU queue: two -> nine -> eight -> six -> five.
			c.Set("ten", 10, WithNoExpiration())
			// LFU queue: two -> nine -> eight -> six -> ten.
			evicted(t, "five", 5, EvictionReasonMaxEntriesExceeded)

			// ReplaceWithFunc should update the entry frequency.
			if ok := c.ReplaceWithFunc("ten", func(int) int { return 10 }, WithNoExpiration()); !ok {
				t.Fatal("got Cache.ReplaceWithFunc(_) = false, want true")
			}
			// LFU queue: two -> ten -> nine -> eight -> six.
			c.Set("eleven", 11, WithNoExpiration())
			// LFU queue: two -> ten -> nine -> eight -> eleven.
			evicted(t, "six", 6, EvictionReasonMaxEntriesExceeded)

			if ok := c.Remove("eight"); !ok {
				t.Fatal("got Cache.Remove(_) = false, want true")
			}
			// LFU queue: two -> ten -> nine -> eleven.
			c.Set("twelve", 12, WithNoExpiration())
			// LFU queue: two -> ten -> nine -> twelve -> eleven.

			// GetAll should update the frequency of all the entries.
			c.GetAll()
			// LFU queue: two -> ten -> nine -> twelve -> eleven.
			c.Set("thirteen", 13, WithNoExpiration())
			// LFU queue: two -> ten -> nine -> twelve -> thirteen.
			evicted(t, "eleven", 11, EvictionReasonMaxEntriesExceeded)
			c.Set("fourteen", 14, WithNoExpiration())
			// LFU queue: two -> ten -> nine -> twelve -> fourteen.
			evicted(t, "thirteen", 13, EvictionReasonMaxEntriesExceeded)

			// RemoveAll resets the LFU queue.
			c.RemoveAll()
			c.Set("fifteen", 15, WithNoExpiration())
			c.Set("sixteen", 16, WithNoExpiration())
			c.Set("seventeen", 17, WithExpiration(time.Nanosecond))
			c.Set("eighteen", 18, WithNoExpiration())
			c.Set("nineteen", 19, WithExpiration(time.Nanosecond))
			// LFU queue: nineteen -> eighteen -> seventeen -> sixteen -> fifteen.
			c.RemoveExpired()
			// LFU queue: eighteen -> sixteen -> fifteen.
			c.GetAll()
			// LFU queue: eighteen -> sixteen -> fifteen.
			c.Set("twenty", 20, WithNoExpiration())
			// PeekAll shouldn't update the entries frequency.
			want = map[string]int{"fifteen": 15, "sixteen": 16, "eighteen": 18, "twenty": 20}
			if got := c.PeekAll(); !reflect.DeepEqual(got, want) {
				t.Fatalf("got Cache.PeekAll() = %v, want %v", got, want)
			}
			// LFU queue: eighteen -> sixteen -> fifteen -> twenty.
			c.Set("twentyone", 21, WithNoExpiration())
			// LFU queue: eighteen -> sixteen -> fifteen -> twentyone -> twenty.
			// GetOrSet should cause eviction if the entry doesn't exist and the size is exceeded.
			if _, ok := c.GetOrSet("twentytwo", 22, WithNoExpiration()); ok {
				t.Fatal("got Cache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			// LFU queue: eighteen -> sixteen -> fifteen -> twentytwo -> twentyone.
			evicted(t, "twenty", 20, EvictionReasonMaxEntriesExceeded)

			// GetOrSet should update the entry frequency if the entry exists.
			if _, ok := c.GetOrSet("twentytwo", 22, WithNoExpiration()); !ok {
				t.Fatal("got Cache.GetOrSet(_, _, _) = _, false, want _, true")
			}
			// LFU queue: twentytwo -> eighteen -> sixteen -> fifteen -> twentyone.
			c.Set("twentythree", 23, WithNoExpiration())
			// LFU queue: twentytwo -> eighteen -> sixteen -> fifteen -> twentythree.
			evicted(t, "twentyone", 21, EvictionReasonMaxEntriesExceeded)
			c.Set("twentyfour", 24, WithNoExpiration())
			// LFU queue: twentytwo -> eighteen -> sixteen -> fifteen -> twentyfour.
			evicted(t, "twentythree", 23, EvictionReasonMaxEntriesExceeded)

			// GetMultiple should update the entries frequency.
			if got := c.GetMultiple("fifteen", "twentyfour"); len(got) != 2 {
				t.Fatalf("got len(Cache.GetMultiple(_)) = %d, want 2", len(got))
			}
			// LFU queue: fifteen -> twentyfour -> twentytwo -> eighteen -> sixteen.
			c.Set("twentyfive", 25, WithNoExpiration())
			// LFU queue: fifteen -> twentyfour -> twentytwo -> eighteen -> twentyfive.
			evicted(t, "sixteen", 16, EvictionReasonMaxEntriesExceeded)

			// CompareAndSwap should update the entry frequency if swapped.
			if swapped, present := c.CompareAndSwap("twentyfive", 25, 25, func(_, _ int) bool { return true }, WithNoExpiration()); !present || !swapped {
				t.Fatalf("got Cache.CompareAndSwap(_, _, _, _, _) = %t, %t, want true, true", swapped, present)
			}
			// LFU queue: fifteen -> twentyfive -> twentyfour -> twentytwo -> eighteen.
			c.Set("twentysix", 26, WithNoExpiration())
			// LFU queue: fifteen -> twentyfive -> twentyfour -> twentytwo -> twentysix.
			evicted(t, "eighteen", 18, EvictionReasonMaxEntriesExceeded)

			// CompareAndSwap should update the entry frequency if not swapped.
			if swapped, present := c.CompareAndSwap("twentysix", 26, 26, func(_, _ int) bool { return false }, WithNoExpiration()); swapped || !present {
				t.Fatalf("got Cache.CompareAndSwap(_, _, _, _, _) = %t, %t, want false, true", swapped, present)
			}
			// LFU queue: fifteen -> twentysix -> twentyfive -> twentyfour -> twentytwo.
			c.Set("twentyseven", 27, WithNoExpiration())
			// LFU queue: fifteen -> twentysix -> twentyfive -> twentyfour -> twentyseven.
			evicted(t, "twentytwo", 22, EvictionReasonMaxEntriesExceeded)
		})
	}
}

func TestSharded_ReplaceKey_MaxEntriesLimit_EvictionPolicyLFU(t *testing.T) {
	tests := []struct {
		name  string
		setup func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason))
	}{
		{
			name: "no eviction callback",
			setup: func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason)) {
				s := NewSharded[int, int](2, hasher{}, WithMaxEntriesLimitOption[int, int](2, EvictionPolicyLFU))
				return s, AssertEntryEvictedFunc[int, int](s, nil)
			},
		},
		{
			name: "eviction callback",
			setup: func() (*Sharded[int, int], func(*testing.T, int, int, EvictionReason)) {
				mcallback := &evictionCallbackMock[int, int]{}
				s := NewSharded[int, int](2, hasher{}, WithMaxEntriesLimitOption[int, int](2, EvictionPolicyLFU), WithEvictionCallbackOption(mcallback.Callback))
				return s, AssertEntryEvictedFunc[int, int](s, mcallback)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, evicted := tt.setup()

			s.Set(1, 1, WithNoExpiration())
			s.Set(2, 2, WithNoExpiration())
			s.Set(3, 3, WithNoExpiration())
			s.Set(4, 4, WithNoExpiration())
			if ok := s.ReplaceKey(4, 5, WithExpiration(50*time.Millisecond)); !ok {
				t.Fatal("Sharded.ReplaceKey(_, _, _) = false, want true")
			}
			evicted(t, 4, 4, EvictionReasonKeyReplaced)
			evicted(t, 1, 1, EvictionReasonMaxEntriesExceeded)
			if _, ok := s.Get(3); !ok {
				t.Fatal("Sharded.Get(_) = _, false, want _, true")
			}
			if _, ok := s.Get(3); !ok {
				t.Fatal("Sharded.Get(_) = _, false, want _, true")
			}
			if _, ok := s.Get(5); !ok {
				t.Fatal("Sharded.Get(_) = _, false, want _, true")
			}
			time.Sleep(50 * time.Millisecond)
			if ok := s.ReplaceKey(2, 7, WithNoExpiration()); !ok {
				t.Fatal("Sharded.ReplaceKey(_, _, _) = false, want true")
			}
			evicted(t, 2, 2, EvictionReasonKeyReplaced)
			evicted(t, 5, 4, EvictionReasonExpired)
			if _, ok := s.Get(3); !ok {
				t.Fatal("Sharded.Get(_) = _, false, want _, true")
			}
		})
	}
}

func TestCache_MaxEntriesLimit_EvictionPolicyRandom(t *testing.T) {
	tests := []struct {
		name  string
		setup func() (*Cache[string, int], func(*testing.T, int, int))
	}{
		{
			name: "no eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, int, int)) {
				c := New(WithMaxEntriesLimitOption[string, int](5, EvictionPolicyRandom))
				return c, func(t *testing.T, len, _ int) {
					t.Helper()
					if c.len() != len {
						t.Fatalf("got Cache.Len() = %d, want %d", c.len(), len)
					}
				}
			},
		},
		{
			name: "eviction callback",
			setup: func() (*Cache[string, int], func(*testing.T, int, int)) {
				mcallback := &evictionCallbackMock[string, int]{}
				c := New(WithEvictionCallbackOption(mcallback.Callback), WithMaxEntriesLimitOption[string, int](5, EvictionPolicyRandom))
				return c, func(t *testing.T, len, times int) {
					t.Helper()
					if c.len() != len {
						t.Fatalf("got Cache.Len() = %d, want %d", c.len(), len)
					}
					mcallback.HasEventuallyBeenCalledTimes(t, times)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, evicted := tt.setup()

			c.Set("one", 1, WithNoExpiration())
			c.Set("two", 2, WithNoExpiration())
			c.Set("three", 3, WithNoExpiration())
			c.Set("four", 4, WithNoExpiration())
			c.Set("five", 5, WithNoExpiration())
			evicted(t, 5, 0)

			// Set should evict the random entry from the Cache if the size is exceeded.
			c.Set("six", 6, WithNoExpiration())
			evicted(t, 5, 1)
			c.Set("seven", 7, WithExpiration(time.Nanosecond))
			evicted(t, 5, 2)
			c.Set("eight", 8, WithNoExpiration())
			evicted(t, 5, 3)

			if ok := c.Remove("eight"); !ok {
				t.Fatal("want Cache.Remove(_) = true, got false")
			}
			evicted(t, 4, 4)

			c.Set("nine", 9, WithNoExpiration())
			evicted(t, 5, 4)

			// GetOrSet should cause eviction if the entry doesn't exist and the size is exceeded.
			if _, ok := c.GetOrSet("ten", 10, WithNoExpiration()); ok {
				t.Fatal("Cache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			evicted(t, 5, 5)
			if _, ok := c.GetOrSet("eleven", 11, WithNoExpiration()); ok {
				t.Fatal("Cache.GetOrSet(_, _, _) = _, true, want _, false")
			}
			evicted(t, 5, 6)
		})
	}
}

func TestCache_MaxEntriesLimit_InvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		policy EvictionPolicy
		limit  int
	}{
		{
			name:   "limit equals to 0",
			policy: EvictionPolicyLFU,
			limit:  0,
		},
		{
			name:   "limit less than 0",
			policy: EvictionPolicyLRU,
			limit:  -112,
		},
		{
			name:   "unsupported eviction policy",
			policy: 0,
			limit:  10000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(WithMaxEntriesLimitOption[string, string](tt.limit, tt.policy))
			if _, ok := c.queue.(nopEvictionQueue[string, string]); !ok {
				t.Error("Cache.queue = _, want nopEvictionQueue")
			}
		})
	}
}

type longRunningEvictionCallback struct {
	done chan struct{}
}

func (c *longRunningEvictionCallback) Callback(key, value string, reason EvictionReason) {
	<-c.done
}

func TestImcache_LongRunning_EvictionCallback(t *testing.T) {
	tests := []struct {
		execute func(imcache[string, string])
		name    string
	}{
		{
			name: "Get evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Get("foo")
			},
		},
		{
			name: "Set evict replaced entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.Set("foo", "foo", WithNoExpiration())
			},
		},
		{
			name: "Set evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Set("foo", "foo", WithNoExpiration())
			},
		},
		{
			name: "Set evict expired entry if max entries limit exceeded",
			execute: func(c imcache[string, string]) {
				c.Set("foo1", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Set("foo3", "foo", WithNoExpiration())
				c.Set("foo5", "foobar", WithNoExpiration())
			},
		},
		{
			name: "Set evict entry if max entries limit exceeded",
			execute: func(c imcache[string, string]) {
				c.Set("foo1", "bar", WithNoExpiration())
				c.Set("foo3", "foo", WithNoExpiration())
				c.Set("foo5", "foobar", WithNoExpiration())
			},
		},
		{
			name: "GetOrSet evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.GetOrSet("foo", "foo", WithNoExpiration())
			},
		},
		{
			name: "GetOrSet evict expired entry if max entries limit exceeded",
			execute: func(c imcache[string, string]) {
				c.Set("foo1", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Set("foo3", "foo", WithNoExpiration())
				c.GetOrSet("foo5", "foobar", WithNoExpiration())
			},
		},
		{
			name: "GetOrSet evict entry if max entries limit exceeded",
			execute: func(c imcache[string, string]) {
				c.Set("foo1", "bar", WithNoExpiration())
				c.Set("foo3", "foo", WithNoExpiration())
				c.GetOrSet("foo5", "foobar", WithNoExpiration())
			},
		},
		{
			name: "Replace evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Replace("foo", "foo", WithNoExpiration())
			},
		},
		{
			name: "Replace evict replaced entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.Replace("foo", "foo", WithNoExpiration())
			},
		},
		{
			name: "ReplaceWithFunc evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.ReplaceWithFunc("foo", func(_ string) string { return "foo" }, WithNoExpiration())
			},
		},
		{
			name: "ReplaceWithFunc evict replaced entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.ReplaceWithFunc("foo", func(_ string) string { return "foo" }, WithNoExpiration())
			},
		},
		{
			name: "ReplaceKey evict expired entry under old key",
			execute: func(c imcache[string, string]) {
				c.Set("foo1", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.ReplaceKey("foo1", "foo2", WithNoExpiration())
			},
		},
		{
			name: "ReplaceKey evict expired entry under new key",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.ReplaceKey("bar", "foo", WithNoExpiration())
			},
		},
		{
			name: "ReplaceKey evict replaced entry under new key",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foo", "bar", WithNoExpiration())
				c.ReplaceKey("bar", "foo", WithNoExpiration())
			},
		},
		{
			name: "ReplaceKey evict replaced key entry",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.ReplaceKey("bar", "foo", WithNoExpiration())
			},
		},
		{
			name: "CompareAndSwap evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.CompareAndSwap("foo", "bar", "foo", func(_, _ string) bool { return true }, WithNoExpiration())
			},
		},
		{
			name: "CompareAndSwap evict replaced entry",
			execute: func(c imcache[string, string]) {
				c.Set("foo", "bar", WithNoExpiration())
				c.CompareAndSwap("foo", "bar", "foo", func(_, _ string) bool { return true }, WithNoExpiration())
			},
		},
		{
			name: "Remove evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.Remove("bar")
			},
		},
		{
			name: "Remove evict removed entry",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Remove("bar")
			},
		},
		{
			name: "RemoveAll evict removed and expired entries",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.RemoveAll()
			},
		},
		{
			name: "RemoveExpired evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.RemoveExpired()
			},
		},
		{
			name: "GetAll evict expired entry",
			execute: func(c imcache[string, string]) {
				c.Set("bar", "foo", WithNoExpiration())
				c.Set("foo", "bar", WithExpirationDate(time.Now().Add(-1*time.Second)))
				c.GetAll()
			},
		},
	}
	caches := []struct {
		create func(EvictionCallback[string, string]) imcache[string, string]
		name   string
	}{
		{
			name: "Cache",
			create: func(ec EvictionCallback[string, string]) imcache[string, string] {
				return New[string, string](WithEvictionCallbackOption[string, string](ec), WithMaxEntriesLimitOption[string, string](2, EvictionPolicyLRU))
			},
		},
		{
			name: "Sharded",
			create: func(ec EvictionCallback[string, string]) imcache[string, string] {
				return NewSharded[string, string](2, DefaultStringHasher64{}, WithEvictionCallbackOption[string, string](ec), WithMaxEntriesLimitOption[string, string](2, EvictionPolicyLRU))
			},
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				ecDoneCh := make(chan struct{})
				ec := &longRunningEvictionCallback{done: ecDoneCh}
				defer func() { close(ecDoneCh) }()
				c := cache.create(ec.Callback)
				done := make(chan struct{})
				go func() {
					tt.execute(c)
					close(done)
				}()
				select {
				case <-done:
				case <-ctx.Done():
					t.Errorf("%s should not block on a long running eviction callback", cache.name)
				}
			})
		}
	}
}

func TestSharded_ReplaceKey_LongRunning_EvictionCallback_MaxEntriesLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ecDoneCh := make(chan struct{})
	ec := &longRunningEvictionCallback{done: ecDoneCh}
	defer func() { close(ecDoneCh) }()
	c := NewSharded[string, string](2, DefaultStringHasher64{}, WithEvictionCallbackOption[string, string](ec.Callback), WithMaxEntriesLimitOption[string, string](1, EvictionPolicyLRU))
	c.Set("foo1", "bar", WithNoExpiration())
	c.Set("foo2", "bar", WithNoExpiration())
	done := make(chan struct{})
	go func() {
		c.ReplaceKey("foo1", "foo4", WithNoExpiration())
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Error("ReplaceKey should not block on a long running eviction callback")
	}
}

func TestEntry_expired(t *testing.T) {
	tests := []struct {
		now   time.Time
		name  string
		entry entry[string, string]
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: noExp}},
			now:   time.Now(),
		},
		{
			name:  "expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(-time.Second).UnixNano()}},
			now:   time.Now(),
			want:  true,
		},
		{
			name:  "not expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(time.Second).UnixNano()}},
			now:   time.Now(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.expired(tt.now); got != tt.want {
				t.Errorf("entry.expired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_slide(t *testing.T) {
	entry := entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(5 * time.Second).UnixNano(), sliding: 5 * time.Second}}
	time.Sleep(2 * time.Second)
	now := time.Now()
	entry.slide(now)
	if want := now.Add(5 * time.Second).UnixNano(); entry.exp.date != want {
		t.Errorf("entry.slide() results in expiration %v, want %v", entry.exp.date, want)
	}
}
