package handler

import (
	"testing"
	"time"
)

func TestResolveTTL(t *testing.T) {
	def := 24 * time.Hour
	if d, err := resolveTTL(0, def); err != nil || d != def {
		t.Fatalf("default: %v %v", d, err)
	}
	if d, err := resolveTTL(168, def); err != nil || d != 168*time.Hour {
		t.Fatalf("168h: %v %v", d, err)
	}
	if d, err := resolveTTL(TTLForever, def); err != nil || d < 50*365*24*time.Hour {
		t.Fatalf("forever: %v %v", d, err)
	}
	if _, err := resolveTTL(3, def); err == nil {
		t.Fatal("3h must be rejected")
	}
	// A forever expiry must still sort after any normal expiry as RFC3339 text.
	a := time.Now().Add(168 * time.Hour).UTC().Format(time.RFC3339Nano)
	b := time.Now().Add(foreverDuration).UTC().Format(time.RFC3339Nano)
	if !(a < b) {
		t.Fatalf("ordering broken: %s !< %s", a, b)
	}
}
