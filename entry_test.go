package imcache

import (
	"testing"
	"time"
)

func TestEntry_HasExpired(t *testing.T) {
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
			if got := tt.entry.HasExpired(tt.now); got != tt.want {
				t.Errorf("entry.HasExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntry_HasNoExpiration(t *testing.T) {
	tests := []struct {
		name  string
		entry entry[string, string]
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: noExp}},
			want:  true,
		},
		{
			name:  "expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(-time.Second).UnixNano()}},
		},
		{
			name:  "not expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(time.Second).UnixNano()}},
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
		entry entry[string, string]
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: noExp}},
		},
		{
			name:  "expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(-time.Second).UnixNano()}},
		},
		{
			name:  "not expired",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(time.Second).UnixNano()}},
		},
		{
			name:  "default expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: defaultExp}},
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
		entry entry[string, string]
		want  bool
	}{
		{
			name:  "no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: noExp}},
		},
		{
			name:  "expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(time.Second).UnixNano()}},
		},
		{
			name:  "sliding expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(time.Second).UnixNano(), sliding: time.Second}},
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
		now     time.Time
		name    string
		entry   entry[string, string]
		want    entry[string, string]
		d       time.Duration
		sliding bool
	}{
		{
			name:  "no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: noExp}},
			now:   now,
			d:     time.Second,
			want:  entry[string, string]{val: "foo", exp: expiration{date: noExp}},
		},
		{
			name:  "expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: now.UnixNano()}},
			now:   now,
			d:     time.Second,
			want:  entry[string, string]{val: "foo", exp: expiration{date: now.UnixNano()}},
		},
		{
			name:  "default expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: defaultExp}},
			now:   now,
			d:     time.Second,
			want:  entry[string, string]{val: "foo", exp: expiration{date: now.Add(time.Second).UnixNano()}},
		},
		{
			name:    "default sliding expiration",
			entry:   entry[string, string]{val: "foo", exp: expiration{date: defaultExp}},
			now:     now,
			d:       2 * time.Second,
			sliding: true,
			want:    entry[string, string]{val: "foo", exp: expiration{date: now.Add(2 * time.Second).UnixNano(), sliding: 2 * time.Second}},
		},
		{
			name:  "default no expiration",
			entry: entry[string, string]{val: "foo", exp: expiration{date: defaultExp}},
			now:   now,
			d:     noExp,
			want:  entry[string, string]{val: "foo", exp: expiration{date: noExp}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.entry.SetDefaultOrNothing(tt.now, tt.d, tt.sliding)
			if tt.entry.exp.date != tt.want.exp.date || tt.entry.exp.sliding != tt.want.exp.sliding || tt.entry.val != tt.want.val {
				t.Errorf("entry.SetDefault() result %v, want %v", tt.entry, tt.want)
			}
		})
	}
}

func TestEntry_SlideExpiration(t *testing.T) {
	entry := entry[string, string]{val: "foo", exp: expiration{date: time.Now().Add(5 * time.Second).UnixNano(), sliding: 5 * time.Second}}
	<-time.After(2 * time.Second)
	now := time.Now()
	entry.SlideExpiration(now)
	if want := now.Add(5 * time.Second).UnixNano(); entry.exp.date != want {
		t.Errorf("entry.SlideExpiration() results in expiration %v, want %v", entry.exp.date, want)
	}
}
