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

func init() {
	rand.Seed(time.Now().UnixNano())
}

type imcache[K comparable, V any] interface {
	Get(key K) (v V, present bool)
	Set(key K, val V, exp Expiration)
	GetOrSet(key K, val V, exp Expiration) (v V, present bool)
	Replace(key K, val V, exp Expiration) (present bool)
	ReplaceWithFunc(key K, f func(old V) (new V), exp Expiration) (present bool)
	ReplaceKey(oldKey, newKey K, exp Expiration) (present bool)
	Remove(key K) (present bool)
	RemoveAll()
	RemoveExpired()
	GetAll() map[K]V
	Len() int
	Close()
}

// caches is a list of string-string caches with default configuration to test.
// If a test needs different type of cache or configured one, it should be
// created within the test.
var caches = []struct {
	name   string
	create func() imcache[string, string]
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
				<-time.After(time.Nanosecond)
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
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
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
				<-time.After(time.Nanosecond)
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
				<-time.After(time.Nanosecond)
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
			<-time.After(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.GetOrSet("foo", "bar", WithSlidingExpiration(500*time.Millisecond)); !ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
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
				<-time.After(time.Nanosecond)
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
	var caches = []struct {
		name   string
		create func() imcache[string, int32]
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
		name    string
		setup   func(imcache[string, int32])
		key     string
		f       func(int32) int32
		val     int32
		present bool
	}{
		{
			name: "success",
			setup: func(c imcache[string, int32]) {
				c.Set("foo", 997, WithNoExpiration())
			},
			key:     "foo",
			f:       Increment[int32],
			val:     998,
			present: true,
		},
		{
			name: "entry expired",
			setup: func(c imcache[string, int32]) {
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name:  "entry doesn't exist",
			setup: func(c imcache[string, int32]) {},
			key:   "foo",
			f:     Increment[int32],
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
		name  string
		setup func(imcache[string, string])
		old   string
		new   string
		want  bool
		val   string
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
				<-time.After(time.Nanosecond)
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
				<-time.After(time.Nanosecond)
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
			<-time.After(time.Nanosecond)
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
			<-time.After(time.Nanosecond)
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
			<-time.After(time.Nanosecond)
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
			<-time.After(300 * time.Millisecond)
			want := map[string]string{
				"foo": "foo",
				"bar": "bar",
			}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(300 * time.Millisecond)
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("imcache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(500 * time.Millisecond)
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
		name   string
		create func() imcache[string, string]
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
			<-time.After(500 * time.Millisecond)
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
		name   string
		create func() imcache[string, string]
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
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
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
	m.calls = append(m.calls, evictionCallbackCall{key: key, val: val, reason: reason})
	m.mu.Unlock()
}

func (m *evictionCallbackMock) HasEventuallyBeenCalledWith(t *testing.T, key string, val interface{}, reason EvictionReason) {
	t.Helper()
	initialBackoff := 20 * time.Millisecond
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
		<-time.After(initialBackoff)
		initialBackoff *= time.Duration(backoffCoefficient)
	}
	t.Fatalf("want EvictionCallback called with key=%s, val=%v, reason=%s", key, val, reason)
}

func (m *evictionCallbackMock) HasNotBeenCalledWith(t *testing.T, key string, val interface{}, reason EvictionReason) {
	t.Helper()
	m.mu.Lock()
	calls := make([]evictionCallbackCall, 0, len(m.calls))
	for _, c := range m.calls {
		if c.key == key {
			calls = append(calls, c)
		}
	}
	m.mu.Unlock()
	for _, c := range calls {
		if c.val == val && c.reason == reason {
			t.Fatalf("want EvictionCallback not called with key=%s, val=%v, reason=%s", key, val, reason)
		}
	}
}

func (m *evictionCallbackMock) HasNotBeenCalled(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) != 0 {
		t.Fatalf("want EvictionCallback not called, got %d calls", len(m.calls))
	}
}

func (m *evictionCallbackMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

func TestImcache_Cleaner(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	caches := []struct {
		name   string
		create func() imcache[string, interface{}]
	}{
		{
			name: "Cache",
			create: func() imcache[string, interface{}] {
				return New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](20*time.Millisecond))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, interface{}] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](20*time.Millisecond))
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
			<-time.After(30 * time.Millisecond)
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonExpired)
			evictioncMock.HasNotBeenCalledWith(t, "foobar", "foobar", EvictionReasonExpired)
			<-time.After(200 * time.Millisecond)
			evictioncMock.HasEventuallyBeenCalledWith(t, "foobar", "foobar", EvictionReasonExpired)
		})
	}
}

