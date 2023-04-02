package imcache

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestShard_Get(t *testing.T) {
	tests := []struct {
		name    string
		s       func() *shard
		key     string
		want    interface{}
		wantErr error
	}{
		{
			name: "success",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
		},
		{
			name: "not found",
			s: func() *shard {
				c := newShard()
				return c
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired",
			s: func() *shard {
				c := newShard()
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
			s := tt.s()
			got, err := s.Get(tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, %v", tt.key, err, tt.wantErr)
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("Cache.Get(%s) = %v, _ want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestShard_Set(t *testing.T) {
	tests := []struct {
		name string
		s    func() *shard
		key  string
		val  interface{}
	}{
		{
			name: "add new entry",
			s: func() *shard {
				c := newShard()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			s: func() *shard {
				c := newShard()
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
			s := tt.s()
			s.Set(tt.key, tt.val, WithNoExpiration())
			got, err := s.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestShard_Add(t *testing.T) {
	tests := []struct {
		name    string
		s       func() *shard
		key     string
		val     interface{}
		wantErr error
	}{
		{
			name: "add new entry",
			s: func() *shard {
				c := newShard()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			s: func() *shard {
				c := newShard()
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
			s := tt.s()
			if err := s.Add(tt.key, tt.val, WithDefaultExpiration()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Add(%s, _, _) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			got, err := s.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestShard_Replace(t *testing.T) {
	tests := []struct {
		name    string
		s       func() *shard
		key     string
		val     interface{}
		wantErr error
	}{
		{
			name: "success",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry expired",
			s: func() *shard {
				c := newShard()
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
			s: func() *shard {
				c := newShard()
				return c
			},
			key:     "foo",
			val:     "bar",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.s()
			if err := s.Replace(tt.key, tt.val, WithDefaultExpiration()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Replace(%s, _, _) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			got, err := s.Get(tt.key)
			if err != nil {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, nil", tt.key, err)
			}
			if !cmp.Equal(got, tt.val) {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestShard_Remove(t *testing.T) {
	tests := []struct {
		name    string
		s       func() *shard
		key     string
		wantErr error
	}{
		{
			name: "success",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
		},
		{
			name: "entry doesn't exist",
			s: func() *shard {
				c := newShard()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "bar",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired",
			s: func() *shard {
				c := newShard()
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
			s := tt.s()
			if err := s.Remove(tt.key); !errors.Is(err, tt.wantErr) {
				t.Fatalf("Cache.Remove(%s) = %v, want %v", tt.key, err, tt.wantErr)
			}
			if _, err := s.Get(tt.key); !errors.Is(err, ErrNotFound) {
				t.Fatalf("Cache.Get(%s) = _, %v, want _, %v", tt.key, err, ErrNotFound)
			}
		})
	}
}

func TestShard_RemoveAll(t *testing.T) {
	s := newShard()
	if err := s.Add("foo", "foo", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	s.RemoveAll()
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
	}
	if _, err := s.Get("bar"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "bar", err, ErrNotFound)
	}
}

func TestShard_RemoveStale(t *testing.T) {
	s := newShard()
	if err := s.Add("foo", "foo", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	s.RemoveStale()
	if _, err := s.Get("foo"); err != nil {
		t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
	}
	if _, err := s.Get("bar"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "bar", err, ErrNotFound)
	}
}

func TestShard_DefaultExpiration(t *testing.T) {
	s := newShard(WithDefaultExpirationOption(500 * time.Millisecond))
	if err := s.Add("foo", "foo", WithDefaultExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	<-time.After(500 * time.Millisecond)
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
	}
}

func TestShard_DefaultSlidingExpiration(t *testing.T) {
	s := newShard(WithDefaultSlidingExpirationOption(500 * time.Millisecond))
	if err := s.Add("foo", "foo", WithDefaultExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	<-time.After(300 * time.Millisecond)
	if _, err := s.Get("foo"); err != nil {
		t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
	}
	<-time.After(300 * time.Millisecond)
	if _, err := s.Get("foo"); err != nil {
		t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
	}
	<-time.After(500 * time.Millisecond)
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
	}
}

func TestShard_Get_SlidingExpiration(t *testing.T) {
	s := newShard()
	if err := s.Add("foo", "foo", WithSlidingExpiration(500*time.Millisecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	<-time.After(300 * time.Millisecond)
	if _, err := s.Get("foo"); err != nil {
		t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
	}
	<-time.After(300 * time.Millisecond)
	if _, err := s.Get("foo"); err != nil {
		t.Errorf("Cache.Get(%s) = _, %v, want _, nil", "foo", err)
	}
	<-time.After(500 * time.Millisecond)
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
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

func TestShard_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	<-time.After(time.Nanosecond)
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
	}
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
}

func TestShard_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	s.Set("foo", "bar", WithNoExpiration())
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
	s.Set("bar", "foo", WithNoExpiration())
	if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
	}
}

func TestShard_Add_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	<-time.After(time.Nanosecond)
	if err := s.Add("foo", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
}

func TestShard_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	if err := s.Replace("foo", "bar", WithNoExpiration()); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Replace(%s, _, _) = %v, want %v", "foo", err, ErrNotFound)
	}
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
	if err := s.Replace("bar", "foo", WithNoExpiration()); err != nil {
		t.Errorf("Cache.Replace(%s, _, _) = %v, want nil", "bar", err)
	}
	if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
	}
}

func TestShard_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	if err := s.Remove("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Remove(%s) = %v, want %v", "foo", err, ErrNotFound)
	}
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
	if err := s.Remove("bar"); err != nil {
		t.Errorf("Cache.Remove(%s) = %v, want nil", "bar", err)
	}
	if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
	}
}

func TestShard_RemoveAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	s.RemoveAll()
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
	if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonRemoved)
	}
}

func TestShard_RemoveStale_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}
	s := newShard(WithEvictionCallbackOption(evictioncMock.Callback))
	if err := s.Add("foo", "foo", WithExpiration(time.Nanosecond)); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "foo", err)
	}
	if err := s.Add("bar", "bar", WithNoExpiration()); err != nil {
		t.Fatalf("Cache.Add(%s, _, _) = %v, want nil", "bar", err)
	}
	<-time.After(time.Nanosecond)
	s.RemoveStale()
	if _, err := s.Get("foo"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Cache.Get(%s) = _, %v, want _, %v", "foo", err, ErrNotFound)
	}
	if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
		t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
	}
}
