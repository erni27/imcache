package imcache

import (
	"errors"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestCache_Get(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Cache[string, string]
		key     string
		want    string
		wantErr error
	}{
		{
			name: "success",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
		},
		{
			name: "not found",
			c: func() Cache[string, string] {
				c := New[string, string]()
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "success - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
		},
		{
			name: "not found - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			got, err := c.Get(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, %v", tt.key, err, tt.wantErr)
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("Cache.Get(%s) = %v, _ want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestCache_Get_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, string]
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
			if err := c.Add("foo", "foo", WithSlidingExpiration(500*time.Millisecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			<-time.After(300 * time.Millisecond)
			if _, err := c.Get("foo"); err != nil {
				t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
			}
			<-time.After(300 * time.Millisecond)
			if _, err := c.Get("foo"); err != nil {
				t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
			}
			<-time.After(500 * time.Millisecond)
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
		})
	}
}

func TestCache_Set(t *testing.T) {
	tests := []struct {
		name string
		c    func() Cache[string, string]
		key  string
		val  string
	}{
		{
			name: "add new entry",
			c: func() Cache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() Cache[string, string] {
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
			c: func() Cache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() Cache[string, string] {
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
			got, err := c.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestCache_Add(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Cache[string, string]
		key     string
		val     string
		wantErr error
	}{
		{
			name: "add new entry",
			c: func() Cache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrAlreadyExists,
		},
		{
			name: "add new entry - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if err := c.Add(tt.key, tt.val, WithDefaultExpiration()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			got, err := c.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestCache_Replace(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Cache[string, string]
		key     string
		val     string
		wantErr error
	}{
		{
			name: "success",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry expired",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "entry doesn't exist",
			c: func() Cache[string, string] {
				c := New[string, string]()
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "success - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry expired - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if err := c.Replace(tt.key, tt.val, WithDefaultExpiration()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Replace(%s, _, _) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			got, err := c.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestCache_Remove(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Cache[string, string]
		key     string
		wantErr error
	}{
		{
			name: "success",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
		},
		{
			name: "entry doesn't exist",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired",
			c: func() Cache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "success - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired - sharded",
			c: func() Cache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if err := c.Remove(tt.key); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Remove(%s) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if _, err := c.Get(tt.key); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, %v", tt.key, err, ErrNotFound)
			}
		})
	}
}

func TestCache_RemoveAll(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, string]
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
			if err := c.Add("foo", "foo", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
			if _, err := c.Get("bar"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "bar", err, ErrNotFound)
			}
		})
	}
}

func TestCache_RemoveStale(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, string]
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
			if err := c.Add("foo", "foo", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			c.RemoveStale()
			if _, err := c.Get("foo"); err != nil {
				t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
			}
			if _, err := c.Get("bar"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "bar", err, ErrNotFound)
			}
		})
	}
}

func TestCache_Len(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, int]
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
				if err := c.Add(strconv.Itoa(i), i, WithNoExpiration()); err != nil {
					t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", strconv.Itoa(i), err)
				}
			}
			if got := c.Len(); got != n {
				t.Errorf("Cache.Len() = %d, want %d", got, n)
			}
		})
	}
}

func TestCache_DefaultExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, string]
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
			if err := c.Add("foo", "foo", WithDefaultExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			<-time.After(500 * time.Millisecond)
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
		})
	}
}

func TestCache_DefaultSlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Cache[string, string]
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
			if err := c.Add("foo", "foo", WithDefaultExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			<-time.After(300 * time.Millisecond)
			if _, err := c.Get("foo"); err != nil {
				t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
			}
			<-time.After(300 * time.Millisecond)
			if _, err := c.Get("foo"); err != nil {
				t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
			}
			<-time.After(500 * time.Millisecond)
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
		})
	}
}

type evictionCallbackCall struct {
	key    string
	val    interface{}
	reason EvictionReason
}

type evictionCallbackMock struct {
	calls []evictionCallbackCall
}

func (m *evictionCallbackMock) Callback(key string, val interface{}, reason EvictionReason) {
	m.calls = append(m.calls, evictionCallbackCall{key, val, reason})
}

type anyValue struct{}

func (m *evictionCallbackMock) HasBeenCalledWith(key string, val interface{}, reason EvictionReason) bool {
	var call evictionCallbackCall
	for _, c := range m.calls {
		if c.key == key {
			call = c
			break
		}
	}
	if _, ok := val.(anyValue); ok {
		return call.key == key && call.reason == reason
	}
	return call.key == key && call.val == val && call.reason == reason
}

func (m *evictionCallbackMock) HasNotBeenCalled() bool {
	return len(m.calls) == 0
}

func (m *evictionCallbackMock) Reset() {
	m.calls = nil
}

func TestCache_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			<-time.After(time.Nanosecond)
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestCache_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
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

func TestCache_Add_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			<-time.After(time.Nanosecond)
			if err := c.Add("foo", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestCache_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			if err := c.Replace("foo", "bar", WithNoExpiration()); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Replace(%s, _, _) = %v, want %v", "foo", err, ErrNotFound)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if err := c.Replace("bar", "foo", WithNoExpiration()); err != nil {
				t.Errorf("Cache.Replace(%s, _, _) = %v, want nil", "bar", err)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestCache_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			if err := c.Remove("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Remove(%s) = %v, want %v", "foo", err, ErrNotFound)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if err := c.Remove("bar"); err != nil {
				t.Errorf("Cache.Remove(%s) = %v, want nil", "bar", err)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestCache_RemoveAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
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

func TestCache_RemoveStale_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			c.RemoveStale()
			if _, err := c.Get("foo"); !errors.Is(err, ErrNotFound) {
				t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestCache_GetAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Cache[string, interface{}]
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
			if err := c.Add("foo", "foo", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("foobar", "foobar", WithNoExpiration()); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
			}
			if err := c.Add("bar", "bar", WithExpiration(time.Nanosecond)); err != nil {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
			}
			<-time.After(time.Nanosecond)
			got := c.GetAll()
			want := map[string]interface{}{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !cmp.Equal(got, want) {
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