func TestImcache_Cleaner_IntervalLessOrEqual0(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	caches := []struct {
		name   string
		create func() imcache[string, interface{}]
	}{
		{
			name: "Cache",
			create: func() imcache[string, interface{}] {
				return New(WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](0))
			},
		},
		{
			name: "Sharded",
			create: func() imcache[string, interface{}] {
				return NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback), WithCleanerOption[string, interface{}](-1))
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
			<-time.After(200 * time.Millisecond)
			evictioncMock.HasNotBeenCalled(t)
		})
	}
}

func TestImcache_Close(t *testing.T) {
	caches := []struct {
		name   string
		create func() imcache[string, string]
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

// cachesWithEvictionCallback is a list of string-interface{}
// caches with eviction callback to test.
// If a test needs different type of cache or one with more sophisticated
// configuration, it should be created within the test.
var cachesWithEvictionCallback = []struct {
	name   string
	create func(EvictionCallback[string, interface{}]) imcache[string, interface{}]
}{
	{
		name: "Cache",
		create: func(f EvictionCallback[string, interface{}]) imcache[string, interface{}] {
			return New[string, interface{}](WithEvictionCallbackOption(f))
		},
	},
	{
		name: "Sharded",
		create: func(f EvictionCallback[string, interface{}]) imcache[string, interface{}] {
			return NewSharded[string, interface{}](8, DefaultStringHasher64{}, WithEvictionCallbackOption(f))
		},
	},
}

func TestImcache_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("imcache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.Set("foo", "bar", WithNoExpiration())
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			c.Set("bar", "foo", WithNoExpiration())
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonReplaced)
		})
	}
}

func TestImcache_GetOrSet_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			<-time.After(time.Nanosecond)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("imcache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
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
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", 1, WithExpiration(time.Nanosecond))
			c.Set("bar", 2, WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.ReplaceWithFunc("foo", func(interface{}) interface{} { return 997 }, WithNoExpiration()); ok {
				t.Errorf("imcache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", 1, EvictionReasonExpired)
			if ok := c.ReplaceWithFunc("bar", func(interface{}) interface{} { return 997 }, WithNoExpiration()); !ok {
				t.Errorf("imcache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", 2, EvictionReasonReplaced)
		})
	}
}

func TestImcache_ReplaceKey_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
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
			<-time.After(time.Nanosecond)
			if ok := c.ReplaceKey("foobar", "barbar", WithNoExpiration()); !ok {
				t.Errorf("imcache.ReplaceKey(%s, _, _) = %t, want true", "foobar", ok)
			}
			evictioncMock.HasEventuallyBeenCalledWith(t, "barbar", "barbar", EvictionReasonExpired)
		})
	}
}

func TestImcache_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
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
	evictioncMock := &evictionCallbackMock{}
	for _, tt := range cachesWithEvictionCallback {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
			evictioncMock.HasEventuallyBeenCalledWith(t, "bar", "bar", EvictionReasonRemoved)
		})
	}
}

func TestImcache_RemoveExpired_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveExpired()
			evictioncMock.HasEventuallyBeenCalledWith(t, "foo", "foo", EvictionReasonExpired)
		})
	}
}

