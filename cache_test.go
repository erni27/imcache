package imcache

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewSharded_NSmallerThan0(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(-1, _) did not panic")
		}
	}()
	NewSharded(-1, NewDefaultHasher32())
}

func TestNewSharded_NilHasher(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(_, nil) did not panic")
		}
	}()
	NewSharded(2, nil)
}

func TestGetConcrete_Sharded(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		key     string
		want    string
		wantErr error
	}{
		{
			name: "success",
			setup: func() {
				c := NewSharded(2, NewDefaultHasher32())
				c.Set("foo", "bar", WithNoExpiration())
				SetInstance(c)
			},
			key:  "foo",
			want: "bar",
		},
		{
			name: "not found",
			setup: func() {
				c := NewSharded(2, NewDefaultHasher32())
				SetInstance(c)
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "entry expired",
			setup: func() {
				c := NewSharded(2, NewDefaultHasher32())
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				SetInstance(c)
			},
			key:     "foo",
			wantErr: ErrNotFound,
		},
		{
			name: "invalid type",
			setup: func() {
				c := NewSharded(2, NewDefaultHasher32())
				c.Set("foo", 997, WithNoExpiration())
				SetInstance(c)
			},
			key:     "foo",
			wantErr: ErrTypeAssertion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := GetConcrete[string](tt.key)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Get(%s) = _, %v, want _, %v", tt.key, err, tt.wantErr)
			}
			if !cmp.Equal(got, tt.want) {
				t.Errorf("Get(%s) = %v, _ want %v, _", tt.key, got, tt.want)
			}
		})
	}
}
