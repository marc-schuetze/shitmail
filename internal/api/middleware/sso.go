package middleware

import (
	"context"
	"crypto/sha256"
	"math/big"
	"net/http"
	"strings"
)

// User is the identity the reverse proxy (Authentik forward-auth) attached to
// the request. shitmail never authenticates itself; it trusts these headers and
// must only be reachable through the proxy.
type User struct {
	UID      string
	Username string
	Admin    bool
	Tag      string // stable per-user suffix used in mailbox addresses
}

type ctxKey struct{}

// SSO rejects requests without an X-Authentik-Uid header and stores the
// resolved User in the request context. adminGroup names the
// X-Authentik-Groups entry that grants admin rights.
func SSO(adminGroup string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid := strings.TrimSpace(r.Header.Get("X-Authentik-Uid"))
			if uid == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"no SSO identity on request"}`))
				return
			}
			u := User{
				UID:      uid,
				Username: r.Header.Get("X-Authentik-Username"),
				Tag:      Tag(uid),
			}
			for _, g := range strings.Split(r.Header.Get("X-Authentik-Groups"), "|") {
				if adminGroup != "" && strings.TrimSpace(g) == adminGroup {
					u.Admin = true
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
		})
	}
}

// UserFrom returns the SSO user stored by SSO, if any.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Tag derives the 5-char base36 address suffix for a uid: base36(sha256(uid))[:5].
// Stateless, so it survives a database reset.
func Tag(uid string) string {
	sum := sha256.Sum256([]byte(uid))
	return new(big.Int).SetBytes(sum[:]).Text(36)[:5]
}
