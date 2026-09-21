package middleware

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestTag(t *testing.T) {
	a, b := Tag("42"), Tag("43")
	if a == b || !regexp.MustCompile(`^[0-9a-z]{5}$`).MatchString(a) || a != Tag("42") {
		t.Fatalf("tag: %q %q", a, b)
	}
}

func TestSSO(t *testing.T) {
	var got User
	h := SSO("authentik Admins")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = UserFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 {
		t.Fatalf("no header: %d", rec.Code)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Authentik-Uid", "7")
	req.Header.Set("X-Authentik-Groups", "users|authentik Admins")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.UID != "7" || !got.Admin || got.Tag != Tag("7") {
		t.Fatalf("user: %+v", got)
	}
}
