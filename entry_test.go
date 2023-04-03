package imcache

import (
	"testing"
	"time"
)

func TestEntry_HasExpired(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
		now   time.Time
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry{val: "foo", expiration: noExp},
			now:   time.Now(),
		},
		{
			name:  "expired",
			entry: entry{val: "foo", expiration: time.Now().Add(-time.Second).UnixNano()},
			now:   time.Now(),
			want:  true,
		},
		{
			name:  "not expired",
			entry: entry{val: "foo", expiration: time.Now().Add(time.Second).UnixNano()},
			now:   time.Now(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasExpired(tt.now); got != tt.want {
				t.Errorf("entry.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_HasNoExpiration(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry{val: "foo", expiration: noExp},
			want:  true,
		},
		{
			name:  "expired",
			entry: entry{val: "foo", expiration: time.Now().Add(-time.Second).UnixNano()},
		},
		{
			name:  "not expired",
			entry: entry{val: "foo", expiration: time.Now().Add(time.Second).UnixNano()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasNoExpiration(); got != tt.want {
				t.Errorf("entry.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_HasDefaultExpiration(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry{val: "foo", expiration: noExp},
		},
		{
			name:  "expired",
			entry: entry{val: "foo", expiration: time.Now().Add(-time.Second).UnixNano()},
		},
		{
			name:  "not expired",
			entry: entry{val: "foo", expiration: time.Now().Add(time.Second).UnixNano()},
		},
		{
			name:  "default expiration",
			entry: entry{val: "foo", expiration: defaultExp},
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasDefaultExpiration(); got != tt.want {
				t.Errorf("entry.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_HasSlidingExpiration(t *testing.T) {
	tests := []struct {
		name  string
		entry entry
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry{val: "foo", expiration: noExp},
		},
		{
			name:  "expiration",
			entry: entry{val: "foo", expiration: time.Now().Add(time.Second).UnixNano()},
		},
		{
			name:  "sliding expiration",
			entry: entry{val: "foo", expiration: time.Now().Add(time.Second).UnixNano(), sliding: time.Second},
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.HasSlidingExpiration(); got != tt.want {
				t.Errorf("entry.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_SetDefault(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		entry   entry
		now     time.Time
		d       time.Duration
		sliding bool
		want    entry
	}{
		{
			name:  "no expiration",
			entry: entry{val: "foo", expiration: noExp},
			now:   now,
			d:     time.Second,
			want:  entry{val: "foo", expiration: noExp},
		},
		{
			name:  "expiration",
			entry: entry{val: "foo", expiration: now.UnixNano()},
			now:   now,
			d:     time.Second,
			want:  entry{val: "foo", expiration: now.UnixNano()},
		},
		{
			name:  "default expiration",
			entry: entry{val: "foo", expiration: defaultExp},
			now:   now,
			d:     time.Second,
			want:  entry{val: "foo", expiration: now.Add(time.Second).UnixNano()},
		},
		{
			name:    "default sliding expiration",
			entry:   entry{val: "foo", expiration: defaultExp},
			now:     now,
			d:       2 * time.Second,
			sliding: true,
			want:    entry{val: "foo", expiration: now.Add(2 * time.Second).UnixNano(), sliding: 2 * time.Second},
		},
		{
			name:  "default no expiration",
			entry: entry{val: "foo", expiration: defaultExp},
			now:   now,
			d:     noExp,
			want:  entry{val: "foo", expiration: noExp},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.entry.SetDefault(tt.now, tt.d, tt.sliding)
			if tt.entry.expiration != tt.want.expiration || tt.entry.sliding != tt.want.sliding || tt.entry.val != tt.want.val {
				t.Errorf("entry.SetDefault() result %v, want %v", tt.entry, tt.want)
			}
		})
	}
}

func TestEntry_SlideExpiration(t *testing.T) {
	entry := entry{val: "foo", expiration: time.Now().Add(5 * time.Second).UnixNano(), sliding: 5 * time.Second}
	<-time.After(2 * time.Second)
	now := time.Now()
	entry.SlideExpiration(now)
	if want := now.Add(5 * time.Second).UnixNano(); entry.expiration != want {
		t.Errorf("entry.SlideExpiration() results in expiration %v, want %v", entry.expiration, want)
	}
}