func TestImcache_GetAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	for _, cache := range cachesWithEvictionCallback {
		t.Run(cache.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := cache.create(evictioncMock.Callback)
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

func TestCache_MaxEntriesLimit(t *testing.T) {
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
	evictioncMock.HasEventuallyBeenCalledWith(t, "one", 1, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "three", 3, EvictionReasonMaxEntriesExceeded)

	// Set should evict the last entry from the queue if the size is exceeded
	// and if the entry is expired the eviction reason should be EvictionReasonExpired.
	c.Set("eight", 8, WithNoExpiration())
	// LRU queue: eight -> seven -> two -> six -> five.
	if _, ok := c.Get("four"); ok {
		t.Error("want Cache.Get(_) = _, false, got _, true")
	}
	evictioncMock.HasEventuallyBeenCalledWith(t, "four", 4, EvictionReasonExpired)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "six", 6, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "seven", 7, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "five", 5, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "ten", 10, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "fifteen", 15, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "fourteen", 14, EvictionReasonMaxEntriesExceeded)

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
	evictioncMock.HasEventuallyBeenCalledWith(t, "seventeen", 17, EvictionReasonExpired)
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
	evictioncMock.HasEventuallyBeenCalledWith(t, "twentyone", 21, EvictionReasonExpired)
}

func TestSharded_ReplaceKey_MaxEntriesLimit(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := NewSharded[string, interface{}](2, DefaultStringHasher64{},
		WithMaxEntriesOption[string, interface{}](1),
		WithEvictionCallbackOption(evictioncMock.Callback),
	)
	s.Set("key-1", 1, WithNoExpiration())
	s.Set("key-2", 2, WithNoExpiration())
	if ok := s.ReplaceKey("key-2", "key-3", WithExpiration(time.Nanosecond)); !ok {
		t.Error("Sharded.ReplaceKey(_, _, _) = false, want true")
	}
	// Entry with key-2 should be evicted with a key replaced reason.
	if _, ok := s.Get("key-2"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
	evictioncMock.HasEventuallyBeenCalledWith(t, "key-2", 2, EvictionReasonKeyReplaced)
	// Entry with key-1 should be evicted with a max entries exceeded reason.
	if _, ok := s.Get("key-1"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
	evictioncMock.HasEventuallyBeenCalledWith(t, "key-1", 1, EvictionReasonMaxEntriesExceeded)
	s.Set("key-4", 4, WithNoExpiration())
	// Wait until key-3 is expired.
	<-time.After(time.Nanosecond)
	if ok := s.ReplaceKey("key-4", "key-5", WithNoExpiration()); !ok {
		t.Error("Sharded.ReplaceKey(_, _, _) = false, want true")
	}
	// Entry with key-4 should be evicted with a key replaced reason.
	if _, ok := s.Get("key-4"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
	evictioncMock.HasEventuallyBeenCalledWith(t, "key-4", 4, EvictionReasonKeyReplaced)
	// Entry with key-3 should be evicted with an expired reason.
	if _, ok := s.Get("key-3"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
	evictioncMock.HasEventuallyBeenCalledWith(t, "key-3", 2, EvictionReasonExpired)
}

func TestSharded_ReplaceKey_MaxEntriesLimit_NoEvictionCallback(t *testing.T) {
	s := NewSharded[string, interface{}](2, DefaultStringHasher64{}, WithMaxEntriesOption[string, interface{}](1))
	s.Set("key-1", 1, WithNoExpiration())
	s.Set("key-2", 2, WithNoExpiration())
	if ok := s.ReplaceKey("key-2", "key-3", WithExpiration(time.Nanosecond)); !ok {
		t.Error("Sharded.ReplaceKey(_, _, _) = false, want true")
	}
	// Entry with key-2 should be evicted with a key replaced reason.
	if _, ok := s.Get("key-2"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
	// Entry with key-1 should be evicted with a max entries exceeded reason.
	if _, ok := s.Get("key-1"); ok {
		t.Error("Sharded.Get(_) = _, true, want _, false")
	}
}

func TestCache_MaxEntriesLimit_NoEvictionCallback(t *testing.T) {
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

func TestCache_MaxEntriesLimit_LessOrEqual0(t *testing.T) {
	c := New(WithMaxEntriesOption[string, string](0))
	if _, ok := c.queue.(*nopq[string]); !ok {
		t.Error("Cache.queue = _, want *nopq")
	}
}

type longRunningEvictionCallback struct {
	ctx context.Context
}

func (c *longRunningEvictionCallback) Callback(key, value string, reason EvictionReason) {
	<-c.ctx.Done()
}

func TestImcache_LongRunning_EvictionCallback(t *testing.T) {
	tests := []struct {
		name    string
		execute func(imcache[string, string])
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
		name   string
		create func(EvictionCallback[string, string]) imcache[string, string]
	}{
		{
			name: "Cache",
			create: func(ec EvictionCallback[string, string]) imcache[string, string] {
				return New[string, string](WithEvictionCallbackOption[string, string](ec), WithMaxEntriesOption[string, string](2))
			},
		},
		{
			name: "Sharded",
			create: func(ec EvictionCallback[string, string]) imcache[string, string] {
				return NewSharded[string, string](2, DefaultStringHasher64{}, WithEvictionCallbackOption[string, string](ec), WithMaxEntriesOption[string, string](2))
			},
		},
	}
	for _, cache := range caches {
		for _, tt := range tests {
			t.Run(cache.name+" "+tt.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				ec := &longRunningEvictionCallback{ctx: ctx}
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
	ec := &longRunningEvictionCallback{ctx: ctx}
	c := NewSharded[string, string](2, DefaultStringHasher64{}, WithEvictionCallbackOption[string, string](ec.Callback), WithMaxEntriesOption[string, string](1))
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
