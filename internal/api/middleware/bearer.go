package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/marc-schuetze/shitmail/internal/storage"
)

// Bearer accepts only user tokens. Proxy headers cannot supply or elevate this identity.
func Bearer(tokens *storage.TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) != 67 || !strings.HasPrefix(parts[1], "sm_") {
				bearerUnauthorized(w)
				return
			}
			owner, err := tokens.Owner(r.Context(), parts[1])
			if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			if owner == "" {
				bearerUnauthorized(w)
				return
			}
			user := User{UID: owner, Tag: Tag(owner)}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, user)))
		})
	}
}

func bearerUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="shitmail agent API"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"missing, expired or revoked bearer token"}`))
}
