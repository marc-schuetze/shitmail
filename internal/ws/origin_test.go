package ws

import (
	"net/http/httptest"
	"testing"
)

func TestCheckOrigin(t *testing.T) {
	PublicHost = "shit.example"
	mk := func(origin, host, fwd string) bool {
		r := httptest.NewRequest("GET", "/ws", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if fwd != "" {
			r.Header.Set("X-Forwarded-Host", fwd)
		}
		return upgrader.CheckOrigin(r)
	}
	for i, c := range []struct {
		origin, host, fwd string
		want              bool
	}{
		{"https://shit.example", "shit.example", "", true},
		{"https://shit.example", "shit.example:443", "", true},
		{"https://shit.example", "10.0.0.9", "", true},                   // public host wins
		{"https://shit.example", "10.0.0.1:8080", "shit.example", true},  // forwarded host
		{"https://evil.example", "shit.example", "", false},
		{"", "anything", "", true},
	} {
		if got := mk(c.origin, c.host, c.fwd); got != c.want {
			t.Errorf("case %d %+v: got %v", i, c, got)
		}
	}
}
