package imcache

import (
	"testing"
)

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
